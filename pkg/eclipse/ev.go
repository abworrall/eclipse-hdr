package eclipse

import (
	"fmt"
)

type rational [2]int64

// An ExposureValue details how the photograph was exposed, and allows
// us to figure out how much physical illumination (cd/m^2) was
// hitting the sensor, given a pixel color from the image.
type ExposureValue struct {
	ISO                        int64    // 100, 800, etc.
	ApertureX10                int64    // f/5.6 is the integer 56.
	ShutterSpeed               rational // 1/500, 1/1000, etc.
	EV                         int64    // The final EV value

	// This is the only value used downstream; it is used to scale the
	// pixel values during image fusion.
	IlluminanceAtMaxExposure   float64  // How many lux generate a channel exposure >= 0xFFFF
}

var(
	// A quick lookup from the Fstop & shutterspeed into one of the
	// standard EV numbers (they all assume ISO100; https://en.wikipedia.org/wiki/Exposure_value)
	// This is super incomplete.
	EVLookup = map[int64]map[rational]int64{ // [FnumberX10][ShutterSpeed] == EV
		56: map[rational]int64{
			rational{1,2000} : 16,
			rational{1,1000} : 15,
			rational{1, 500} : 14,
			rational{1, 250} : 13,
			rational{1, 125} : 12,
		},

		110: map[rational]int64{
			rational{1,2000} : 18,
			rational{1,1000} : 17,
			rational{1, 500} : 16,
			rational{1, 250} : 15,
			rational{1, 125} : 14,
			rational{1,  60} : 13,
			rational{1,  30} : 12,
			rational{1,  15} : 11,
			rational{1,   8} : 10,
		},
	}
	
	// https://en.wikipedia.org/wiki/Exposure_value#EV_as_a_measure_of_luminance_and_illuminance
	// Maps the EV to Illuminance, the max incident illumination at the
	// sensor, measured in Lux (lumens/m^2).
	illuminanceLookup = map[int64]float64 {
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
	if _, ok := EVLookup[ev.ApertureX10]; !ok {
		return fmt.Errorf("(%s) had unhandled aperture", ev)

	} else if base, ok := EVLookup[ev.ApertureX10][ev.ShutterSpeed]; !ok {
		return fmt.Errorf("(%s) had unhandled shutterspeed", ev)

	} else {
		// Adjust for ISO; the higher the ISO, the less physical light
		// needed to fully expose. (As ISO goes up, the camera generally
		// just waits for less time and multiplies the photon-count
		// accordingly)
		switch ev.ISO {
		case  100:
		case  200: base -= 1
		case  400: base -= 2
		case  800: base -= 3
		case 1600: base -= 4
		case 3200: base -= 5
		default: return fmt.Errorf("(%s) had unhandled ISO", ev)
		}
		
		ev.EV = base
		ev.IlluminanceAtMaxExposure = illuminanceLookup[base]
	}

	return nil
}
