package estack

import "math"

// All of this stuff expects to operate on color channel values in the range [0, 1.0]

var(
	// Translates XYZ(D50) to sRGB(D65)
	// https://sites.google.com/site/crossstereo/raw-converting/dng
	// http://www.brucelindbloom.com/index.html?Eqn_RGB_XYZ_Matrix.html, second table
	XYZD50_to_linear_sRGBD65 = MyMat3{
		 3.1338561, -1.6168667, -0.4906146,
    -0.9787684,  1.9161415,  0.0334540,
     0.0719453, -0.2289914,  1.4052427,
	}
)

// This is redundant now, as we white balance earlier in the pipeline and so
// shouldn't use the `D` matrix
func Get_CamRGB_to_XYZD50(asShotNeutral MyVec3, forwardMatrix MyMat3) MyMat3 {
	D := asShotNeutral.InvertDiag()
	return forwardMatrix.MatMult(D)
}

// https://www.sjbrown.co.uk/posts/gamma-correct-rendering/ - "linear RGB to sRGB"
// Each channel in v1 is assumed to be in the range [0,1]
func GammaExpand_sRGB(v MyVec3) MyVec3 {
	return MyVec3{
		GammaExpand_F64(v[0]),
		GammaExpand_F64(v[1]),
		GammaExpand_F64(v[2]),
	}
}

func GammaExpand_F64(f float64) float64 {
	if f <= 0.0031308 {
		return 12.92 * f
	}
	return 1.055 * math.Pow(f, 1.0/2.4) - 0.055
}

// If we want to adjust the luminance, we need to trafirst translate
// from XYZ to xyY - only in xyY is the luminance fully orthogonal to
// the chromicity. (Changing Y in XYZ also change the color). Useful
// link: https://graphics.stanford.edu/courses/cs148-10-summer/docs/2010--kerr--cie_xyz.pdf

