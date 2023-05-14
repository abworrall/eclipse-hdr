# Camera Native colors

There is a pipeline of things that turn a camera's sensor readings
into an RGB image you're looking at. This is called "developing" a
digital image.

We're going to assume the photos are Adobe DNG files from here. Adobe
provides (the source code for) a tool, `dng_validate.exe`, which lets
you tap the data at intermediate stages of processing:
- Stage 1: the base data, raw counts right off the photosites
- Stage 2: linearized, in a canonical range [0, 0xFFFF]
- Stage 3: demosaiced, e.g. into a 3-channel RGB color

We work on stage 3 data, which is "linear" - hasn't been white
balanced, color corrected, color curved, gamma expanded, etc. It will
look greenish and dim if you view it directly.

This data is in the Camera Native color space.

## Getting the data

First, get hold of the `dng_validate.exe` binary. Adobe don't provide
it directly, if you can't find one somewhere you'll need to download
the source of the DNG SDK from
[here](https://helpx.adobe.com/camera-raw/digital-negative.html), and
maybe a trial version of Microsoft Visual Studio to compile it with
(some notes [here](https://stackoverflow.com/questions/5517809/is-there-any-pre-built-exe-binary-for-adobe-dng-sdk))

You'll also need a copy of `exiftool.exe` to copy over metadata, so
we'll know how each image was exposed. Now you can do this for your
DNG file:

    C:\> dng_validate.exe -3 mynewfile.tif DSC_1234.DNG
    C:\> exiftool -tagsFromFile DSC_1234.DNG mynewfile.tif

You will also want to run `dng_validate.exe` in verbose mode, to get
some important info (rest of output ignored):

    C:\> dng_validate.exe -v DSC_1234.DNG

    
    ColorMatrix1:
                                             0.9033  -0.3555  -0.0287
                                            -0.4881   1.2214   0.3022
                                            -0.0459   0.0926   0.7932
    ColorMatrix2:
                                             0.8598  -0.2848  -0.0857
                                            -0.5618   1.3606   0.2195
                                            -0.1002   0.1773   0.7137
    CameraCalibration1:
                                             1.0000   0.0000   0.0000
                                             0.0000   1.0000   0.0000
                                             0.0000   0.0000   1.0000
    CameraCalibration2:
                                             1.0000   0.0000   0.0000
                                             0.0000   1.0000   0.0000
                                             0.0000   0.0000   1.0000
    AnalogBalance: 1.0000 1.0000 1.0000
    AsShotNeutral: 0.5010 1.0000 0.7014
    CalibrationIlluminant1: Standard light A
    CalibrationIlluminant2: D65
    ForwardMatrix1:
                                             0.6919   0.1772   0.0952
                                             0.2645   0.8172  -0.0817
                                             0.0392  -0.2442   1.0301
    ForwardMatrix2:
                                             0.6227   0.3389   0.0026
                                             0.2548   0.9378  -0.1926
                                             0.0156  -0.1330   0.9425

Depending on your camera, you might get `AsShotXY` instead of
`AsShotNeutral`, which isn't handled by eclipse-hdr.

What you need to see here, for your images to work with eclipse-hdr:
- `AsShotNeutral` - and *not* `AsShotXY`, which isn't supported
- `AnalogBalance` - should be `1.0 1.0 1.0`
- `CameraCalibration` - both should be identity matrices, as above

You need to pick a `CalibrationIlluminant` to know which
`ForwardMatrix` to use - go for D65 (or whatever is closest to it),
because eclipse light is direct sunlight. In the example above that
means `CalibrationIlluminant2`, which in turn means I want
`ForwardMatrix2`.

If you have `AsShotNeutral` and `ForwardMatrix`, and the dng_validate
stage 3 TIFF files, you're good to go :)

Some notes on `dng_validate.exe`: http://robdose.com.au/extracting-the-raw-image-data-from-a-dng/

## How DNG Develops the raw data

The DNG spec has a useful description of how it performs these steps.

We're going to walk through the section "Mapping Camera Color Space to
CIE XYZ Space", pp. 85-88 in v1.6.0.0 of the spec. Useful links:
- the spec: https://helpx.adobe.com/content/dam/help/en/photoshop/pdf/dng_spec_1_6_0_0.pdf
- a matlab implementation: https://sites.google.com/site/crossstereo/raw-converting/dng

Things we can ignore in the spec, given the data from our images:
- CC - `CameraCalibration` Matrices - they are all identity matrices
- AB - `AnalogBalance` - this is also identity

### page 86: Translating White Balance xy Coords to Camera Neutral Coords

The goal of this section: a 3x1 vector, a "camera neutral coordinate",
i.e. a perfect grey located in the camera native RGB coordinate space.

We have `AsShotNeutral`: "specifies the selected white balance at time
of capture, encoded as the coordinates of a perfectly neutral color in
linear reference space values." This _is_ the camera neutral
coordinate (`CameraNeutral` in the spec), so if we're happy to use the
white balance that the camera deduced at time of shooting, we can
skip this section. And we are :)

If we wanted to change the color temperature we'd need to do a whole
bunch of work here. In some sample photos of the same thing taken
seconds apart, you can see saw a range of color temps assigned by the
camera (4800-5000K). (Note that the `dng_validate.exe -v` output omits
the color temp info; but if you run the `tiffinfo` tool against a TIFF
with all the EXIF data, it dumps out some XML that has a tag
<crs:Temperature/> that has the Kelvin color temp the camera assigned
to that image.)

### pages 87-88: Camera to XYZ (D50) Transform

The goal of this section: a 3x3 matrix transform. Its input is an RGB
color in camera native space. The output is a color in the
camera-independent XYZ(D50) space (the XYZ space, anchored to a D50
illuminant - i.e. with a hardwired white balance corresponding to the
"warm daylight" D50 standard illuminant)

It's performing two distinct steps: apply a white balance correction
that will make the RGB value appear neutral at D50; then map the RGB
color into an XYZ color.

We have `ForwardMatrix`: "defines a matrix that maps white balanced
camera colors to XYZ D50 colors.". This makes things really easy.

Quickly boiling down the matrix calcs, given:
    AB = I (the identity matrix)
    CC = I
    CameraNeutral = AsShotNeutral

    ReferenceNeutral = invert(AB * CC) * CameraNeutral
                     = invert(I * I) * AsShotNeutral
                     = I * AsShotNeutral
                     = AsShotNeutral

                   D = invert(diag(ReferenceNeutral)
                     = invert(diag(AsShotNeutral)

     CameraToXYZ_D50 = FM * D * inverse(AB * CC)
                     = FM * D * inverse(I * I)
                     = FM * invert(diag(AsShotNeutral)

The first part of the transform, the white balance correction D, will
scale the RGB values it takes as input, multiplying R by
1/AsShotNeutralR, G by 1/AsShotNeutralG, etc. Random internet comments
suggest doing this exact thing to apply a white balance correction on
camera native RGB, given an RGB that represents neutral/grey.

Then the `ForwardMatrix` will map the "white balanced camera colors"
over to XYZ(D50).

### Conversion back to RGB for TIFF output

We're going to render into the sRGB color space (there are other RGB
spaces), as it is well known, e.g.:
- https://www.sjbrown.co.uk/posts/gamma-correct-rendering/
- https://observablehq.com/@sebastien/srgb-rgb-gamma
- https://www.image-engineering.de/library/technotes/958-how-to-convert-between-srgb-and-ciexyz
- http://www.brucelindbloom.com/index.html?Eqn_RGB_XYZ_Matrix.html

We need to take care about white reference points. sRGB assumes a D65
white reference, but our color correction into XYZ assumed a D50 white
reference. So a chromatic adaptation is needed, or the white balance
will look wrong in our sRGB file. Bruce Lindbloom's site has a
matrix that includes this step, so we use that.

There is also an optional gamma expansion in sRGB, to adjust for human
brightness perception. We don't do that, since we're going to run
HDR->LDR tonemapping algorithms.

Some great links about all this stuff:
- https://www.strollswithmydog.com/determining-forward-color-matrix/ (this whole blog is great)
- https://www.odelama.com/photo/Developing-a-RAW-Photo-by-hand/Developing-a-RAW-Photo-by-hand_Part-2/
- http://rawtherapee.com/mirror/dcamprof/camera-profiling.html#dng_profiles

