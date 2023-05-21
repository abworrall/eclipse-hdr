package ecolor

import(
	"fmt"
	"image/color"

	"github.com/mdouchement/hdr/hdrcolor"

	"github.com/abworrall/eclipse-hdr/pkg/emath"
)

// A CameraNative color is a sensor reading, combined with an exposure
// value, that has not yet been color corrected or white balanced. It
// exists in an RGB space specific to the camera.
type CameraNative struct {
	// The sensor photosites give values in the range [0, 0xFFFF]; we map those to [0.0, 1.0]
	hdrcolor.RGB // This field implements color.Color and hdrcolor.Color interfaces

	// How much Illuminance (in lux) is needed to generate a photosite value of 0xFFFF
	IllumAtMax     float64  
}

var(
	// Translates XYZ(D50) to sRGB(D65)
	//
	// https://sites.google.com/site/crossstereo/raw-converting/dng
	// http://www.brucelindbloom.com/index.html?Eqn_RGB_XYZ_Matrix.html
	//
	// We use the second table on Bruce Lindblooms's site; it bundles in
	// the chromatic adaptation transform that we need to move from D50
	// to D65 reference whites without seeing the image's white balance
	// shift. (Most XYZ->sRGB matrices on the web ignore the change to
	// reference white, so come out looking wrong)
	XYZD50_to_linear_sRGBD65 = emath.Mat3{
		 3.1338561, -1.6168667, -0.4906146,
    -0.9787684,  1.9161415,  0.0334540,
     0.0719453, -0.2289914,  1.4052427,
	}
)

// Treats the input RGB channels as [0, 0xFFFF]
func NewCameraNative(col color.Color, illumAtMax float64) CameraNative {
	r, g, b, _ := col.RGBA()

	return CameraNative{
		RGB: hdrcolor.RGB{
			R: float64(r) / float64(0xFFFF),
			G: float64(g) / float64(0xFFFF),
			B: float64(b) / float64(0xFFFF),
		},
		IllumAtMax: illumAtMax,
	}
}

func (cn CameraNative)String() string {
	return fmt.Sprintf("[%12.10f, %12.10f, %12.10f] @%.0f lumens", cn.RGB.R, cn.RGB.G, cn.RGB.B, cn.IllumAtMax)
}

// AdjustIllumAtMax rescales the RGB values.
func (cn *CameraNative)AdjustIllumAtMax(newIllumAtMax float64) {
	cn.RGB.R *= cn.IllumAtMax / newIllumAtMax
	cn.RGB.G *= cn.IllumAtMax / newIllumAtMax
	cn.RGB.B *= cn.IllumAtMax / newIllumAtMax
	cn.IllumAtMax = newIllumAtMax
}

// ApplyAsShotNeutral performs white balancing. After this operation,
// the color is no longer CameraNative, it is camera-neutral (i.e.
// white balanced), so return as arbitrary RGB.
func ApplyAsShotNeutral(cn CameraNative, asShotNeutral emath.Vec3) hdrcolor.RGB {
	return hdrcolor.RGB{
		R: cn.RGB.R / asShotNeutral[0],
		G: cn.RGB.G / asShotNeutral[1],
		B: cn.RGB.B / asShotNeutral[2],
	}
}

// ApplyForwardMatrix does all the color correction, assuming a DNG ForwardMatrix.
// The result is a camera-indepedent XYZ(D50) value.
func ApplyForwardMatrix(rgb hdrcolor.RGB, forwardMatrix emath.Mat3) hdrcolor.XYZ {
	xyz := forwardMatrix.Apply(emath.Vec3{rgb.R, rgb.G, rgb.B})
	return hdrcolor.XYZ{xyz[0], xyz[1], xyz[2]}
}

// This XYZToSRGB also adjusts reference white from D50 to D65. (The
// DNG ForwardMatrix maps CameraNative into XYZ(D50), but the standard
// sRGB output space assumes D65, so a chromatic adapation is needed.)
func XYZToSRGB(xyz hdrcolor.XYZ) hdrcolor.RGB {
	rgb := XYZD50_to_linear_sRGBD65.Apply(emath.Vec3{xyz.X, xyz.Y, xyz.Z})
	return hdrcolor.RGB{rgb[0], rgb[1], rgb[2]}
}

// DevelopDNG follows the DNG spec to perform white balance adjustment
// and color correction, to generate a color in XYZ(D50). You prob
// want to then convert that back down to an sRGB(D65) color with
// `ecolor.XYZToSRGB`
func (cn CameraNative)DevelopDNG(asShotNeutral emath.Vec3, forwardMatrix emath.Mat3) hdrcolor.XYZ {
	wbRgb  := ApplyAsShotNeutral(cn, asShotNeutral)
	xyzD50 := ApplyForwardMatrix(wbRgb, forwardMatrix)
	return xyzD50
}

// AverageBalancedCameraNativeRGBs accounts for the different exposures that
// each CameraNative may have
func AverageBalancedCameraNativeRGBs(in []CameraNative) CameraNative {
	maxIllum := 0.0
	for i:=0; i<len(in); i++ {
		if in[i].IllumAtMax > maxIllum { maxIllum = in[i].IllumAtMax }
	}

	ret := CameraNative{IllumAtMax: maxIllum}

	for i:=0; i<len(in); i++ {
		ret.RGB.R += (in[i].RGB.R * in[i].IllumAtMax / maxIllum)
		ret.RGB.G += (in[i].RGB.G * in[i].IllumAtMax / maxIllum)
		ret.RGB.B += (in[i].RGB.B * in[i].IllumAtMax / maxIllum)
	}

	ret.RGB.R /= float64(len(in))
	ret.RGB.G /= float64(len(in))
	ret.RGB.B /= float64(len(in))

	return ret
}

func HDRRGBFloorAt(c1 hdrcolor.RGB, min float64) hdrcolor.RGB {
	c2 := c1
	if c2.R < min { c2.R = min }
	if c2.G < min { c2.G = min }
	if c2.B < min { c2.B = min }
	return c2
}
