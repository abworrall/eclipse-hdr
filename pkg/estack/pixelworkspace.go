package estack

import(
	"fmt"
	"image"
	"image/color"
)

var(
	DebugPixels = map[image.Point]bool{
		//image.Point{1530, 1263}: true,
	}
)

// As we walk through all the steps of handling a pixel, we use this data structure to hold
// all the interim values.
type PixelWorkspace struct {
	Config          Configuration
	LunarCenter     image.Point
	LunarRadiusPix  int

	InputPos        image.Point   // in coords of input image
	OutputPos       image.Point   // in coords of output image

	DimmestExposureInStack ExposureValue
	
	// The pipeline of color value conversions2
	RawInputs     []color.Color
	Inputs        []BalancedCameraNativeRGB
	FusedRGB        BalancedCameraNativeRGB
	FusedXYZ        XYZ_D50                  // FusedRGB mapped into the XYZ(D50) camera-independent colour space
	TonemappedRGB   BalancedCameraNativeRGB
	DevelopedRGB    Linear_sRGB_D65
	Output          color.RGBA64

	// Notes generated along the way
	LayerMostUsed   int // Which input layer most contributed to this pixel
}

// BalancedCameraNativeRGB is the color as read from the original
// image file (e.g. a "sensor" value, after demosaicing and
// linearization - dng_validate stage3), scaled to unit floats. The
// exposure info from the TIFF metadata is used to figure out
// IlluminanceAtMax
type BalancedCameraNativeRGB struct {
	F64                  MyVec3    // values mapped from [0, 0xFFFF] to range [0.0, 1.0], and white balanced
	IlluminanceAtMax     float64   // This is the illuminance (in lux) that causes the sensor to read 0xFFFF
}

// XYZ_D50 is a neutral color space (independent of camera etc.)
type XYZ_D50 MyVec3

// Linear_sRGB_D65 is a color in the sRGB color space (which assumes D65). It is linear, i.e. has not
// had the sRGB gamma expansion applied.
type Linear_sRGB_D65 MyVec3


func (bcnRGB BalancedCameraNativeRGB)String() string {
	return fmt.Sprintf("%s @%.0f lumens", bcnRGB.F64, bcnRGB.IlluminanceAtMax)
}
func (rgb Linear_sRGB_D65)String() string { return MyVec3(rgb).String() }
func (xyz XYZ_D50)String() string { return MyVec3(xyz).String() }


type PixelFunc func(*PixelWorkspace)

func (s *Stack)GetPixelWorkspaceForInputAt(x, y int) *PixelWorkspace {
	ws := PixelWorkspace{
		Config:                   s.Configuration,
		LunarCenter:              s.GetViewportCenter(),
		LunarRadiusPix:           s.GetSolarRadiusPixels(),
		InputPos:                 image.Point{x, y},
		RawInputs:              []color.Color{},
		Inputs:                 []BalancedCameraNativeRGB{},
		DimmestExposureInStack:   s.Images[len(s.Images)-1].ExposureValue,
	}

	// Sample all the images (all the .XImage are aligned, so (x,y) gets the same point in each one)
	for _, si := range s.Images {
		// 0. Apply white balance conversion
		rgb := si.XImage.At(x, y)
		wbRgb := ws.NewBalancedCameraNativeRGB(rgb, si.ExposureValue)
		ws.Inputs = append(ws.Inputs, wbRgb)
		ws.RawInputs = append(ws.RawInputs, rgb)
	}

	return &ws
}

// We white balance right away, doing all the work of the `D` matrix from the DNG spec.
func  (ws *PixelWorkspace)NewBalancedCameraNativeRGB(col color.Color, ev ExposureValue) BalancedCameraNativeRGB {
	ret := BalancedCameraNativeRGB{}
	r, g, b, _ := col.RGBA()

	ret.F64 = [3]float64{
		float64(r) / float64(0xFFFF) * (1.0 / ws.Config.AsShotNeutral[0]),
		float64(g) / float64(0xFFFF) * (1.0 / ws.Config.AsShotNeutral[1]),
		float64(b) / float64(0xFFFF) * (1.0 / ws.Config.AsShotNeutral[2]),
	}
	ret.IlluminanceAtMax = float64(ev.IlluminanceAtMaxExposure)

	return ret
}

func (ws *PixelWorkspace)FuseExposures() {
	// Run the specific algo to generate a single fused pixel
	fuser := ws.Config.GetFuser() 
	fuser(ws)

	// Each pixel scales [0.0, 1.0] to the ILluminanceAtMax from the
	// image the pixel came from. We want to normalize these, so pixels
	// can be compared; so rescale to the IlluminanceAtMax from the
	// dimmest / least exposed image (i.e. the one that has the most
	// photons to reach 0xFFFF)
	globalIlluminanceAtMax := ws.DimmestExposureInStack.IlluminanceAtMaxExposure
	ws.FusedRGB.F64[0] *= ws.FusedRGB.IlluminanceAtMax / globalIlluminanceAtMax
	ws.FusedRGB.F64[1] *= ws.FusedRGB.IlluminanceAtMax / globalIlluminanceAtMax
	ws.FusedRGB.F64[2] *= ws.FusedRGB.IlluminanceAtMax / globalIlluminanceAtMax

	ws.FusedRGB.IlluminanceAtMax = globalIlluminanceAtMax

	// Now map into the XYZ(D50) color space
	xyz := ws.Config.ForwardMatrix.Apply(ws.FusedRGB.F64)
	
	ws.FusedXYZ = XYZ_D50{xyz[0], xyz[1], xyz[2]}
}

