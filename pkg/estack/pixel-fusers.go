package estack

import(
	"math"
)

func FuseBySingleMostExposed(ws *PixelWorkspace) {
	max := 0.8 // pixel is too exposed if any channel recorded more than this (range [0.0, 1.0])

	// The images are pre-sorted in asc EV; slowest exposures first, most likely to over-expose.
	for i:=0; i<len(ws.Inputs); i++ {
		// If this looks too exposed, and we have less-exposed layers left, move on.
		if i < len(ws.Inputs)-1 {
			if ws.Inputs[i].F64[0] > max || ws.Inputs[i].F64[1] > max || ws.Inputs[i].F64[2] > max {
				continue
			}
		}

		ws.LayerMostUsed = i
		ws.FusedRGB = ws.Inputs[i]

		return
	}
}

// FuseBySector cuts up the image into pie slices, and simply picks
// a source layer based on which pie segment the pixel lies inside.
// It's useful for kinda comparing all the source images together, and
// seeing how well they align.
func FuseBySector(ws *PixelWorkspace) {
	pos                 := ws.InputPos
	thetaRadians        := math.Atan2(float64(pos.Y-ws.LunarCenter.Y), float64(pos.X-ws.LunarCenter.X))
	thetaDegrees        := 180 + thetaRadians * 180.0 / math.Pi
	numSegmentsPerLayer := 5
	numSegments         := len(ws.Inputs) * numSegmentsPerLayer
	segmentWidth        := 360.0 / float64(numSegments)
	thisSegment         := int(thetaDegrees / segmentWidth)

	ws.LayerMostUsed = thisSegment % len(ws.Inputs)
	ws.FusedRGB = ws.Inputs[ws.LayerMostUsed]
}

// Average all the non-overexposed pixels together
func FuseByAverage(ws *PixelWorkspace) {
	max := 0.8 // pixel is too exposed if any channel recorded more than this (range [0.0, 1.0])

	toAvg := []BalancedCameraNativeRGB{}

	// The images are pre-sorted in asc EV; slowest exposures first, most likely to over-expose.
	for i:=0; i<len(ws.Inputs); i++ {
		// If this looks too exposed, and we have less-exposed layers left, move on.
		if i < len(ws.Inputs)-1 {
			if ws.Inputs[i].F64[0] > max || ws.Inputs[i].F64[1] > max || ws.Inputs[i].F64[2] > max {
				continue
			}
		}

		toAvg = append(toAvg, ws.Inputs[i])
	}

	ws.FusedRGB = averageBalancedCameraNativeRGBs(ws, toAvg)
	ws.LayerMostUsed = len(toAvg) // Not really 'most used'
}


func averageBalancedCameraNativeRGBs(ws *PixelWorkspace, in []BalancedCameraNativeRGB) BalancedCameraNativeRGB {
	maxIllum := 0.0
	for i:=0; i<len(in); i++ {
		if in[i].IlluminanceAtMax > maxIllum { maxIllum = in[i].IlluminanceAtMax }
	}

	ret := BalancedCameraNativeRGB{IlluminanceAtMax:maxIllum}
	for j:=0; j<3; j++ {
		for i:=0; i<len(in); i++ {
			ret.F64[j] += (in[i].F64[j] * in[i].IlluminanceAtMax / maxIllum)
		}
		ret.F64[j] /= float64(len(in))
	}

	return ret
}
