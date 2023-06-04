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

	"github.com/abworrall/go-dng/pkg/dng"

	"github.com/abworrall/eclipse-hdr/pkg/ecolor"
	"github.com/abworrall/eclipse-hdr/pkg/emath"
)



func (fi *FusedImage)LoadFilesAndDirs(args ...string) (error) {
	if err := fi.loadThings(args...); err != nil {
		return err
	}

	// Now everything is loaded, tidy up config
	if len(fi.Layers) > 0 && fi.Layers[0].CameraToPCS[1] != 0.0 {
		log.Printf("Taking CameraWhite/CameraToPCS from DNG data in %s\n", fi.Layers[0].Filename())
		fi.Config.CameraWhite = fi.Layers[0].CameraWhite
		fi.Config.CameraToPCS = fi.Layers[0].CameraToPCS

	} else if fi.Config.ManualOverrideForwardMatrix[0] != 0.0 {
		log.Printf("Taking CameraWhite/CameraToPCS from manual overrides in config.yaml\n")
		fi.Config.CameraWhite = fi.Config.ManualOverrideAsShotNeutral
		fi.Config.CameraToPCS = ecolor.MakeCameraToPCS(fi.Config.ManualOverrideAsShotNeutral,
			fi.Config.ManualOverrideForwardMatrix)

	} else {
		return fmt.Errorf("No color correction info; need DNGs, or ManualOverride{AsShotNeutral,ForwardMatrix} in conf.yaml")
	}

	return nil
}

func (fi *FusedImage)loadThings(args ...string) (error) {
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
				if err := fi.loadThings(filepath.Join(arg, content.Name())); err != nil {
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

	case ".dng":
		layer, err := loadDNG(filename)
		if err != nil {
			return fmt.Errorf("Loading %s as DNG failed: %v", filename, err)
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

func loadDNG(filename string) (Layer, error) {
	l := Layer{LoadFilename: filename}

	img := dng.Image{ImageKind:dng.ImageStage3}
	if err := img.Load(filename); err != nil {
		return Layer{}, err
	}

	fnum := img.ExifFNumber()
	exposure := img.ExifExposureTime()

	l.ExposureValue.ISO = img.ExifISO()
	l.ApertureX10 = fNumberToX10(int(fnum[0]), int(fnum[1]))
	l.ShutterSpeed = rat64{int64(exposure[0]), int64(exposure[1])}

	l.CameraWhite = emath.Vec3(img.CameraWhite())
	l.CameraToPCS = emath.Mat3(img.CameraToPCS())
	
	if err := l.ExposureValue.Validate(); err != nil {
		return l, fmt.Errorf("image '%s' Invalid EV: %v", filename, err)
	}

	l.LoadedImage = img
	l.Image = l.LoadedImage // Default to no alignment (needed for first image ?) - FIXME, this is messy

	return l, nil
}

func loadTIFF(filename string) (Layer, error) {
	l := Layer{LoadFilename: filename}

	// First, try to load the EXIF metadata.
	if reader, err := os.Open(filename); err != nil {
		return l, fmt.Errorf("open+r exif '%s': %v", filename, err)

	} else if ex, err := exif.Decode(reader); err != nil {
		return l, fmt.Errorf("exif parsing '%s': %v", filename, err)

	} else {
		if tag, err := ex.Get(exif.ISOSpeedRatings); err != nil {
			return l, fmt.Errorf("exif ISO '%s': %v", filename, err)
		} else if val, err := tag.Int64(0); err != nil {
			return l, fmt.Errorf("exif ISO '%s': %v", filename, err)
		} else {
			l.ExposureValue.ISO = int(val)
		}

		if tag, err := ex.Get(exif.FNumber); err != nil {
			return l, fmt.Errorf("exif FNumber '%s': %v", filename, err)
		} else if num, denom, err := tag.Rat2(0); err != nil {
			return l, fmt.Errorf("exif FNumber '%s': %v", filename, err)
		} else {
			l.ApertureX10 = fNumberToX10(int(num), int(denom))
		}

		if tag, err := ex.Get(exif.ExposureTime); err != nil {
			return l, fmt.Errorf("exif ExposureTime '%s': %v", filename, err)
		} else if num, denom, err := tag.Rat2(0); err != nil {
			return l, fmt.Errorf("exif ExposureTime '%s': %v", filename, err)
		} else {
			l.ShutterSpeed = rat64{num,denom}
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
		l.Image = l.LoadedImage // Default to no alignment (needed for first image ?)
	}
	
	return l, nil
}

func fNumberToX10(num, denom int) int {
	switch denom {
	case 10: return num
	case  5: return num * 2
	case  1: return num * 10
	default:
		panic(fmt.Sprintf("exif FNumber denom unhandled '%d/%d'", num, denom))
	}
	return -1
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
