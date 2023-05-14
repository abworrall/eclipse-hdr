package eclipse

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/rwcarlsen/goexif/exif"
	"golang.org/x/image/tiff"
)

func (fi *FusedImage)LoadFilesAndDirs(args ...string) (error) {
	for _, arg := range args {
		item, err := os.Stat(arg)

		switch {

		case err != nil:
			return fmt.Errorf("load %s: %v", arg, err)

		case item.IsDir():
			// Is a dir, recurse into contents
			contents, err := ioutil.ReadDir(arg)
			if err != nil {
				return fmt.Errorf("readdir %s: %v", arg, err)
			}
			for _, content := range contents {
				if err := fi.LoadFilesAndDirs(filepath.Join(arg, content.Name())); err != nil {
					return fmt.Errorf("load %s: %v", arg, err)
				}
			}

		default: // is a file, load it
			if err := fi.loadFile(arg); err != nil {
				return fmt.Errorf("loadfile %s: %v", arg, err)
			}
		}
	}

	return nil
}

func (fi *FusedImage)loadFile(filename string) error {
	ext := filepath.Ext(filename)

	switch strings.ToLower(ext) {

	case ".tif":
		layer, err := loadTIFF(filename)
		if err != nil {
			return fmt.Errorf("Loading %s as TIFF failed: %v", filename, err)
		}
		fi.AddLayer(layer)

	case ".yaml":
		cfg, err := loadConfig(filename)
		if err != nil {
			return fmt.Errorf("Loading %s as config YAML failed: %v", filename, err)
		}
		fi.Config = cfg
		log.Printf("Loaded base configuration from %s\n", filename)
	}

	return nil
}

func loadConfig(filename string) (Config, error) {
	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		return Config{}, fmt.Errorf("config read %s: %v", filename, err)
	}

	return newConfigFromYaml(contents)
}

func loadTIFF(filename string) (Layer, error) {
	l := Layer{LoadFilename: filename}

	// First, try to load the EXIF metadata.
	if reader, err := os.Open(filename); err != nil {
		return l, fmt.Errorf("open+r exif '%s': %v", filename, err)

	} else if ex, err := exif.Decode(reader); err != nil {
		return l, fmt.Errorf("exif parsing '%s': %v", filename, err)

	} else {
		if tag,err := ex.Get(exif.ISOSpeedRatings); err != nil {
			return l, fmt.Errorf("exif ISO '%s': %v", filename, err)
		} else if val,err := tag.Int64(0); err != nil {
			return l, fmt.Errorf("exif ISO '%s': %v", filename, err)
		} else {
			l.ExposureValue.ISO = val
		}

		if tag,err := ex.Get(exif.FNumber); err != nil {
			return l, fmt.Errorf("exif FNumber '%s': %v", filename, err)
		} else if num,denom,err := tag.Rat2(0); err != nil {
			return l, fmt.Errorf("exif FNumber '%s': %v", filename, err)
		} else {
			switch denom {
			case 10: l.ApertureX10 = num
			case  5: l.ApertureX10 = num * 2
			case  1: l.ApertureX10 = num * 10
			default:
				return l, fmt.Errorf("exif FNumber denom '%s' unhandled '%d/%d'", filename, num, denom)
			}
		}

		if tag,err := ex.Get(exif.ExposureTime); err != nil {
			return l, fmt.Errorf("exif ExposureTime '%s': %v", filename, err)
		} else if num,denom,err := tag.Rat2(0); err != nil {
			return l, fmt.Errorf("exif ExposureTime '%s': %v", filename, err)
		} else {
			l.ShutterSpeed = rational{num,denom}
		}

		// Note: we ignore Exposure Compensation, as it is informational. The
		// Fstop/Speed/ISO triple fully defines how much light would expose a pixel.
		
		if err := l.ExposureValue.Validate(); err != nil {
			return l, fmt.Errorf("image '%s' EV: %v", filename, err)
		}
	}

	// Re-open the file, now for the image data
	if reader, err := os.Open(filename); err != nil {
		return l, fmt.Errorf("open+r img '%s': %v", filename, err)
	} else if img, err := tiff.Decode(reader); err != nil {
		return l, fmt.Errorf("tiff loading '%s': %v", filename, err)
	} else {
		l.LoadedImage = img
		l.Image = l.LoadedImage // Default to no alignment
	}
	
	return l, nil
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