func (ws *PixelWorkspace)DNGDevelop() {
	if ws.Config.Rendering.DNGDevelop {
		// White balance has already been done, so we ignore matrix `D`. Build a single matrix
		// that applies these two transformations (in this order):
		//  1: map from (white balanced camera native) to (XYZ at D50)   - this is what ForwardMatrix does
		//  2: map from (XYZ at D50) to (linear sRGB at D65)             - standard matrix you can find on the internet
		cam2sRgb := XYZD50_to_linear_sRGBD65.MatMult(ws.Config.ForwardMatrix)
		sRgb     := cam2sRgb.Apply(ws.TonemappedRGB.F64)

		ws.DevelopedRGB = Linear_sRGB_D65{sRgb[0], sRgb[1], sRgb[2]}

	} else {
		// Skip the color correction, just copy the values over
		ws.DevelopedRGB = Linear_sRGB_D65{ws.TonemappedRGB.F64[0], ws.TonemappedRGB.F64[1], ws.TonemappedRGB.F64[2]}
	}
}

func (ws *PixelWorkspace)Publish() {
	sRGB := MyVec3{ws.DevelopedRGB[0], ws.DevelopedRGB[1], ws.DevelopedRGB[2]}

	// THIS IS THE ONLY PLACE WE DO CLIPPING IN THE WHOLE PIPELINE
	sRGB.FloorAt(0.0)
	sRGB.CeilingAt(1.0)

	if ws.Config.Rendering.ApplyGammaExpansion {
		sRGB = GammaExpand_sRGB(sRGB)
	}

	ws.Output = color.RGBA64{
		uint16(sRGB[0] * float64(0xFFFF)),
		uint16(sRGB[1] * float64(0xFFFF)),
		uint16(sRGB[2] * float64(0xFFFF)),
		0xffff,
	}
}

func (ws *PixelWorkspace)ColorTweaks() {
	if tweaker := ws.Config.GetColorTweaker(); tweaker != nil {
		tweaker(ws)
	}
}


func (ws *PixelWorkspace)DebugDump() {
	fmt.Printf("\n----- DebugDump for pixel (%d,%d) -----\n", ws.OutputPos.X, ws.OutputPos.Y)

	fmt.Printf("Raw Inputs:-\n")
	for i:=0; i<len(ws.Inputs); i++ {
		r, g, b, _ := ws.RawInputs[i].RGBA()
		fmt.Printf("-- layer %d         : [      0x%04X,       0x%04X,       0x%04X]\n", i, r, g, b)
	}
	fmt.Printf("White Balanced Inputs:-\n")
	for i:=0; i<len(ws.Inputs); i++ {
		fmt.Printf("-- layer %d         : %s\n", i, ws.Inputs[i])
	}
	fmt.Printf("\n")
	fmt.Printf("FusedRGB           : %s (used layer %d)\n", ws.FusedRGB, ws.LayerMostUsed)
	fmt.Printf("FusedXYZ           : %s\n", ws.FusedXYZ)
	fmt.Printf("TonemappedRGB      : %s\n", ws.TonemappedRGB)
	fmt.Printf("DevelopedRGB       : %s\n", ws.DevelopedRGB)

	r, g, b, _ := ws.Output.RGBA()
	fmt.Printf("Output(RGB64)      : [      0x%04X,       0x%04X,       0x%04X]\n", r, g, b)

	r, g, b = r>>8, g>>8, b>>8
	fmt.Printf("Output(RGB32)      : [%12d, %12d, %12d]\n", r, g, b)
	fmt.Printf("\n")
}

// We map each pixel color value [0,0xFFFF] to a luminance
// [0,MaxLuminance], which lets us compare pixels between images that
// were taken at very different exposures - this is the first (easy)
// half of HDR.
//
// Noise: there is always noise, maybe a few bits per pixel. So we
// prefer pixels that have a signal above the noise floor, e.g. values
// > 0x0010 or so. To avoid noise, the best image for a pixel is the
// one with the lowest EV, the one that needed the fewest photons to
// fully saturate a pixel (e.g. the longest exposure time) - this
// is also the one most likely to over-expose.
//
// Linearity: the mapping ([0,0xFFFF] to [0,MaxLuminance]) starts to
// break down as the photo sites get more saturated. This means that
// the Luminance values we generate for a pixel differ a little
// between different photos, if the pixel is highly exposed in either
// of them. This difference manifests as visual 'seams' in the output
// image, drawing boundaries between the regions coming from different
// images. To avoid this, we consider a pixel over-exposed even at
// quite low levels (0x8000 - see CombineBestExposed)