/*

Once we've combined pixels, we need to do some color processing -
basically do the same stuff that the DNG conversion process would
normally do _after_ stage3.

Quick cut-n-paste of internet comment explaining what the different output
stages of `dng_validate.exe` contain:

		[...] "stage 1" image to describe the raw image data, "stage 2" to
		describe the linearized raw image data (in a canonical space [0, 1],
		or [0, 65535] if you prefer), and "stage 3" to describe the 3-channel
		or 4-channel data (i.e., after demosaic). If you want to do
		processing on linearized mosaic data, you want to grab the stage 2
		image; see the Stage2Image () routine in the dng_negative class
		(dng_negative.h). If you want to do processing on RGB demosaiced data
		in the native camera color space, prior to white balance, you want
		the stage 3 image; see the Stage3Image () routine in the dng_negative
		class.

We want stage3 data:
- has been demosiaced and linearized (RGB triples, each channel in range [0,0xFFFF])
- RGB space is "camera native"
- has not been white balanced (so it all looks quite green)
- has not been color-corrected via the camera's matrices

To generate the final output image, we want to apply camera color
matrices, do white balance correction and then transform into the sRGB
color space complete with optional gamma expansion. Most external
tools will assume sRGB when they read in a TIFF.

The DNG spec has a useful description of how it performs these steps.
We're going to walk through the section "Mapping Camera Color Space to
CIE XYZ Space", pp. 85-88 in v1.6.0.0 of the spec, and make notes
about how it applies to the data we have in our DNGs (which we get via
`dng_validate.exe -v`)

Links about the DNG color processing stages:
- the spec: https://helpx.adobe.com/content/dam/help/en/photoshop/pdf/dng_spec_1_6_0_0.pdf
- a matlab implementation: https://sites.google.com/site/crossstereo/raw-converting/dng

Things we can ignore, given the data from our images:
- CC - `CameraCalibration` Matrices - they are all identity matrices for us
- AB - `AnalogBalance` - this is also identity

# page 86: Translating White Balance xy Coords to Camera Neutral Coords

The goal of this section: a 3x1 vector, a "camera neutral coordinate",
i.e. a perfect grey located in the camera native RGB coordinate space.

We have `AsShotNeutral`: "specifies the selected white balance at time
of capture, encoded as the coordinates of a perfectly neutral color in
linear reference space values." This _is_ the camera neutral
coordinate (`CameraNeutral` in the spec), so if we're happy to use the
white balance that the camera deduced at time of shooting, we can
skip this section. And we are :)

If we wanted to change the color temperature we'd need to do a whole
bunch of work here. In my sample photos - all of the same thing, the
corona - I saw a range of color temps assigned by the camera
(4800-5000K). (Note that the `dng_validate.exe -v` output omits the
color temp info; but if you run the `tiffinfo` tool against a TIFF
with all the EXIF data, it dumps out some XML that has a tag
<crs:Temperature/> that has the Kelvin color temp the camera assigned
to that image.


# pages 87-88: Camera to XYZ (D50) Transform

The goal of this section: a 3x3 matrix transform. Its input is an RGB
color in camera native space. The output is a color in the
camera-independent XYZ(D50) space (the XYZ space, anchored to a D50
illuminant - i.e. with a hardwired white balance corresponding to the
"warm daylight" D50 standard illuminant)

It's performing two distinct steps - apply a white balance correction
that will make the RGB value appear neutral at D50 - then map the RGB
color into an XYZ color.

We have `ForwardMatrix`: "defines a matrix that maps white balanced
camera colors to XYZ D50 colors.". This makes things really easy.

I'll quickly boil down the matrix combinations, given:
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

This makes intuitive sense.

The first part of the transform, the white balance correction D, will
thus scale the RGB values it takes as input, multiplying R by
1/AsShotNeutralR, G by 1/AsShotNeutralG, etc. Random internet comments
suggest doing this exact thing to apply a white balance correction on
camera native RGB, given an RGB that represents neutral/grey.

Then the `ForwardMatrix` will map the "white balanced camera colors"
over to XYZ(D50).


# Conversion back to RGB for TIFF output

We're going to render into the sRGB color space (there are other RGB spaces), as its the default

This is standard and well known, e.g.:
- https://www.sjbrown.co.uk/posts/gamma-correct-rendering/
- https://observablehq.com/@sebastien/srgb-rgb-gamma
- https://www.image-engineering.de/library/technotes/958-how-to-convert-between-srgb-and-ciexyz
- http://www.brucelindbloom.com/index.html?Eqn_RGB_XYZ_Matrix.html

We just apply the XYZtosRGB transform matrix.

The gamma step is optional, and happens right at the end of
everything. You'll want it if generating a final image for human
consumption; you won't want it if you're going to feed the image into
another piece of software for further processing.

Other good links about all this:
- https://www.strollswithmydog.com/determining-forward-color-matrix/ (this whole blog is great)
- https://www.odelama.com/photo/Developing-a-RAW-Photo-by-hand/Developing-a-RAW-Photo-by-hand_Part-2/
- http://rawtherapee.com/mirror/dcamprof/camera-profiling.html#dng_profiles


# Partial output from `dng_validate.exe -v` against one of my sample DNGs:

	ExifIFD: 24721836
	ImageNumber: 6015
	DNGVersion: 1.4.0.0
	DNGBackwardVersion: 1.1.0.0
	UniqueCameraModel: "Nikon Df"
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
	BaselineExposure: +0.25
	BaselineNoise: 0.60
	BaselineSharpness: 1.00
	LinearResponseLimit: 1.00
	CameraSerialNumber: "3001580"
	LensInfo: 200.0-500.0 mm f/5.6
	ShadowScale: 1.0000

	CalibrationIlluminant1: Standard light A
	CalibrationIlluminant2: D65

	CameraCalibrationSignature: "com.adobe"
	ProfileCalibrationSignature: "com.adobe"
	ProfileName: "Adobe Standard"
	ProfileHueSatMapDims: Hues = 90, Sats = 30, Vals = 1
	ProfileHueSatMapData1:
					 h [ 0] s [ 0]:  h=  2.0000 s=1.0000 v=1.0000
					 ....

	ForwardMatrix1:
						 0.6919   0.1772   0.0952
						 0.2645   0.8172  -0.0817
						 0.0392  -0.2442   1.0301
	ForwardMatrix2:
						 0.6227   0.3389   0.0026
						 0.2548   0.9378  -0.1926
						 0.0156  -0.1330   0.9425

 */
