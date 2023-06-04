package eclipse

import (
	"fmt"
)

type rat64 [2]int64

// An ExposureValue details how the photograph was exposed, and allows
// us to figure out how much physical illumination (cd/m^2) was
// hitting the sensor, given a pixel color from the image.
//
// This type figures out an 'EV' value, basicaly how many 'stops'; but
// it rounds off the shutterspeed & aperture-fnumber to the values
// that are "whole" stops.
type ExposureValue struct {
	ISO                        int    // 100, 800, etc.
	ApertureX10                int    // f/5.6 is the integer 56.
	ShutterSpeed               rat64  // 1/500, 1/1000, etc.
	EV                         int    // The final EV value - https://en.wikipedia.org/wiki/Exposure_value

	// This is the only value used downstream; it is used to scale the
	// pixel values during image fusion.
	IlluminanceAtMaxExposure   float64  // How many lux generate a channel exposure == 0xFFFF
}

var(
	// The sequence of "whole" f-stops from f/1.0 to f/32, as x10 int values
	apertureX10FStops = []int{10, 14, 20, 28, 40, 56, 80, 110, 160, 220, 320}

	// This sequence isn't quite mathematical
	shutterSpeeds = []rat64{
		rat64{1, 4000},
		rat64{1, 2000},
		rat64{1, 1000},
		rat64{1,  500},
		rat64{1,  250},
		rat64{1,  125},
		rat64{1,   60},
		rat64{1,   30},
		rat64{1,   15},
		rat64{1,    8},
		rat64{1,    4},
		rat64{1,    2},
		rat64{1,    1},
		rat64{2,    1},
		rat64{4,    1},
		rat64{8,    1},
		rat64{16,   1},
		rat64{32,   1},
		rat64{64,   1}, // Surely no exposure will be longer than 64 seconds ...
	}
	
	// https://en.wikipedia.org/wiki/Exposure_value#EV_as_a_measure_of_luminance_and_illuminance
	// Maps the EV to Illuminance, the max incident illumination at the
	// sensor, measured in Lux (lumens/m^2).
	illuminanceLookup = map[int]float64 {
		6:	   160.0,
		7:     320.0,
		8:     640.0,
		9:    1280.0,
		10:   2560.0,
		11:   5120.0,
		12:	 10240.0,
		13:	 20480.0,
		14:	 40960.0,
		15:	 81920.0,
		16:	163840.0,
		17:	327680.0,
		18:	655360.0,
	}
)

// An aperture index doesn't have meaning per se, but the distance
// between two of them does - e.g. differ by 2, then the respective
// apertures differ by 2 'stops'
func closestApertureIndex(apertureX10 int) int {
	ret := 0
	for i, fstop := range apertureX10FStops {
		if fstop <= apertureX10 {
			ret = i
		}
	}
	return ret
}

func closestShutterSpeedIndex(ssIn rat64) int {
	ret := 0
	for i, ss := range shutterSpeeds {
		if ssIn[0] >= ss[0] && ss[1] >= ssIn[1] {
			ret = i
		}
	}
	return ret
}

func (ev ExposureValue)String() string {
	s := fmt.Sprintf("f/%.1f", float32(ev.ApertureX10)/10.0)
	if ev.ShutterSpeed[1] != 1 {
		s += fmt.Sprintf(", %d/%4d", ev.ShutterSpeed[0], ev.ShutterSpeed[1])
	} else {
		s += fmt.Sprintf(", %d", ev.ShutterSpeed[0])
	}
	s += fmt.Sprintf(", ISO%d", ev.ISO)
	return s + fmt.Sprintf(", EV %2d (%6.0f lux)", ev.EV, ev.IlluminanceAtMaxExposure)
}

func (ev *ExposureValue)Validate() error {
	// We know that f/5.6, at 1/4000, is EV=17; figure how we differ from this in stops.
	apAdj := closestApertureIndex(56)                 - closestApertureIndex(int(ev.ApertureX10))
	ssAdj := closestShutterSpeedIndex(rat64{1, 4000}) - closestShutterSpeedIndex(ev.ShutterSpeed)

	base := 17 - apAdj - ssAdj
	if base < 6 || base > 18 {
		return fmt.Errorf("Exposure info looks suspicous, base EV=%d: %v\n", base, ev)
	}

	// Adjust for ISO; the higher the ISO, the less physical light
	// needed to fully expose.
	switch ev.ISO {
	case   100: // do nothing
	case   200: base -= 1
	case   400: base -= 2
	case   800: base -= 3
	case  1600: base -= 4
	case  3200: base -= 5
	case  6400: base -= 6
	case 12800: base -= 7
	default: return fmt.Errorf("(%s) had unhandled ISO", ev)
	}
		
	ev.EV = base
	ev.IlluminanceAtMaxExposure = illuminanceLookup[base]

	return nil
}
