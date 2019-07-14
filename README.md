# estacker

Take a pile of photos of a full solar eclipse, and exposure stack
them. The goal is to try and get as a pretty a picture of the corona
as we can.

Usage: `estacker -d ~/your_image_dir/`

## Input files

The input should be a set of TIFF files (16 bits per channel), that
have complete EXIF metadata about the exposure. The TIFF files should
contain uncorrected image data, i.e. the pixel values should linearly
map to the luminance recorded by the sensor, with no curves or color
correction.

The simplest way to get such images is using Adobe's `dng_validate`
tool on your DNG files, with the argument `-3` to output 'stage 3'
data - i.e. after bayer processing, but before tone curves are
applied. There is a nice writeup
[here](http://robdose.com.au/extracting-the-raw-image-data-from-a-dng/).
(Simply exporting a TIFF from Lightroom with all the sliders set to
zero is insufficient; whe Lightroom first imports a raw file, it will
apply various color curves and gamma corrections.)

The `dng_validate` step will lose your EXIF metadata, so install `exiftool`,
and use `exiftool -TagsFromFile orig new` to copy it over.

## Config file

Your input TIFF files should be in a directory, along with a config
file called `stack.yaml`.

- gamma: we use a simple gamma to do output tone mapping. Values
  around 0.2-0.5 are good.
- maxcandles: this is the ceiling we aply to absolute luminance
  values. It should typically by 50% of the max candles of your least
  sensitive exposure (i.e. highest EV number).
- combinerstrategy - how to combine pixels from the input files
  - hdr - after removing overflow & underlflow, takes an average from
    all frames, after luminance normalization
  - bestexposed - picks one value from the frame considered to have
    the best exposure (e.g. slowest shutter speed that doesn't
    overflow)
  - quad - each quadrant comes from a different image, so you can see
    how the absolute luminances and alignments compare
  - bullseye - draws a bulleye in the center of the image, with a
    central gap at `solarradiuspixels`. This helps you get your
    boundingbox nice and centered
- radialexponent - this is pretty hacky. As pixels get further from
  the center, apply an increasing exponential function to lighten
  them. Helps bring out some more detail in the outer corona.
- solarradiuspixels - used for the radialexponent. Figure it out using
  `bullseye` above.
- bounds - the bounding box for the output image. Use `bullseye` above
  to figure it out.
- alignment: list every TIFF file here, with the coord translation
  needed to align it. Origin for translation is top-left, i.e. Y axis
  goes downwards. Figuring this out is a royal pain. You can run the
  tool with the `-alignslideshow` argument, which will cause it to
  spit out separate frames for each input image, after applying
  alignment and bounds; you can view these images and flip between
  them to help figure out values. One image should be your base image,
  and thus have an alignment of [0,0].
- exclude: a list of files to exclude (i.e. ignore them entirely).
  When figuring out alignment using the slideshow, it can be handy to
  ignore all but two files.

Example `stack.yaml`:
```
rendering:
  outputfilename: out.png
  gamma: 0.275
  maxcandles: 512
  combinerstrategy: bestexposed
  radialexponent: 1.25
  solarradiuspixels: 315
  bounds:
    min:
      x: 1775
      y: 205
    max:
      x: 4175
      y: 2605

# How to translate each image in the stack. Origin is top-right, so [10,15] means 10 left
# and 15 down.
alignment:
  5662.tif: [  0,  0]  # Explicitly include the reference image, that is left as-is
  5663.tif: [ -5,  0]
  5664.tif: [ -7,  3]
  5665.tif: [-10,  5]
  5666.tif: [-14,  6]

exclude: [
#  5663.tif,
#  5664.tif,
]  

```

## Processing

The program does the following ...

### Alignment

The images are presumed to be a series that was taken in succession, on a
tripod. But the sun is moving, so it will move some number of pixels between
frames.

More subtly, the moon is also moving, and much more quickly; this means the
circular shadow will be moving in relation to the corona. Which means we can't
simply align the images by aligning the circular shadows.

Ideally, there will be a star that's bright enough to be used as an anchor.

For now, I'm punting on alignment, and expecting you to figure it out, and
provide a table of the offsets of each image from the first in the series.

### Normalization

The exposure information in each image is used to derive an [exposure
value](https://en.wikipedia.org/wiki/Exposure_value), that represents the
Luminance (in cd/m^2) needed to fully expose a pixel.

Assuming that no tone curves or corrections have been applied, then a pixel
value of `0xFFFF` should correspond to the physical luminance `MaxLum` that
fully exposes for that EV. The values should be linear, so we can easily map
the pixel values from [0,0xFFFF] to luminances [0,Lum].

Once this is done, pixel values from images with different exposures can be
compared and combined with each other.

Pixels that are very near the maax of `0xFFFF` are discarded, on the
assumption that they are overflowing.

### Merging images

There are a range of algorithms here, as listed above in the config
section. In practice, I just use `bestexposed` all the time.

This should really be done using Laplacian pyramids, difference
images, etc:
- http://www.cs.technion.ac.il/~ronrubin/Projects/fusion/index.html
- http://www.cs.toronto.edu/~jepson/csc320/notes/pyramids.pdf


### Tone mapping

Finally, we need to map the output Luminance values into RGBA colors for the
final image. In regular HDR, this is a very sophisticated operation.

For eclipse shots, we just use a simple encoding gamma correction, to pull as
much detail out of the shadows as possible, so we can emphasize the corona.

We also have a `radial exponent` function, which is a blunt hack that
tries to extract more detail from the outer corona. Given the distance
of the pixel from the center of the sun as a multiple of solar radii,
`R` (e.g. a value of 1.0 means right on the circumference of the sun),
and an exponent of `x`, apply the (hyper?)exponential scaling
function: L' = L^(R^x).

Values of `x` between 1.0 and 1.8 seem to work well.

## Further reading and related links

- https://www.cl.cam.ac.uk/~rkm38/pdfs/tone_mapping.pdf
- https://github.com/mdouchement/hdr
