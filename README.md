# eclipse-hdr

Takes a pile of DNG photos of totality during a solar eclipse, and
exposure stack them to get a pretty picture of the corona.

![fattal02](https://github.com/abworrall/eclipse-hdr/blob/master/samples/thumb/tmo-fattal02.png)

It works like this ...

## 1. Image alignment

If you stuck your camera on a tripod and took a bunch of
exposure-bracketed photos during totality, then the sun & moon will
have moved a little between each frame.

The alignment stage compensates for this, trying various translations
and rotations to find where the images agree the most. It performs
sub-pixel alignment, using Catmull Rom interpolation as needed.

## 2. Image fusion

Once the input images have been aligned, we fuse them into a single
HDR (high dynamic range) image. This normalizes across the differing
exposure values used for the images.

It generates a `.hdr` image file, which can be used with other HDR
software such as Adobe PhotoShop, or the command line suite `pfstmo`:

    pfsin fused.hdr | pfstmo_fattal02 --white-point 0.00001  | pfsout fattal.png

## 3. Tone mapping

HDR files can't really be viewed directly - they need to be converted
into LDR (low dynamic range) files. This conversion is called tone
mapping.

We bundle a number of tone-mapping operators: drago03, durand,
fattal02, icam06, reinhard05, and a linear operator. You can see some
output in [samples/](samples/README.md). (Huge thanks to
github.com/mdouchement/hdr for most of these !)

# How to use it

The biggest prep step is building the DNG SDK on your system - Adobe's
library is too hairy to be fully buildable as a Golang package.
Luckily that's only a few commands, details here:
https://github.com/abworrall/go-dng#using-the-sdk-from-go

Prep steps:

    # Install DNG SDK: https://github.com/abworrall/go-dng#using-the-sdk-from-go

    # Now, you can build eclipse-hdr
    sudo apt install build-essential libfftw3-dev
    go install github.com/abworrall/eclipse-hdr/cmd/eclipse-hdr@latest
    ~/go/bin/eclipse-hdr -h

Usage:

    eclipse-hdr images/                   # load everything in the dir
    eclipse-hdr images/1234.DNG ...       # load specific file(s)
    eclipse-hdr -finetunealign images/    # generate fine-tuned alignment (takes ages)
    eclipse-hdr images/ ./conf.yaml       # also load a config file

    eclipse-hdr -developer=layer images/  # see which layers get used
    eclipse-hdr -width=1.2 images/        # generate images not much wider than the sun

## Supported photo files

This tool expects to see DNG files (Adobe Digital Negative). As well
as pixel data, and exposure metadata, DNGs provide matrices for color
correction and white balance. Plus we can access them as simple
linearized sensor data (DNG's "stage 3"), which is what we need for
image fusion.

### Using TIFFs instead

If for some reason you can't use DNGs, then you can make do with TIFF
files and some color configuration. The TIFF files should contain
linear (unadjusted) image data, and the easiest way to get those is
using Adobe's `dng_validate.exe` tool to output stage 3 data.

You then need to use `exiftool.exe` to copy over the EXIF metadata.
For more detail, go [here](pkg/ecolor/README.md).

Take note of your `AsShotNeutral` and `ForwardMatrix` info that gets
printed by `dng_validate.exe -v`; you'll need it for `conf.yaml`.

Note you can build `dng_validate.exe` for linux; details at
https://github.com/abworrall/go-dng#building-the-sdk-on-linux

## Alignment fine-tuning

By default, the alignment is pretty coarse - it just lines up the dark
moon in each photo. The `-alignfinetune` argument does much more work,
trying hundreds of possible alignments and scoring how well the images
agree. This can take many hours, so you only want to do it once, and
do it overnight.

When it finishes, it will print out some configuration. You should
save this for your `conf.yaml` (see below).

If you run in verbose mode (`-v=2`), it will write hundreds of images
to disc, each one a luminance diff of a proposed alignment.

## conf.yaml

Mostly you should put your alignment info in here, as it takes so
long to compute.

If you're using TIFF files, you'll also need your color correction
info - the manual overrides for AsShotNeutral and ForwardMatrix that
you've figured out some other way.

```yaml    
# Reuse expensive-to-compute fine alignments
alignments:
  5671-5667:
    name: 5671-5667
    translatebyx: 11.75
    translatebyy: -16.5
    rotationcenterx: 3269
    rotationcentery: 1344
    rotatebydeg: 3.191891195797325e-16
    errormetric: 21455.073216304932
  5671-5668:
    name: 5671-5668
    translatebyx: 8.5
    translatebyy: -6.25
    rotationcenterx: 3269
    rotationcentery: 1344
    rotatebydeg: 3.191891195797325e-16
    errormetric: 24212.739338470437

# Needed for TIFF files
manualoverrideasshotneutral:
- 0.501
- 1
- 0.7014
manualoverrideforwardmatrix:
- 0.6227
- 0.3389
- 0.0026
- 0.2548
- 0.9378
- -0.1926
- 0.0156
- -0.133
- 0.9425
```

Note the forwardmatrix is a row at a time; the one above corresponds
to this output from `dng_validate.exe`:

    ForwardMatrix2:
                                             0.6227   0.3389   0.0026
                                             0.2548   0.9378  -0.1926
                                             0.0156  -0.1330   0.9425

You only want one config file to be loaded, the last one overwrites.

## Output files

The outputs are all centered on the eclipse itself, are square, and
are sized in terms of lunar diameters via `-width`.

### Fused HDR image, suitable for PhotoShop, PFSTMO, etc

The main output is `fused.hdr`, a high-dynamic range file combining
all the exposures. You can process this further in standard software.

### Tonemapped LDR images

It will also generate a PNG file for each supported tonemapping
operator, e.g. `tmo-fattal02.png`.

- fattal02 seems the most reliable, though it amplifies noise a bit
- icam06 seems to use a different white reference, so is pinkish and warm
- linear always looks dim, that's why we need fancy tonemappers
- reinhard05 looks great with width<=3, but goes wrong when there is too much dark sky
