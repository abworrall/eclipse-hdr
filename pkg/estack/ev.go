package estack

import (
	"fmt"
)

type rational [2]int64

var(
	// Hand pick values I need right now
	// https://en.wikipedia.org/wiki/Exposure_value
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

	// How much physical light is needed to fully expose a pixel, in cd/m^2, for a given EV value
	LuminanceLookup = map[int64]int64{
		6:      8,
		7:     16,
		8:     32,
		9:     64,
		10:   128,
		11:   256,
		12:   512,
		13:  1024,
		14:  2048,
		15:  4096,
		16:  8192,
		17: 16384,
		18: 32768,
	}
)

// An ExposureValue represents how the photograph was exposed, and
// from that can derive some absolute measure of how much light was
// needed to fully expose a pixel (in candles/m^2)
type ExposureValue struct {
	ISO                 int64    // 100, 800, etc.
	ApertureX10         int64    // e.g. f/5.6 is the integer 56.
	ShutterSpeed        rational // .
	ExposureComp        int64    // in F-stops. Fractional stops not supported.

	EV                  int64    // The final EV value
	MaxLuminance        int64    // ... and the luminance it corresponds to
}

func (ev ExposureValue)String() string {
	s := fmt.Sprintf("f/%.1f", float32(ev.ApertureX10)/10.0)

	if ev.ShutterSpeed[1] != 1 {
		s += fmt.Sprintf(", %d/%d", ev.ShutterSpeed[0], ev.ShutterSpeed[1])
	} else {
		s += fmt.Sprintf(", %d", ev.ShutterSpeed[0])
	}

	s += fmt.Sprintf(", ISO%d", ev.ISO)

	if ev.ExposureComp != 0 {
		s += fmt.Sprintf("+%d stops ", ev.ExposureComp)
	}

	return s + fmt.Sprintf(" : EV %d (%d cd/m^2)", ev.EV, ev.MaxLuminance)
}

func (ev *ExposureValue)Validate() error {
	if _,ok := EVLookup[ev.ApertureX10]; !ok {
		return fmt.Errorf("(%s) had unhandled aperture", ev)
	}
	if base,ok := EVLookup[ev.ApertureX10][ev.ShutterSpeed]; !ok {
		return fmt.Errorf("(%s) had unhandled shutterspeed", ev)

	} else {
		// Adjust for ISO; the higher the ISO, the less physical light needed to fully expose
		switch ev.ISO {
		case  100:
		case  200: base -= 1
		case  400: base -= 2
		case  800: base -= 3
		case 1600: base -= 4
		case 3200: base -= 5
		default: return fmt.Errorf("(%s) had unhandled ISO", ev)
		}

		// Adjust for ExposureComp; the higher the comp, the less physical light needed
		base -= ev.ExposureComp
		
		ev.EV = base
		ev.MaxLuminance = LuminanceLookup[base]
	}

	return nil
}

/* Example EXIF dump from a 16-bit TIFF exported by lightroom from a DNG imported from a Nikon Df.

ApertureValue: "4970854/1000000"
SceneType: ""
SceneCaptureType: 0
Flash: 0
ColorSpace: 65535
FocalLengthIn35mmFilm: 480
MeteringMode: 5
LensModel: "200.0-500.0 mm f/5.6"
SamplesPerPixel: 3
PlanarConfiguration: 1
FNumber: "56/10"
DateTimeDigitized: "2017:08:21 11:34:53"
FocalPlaneYResolution: "44855751/32768"
FileSource: ""
DigitalZoomRatio: "1/1"
GainControl: 1
Compression: 1
XResolution: "72/1"
ExifIFDPointer: 15320
ShutterSpeedValue: "10965784/1000000"
ImageWidth: 4928
ResolutionUnit: 2
ExposureMode: 1
Saturation: 0
MaxApertureValue: "50/10"
FocalPlaneResolutionUnit: 3
SensingMethod: 2
Contrast: 0
ImageLength: 3280
DateTime: "2019:06:29 15:38:50"
ExposureProgram: 1
ExposureBiasValue: "-12/6"
SubjectDistanceRange: 0
ISOSpeedRatings: 800
DateTimeOriginal: "2017:08:21 11:34:53"
FocalLength: "4800/10"
SubSecTimeOriginal: "4"
PhotometricInterpretation: 2
Model: "NIKON Df"
YResolution: "72/1"
ExposureTime: "1/2000"
SubSecTimeDigitized: "40"
CFAPattern: ""
WhiteBalance: 0
FocalPlaneXResolution: "44855751/32768"
CustomRendered: 0
BitsPerSample: [16,16,16]
Make: "NIKON CORPORATION"
ExifVersion: "0231"
LightSource: 0

 */
