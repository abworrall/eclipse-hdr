package estack

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

func (s *Stack)Load(args ...string) error {	
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
				if err := s.Load(filepath.Join(arg, content.Name())); err != nil {
					return fmt.Errorf("load %s: %v", arg, err)
				}
			}

		default: // is a file, load it
			if err := s.LoadFile(arg); err != nil {
				return fmt.Errorf("loadfile %s: %v", arg, err)
			}
		}
	}

	return nil
}

func (s *Stack)LoadFile(filename string) error {
	ext := filepath.Ext(filename)

	switch strings.ToLower(ext) {

	case ".tif":
		si, err := LoadTIFF(filename)
		if err != nil {
			return fmt.Errorf("Loading %s as TIFF failed: %v", filename, err)
		}
		s.Add(si)

	case ".yaml":
		cfg, err := LoadConfiguration(filename)
		if err != nil {
			return fmt.Errorf("Loading %s as config YAML failed: %v", filename, err)
		}
		s.Configuration = cfg
		log.Printf("Loaded base configuration from %s\n", filename)
	}

	return nil
}


func LoadConfiguration(filename string) (Configuration, error) {
	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		return Configuration{}, fmt.Errorf("config read %s: %v", filename, err)
	}

	return NewConfigurationFromYaml(contents)
}


/*type TiffDumpWalker struct {}
func (tdw TiffDumpWalker)Walk(name exif.FieldName, tag *exiftiff.Tag) error {
	fmt.Printf("EXIF tag: %-30.30s: %s\n", name, tag)
	return nil
}*/

func LoadTIFF(filename string) (StackedImage, error) {
	si := StackedImage{LoadFilename: filename}

	// First, try to load the EXIF metadata.
	if reader, err := os.Open(filename); err != nil {
		return si, fmt.Errorf("open+r exif '%s': %v", filename, err)

	} else if ex, err := exif.Decode(reader); err != nil {
		return si, fmt.Errorf("exif parsing '%s': %v", filename, err)

	} else {
		//ex.Walk(TiffDumpWalker{})

		if tag,err := ex.Get(exif.ISOSpeedRatings); err != nil {
			return si, fmt.Errorf("exif ISO '%s': %v", filename, err)
		} else if val,err := tag.Int64(0); err != nil {
			return si, fmt.Errorf("exif ISO '%s': %v", filename, err)
		} else {
			si.ExposureValue.ISO = val
		}

		if tag,err := ex.Get(exif.FNumber); err != nil {
			return si, fmt.Errorf("exif FNumber '%s': %v", filename, err)
		} else if num,denom,err := tag.Rat2(0); err != nil {
			return si, fmt.Errorf("exif FNumber '%s': %v", filename, err)
		} else {
			switch denom {
			case 10: si.ApertureX10 = num
			case  5: si.ApertureX10 = num * 2
			case  1: si.ApertureX10 = num * 10
			default:
				return si, fmt.Errorf("exif FNumber denom '%s' unhandled '%d/%d'", filename, num, denom)
			}
		}

		if tag,err := ex.Get(exif.ExposureTime); err != nil {
			return si, fmt.Errorf("exif ExposureTime '%s': %v", filename, err)
		} else if num,denom,err := tag.Rat2(0); err != nil {
			return si, fmt.Errorf("exif ExposureTime '%s': %v", filename, err)
		} else {
			si.ShutterSpeed = rational{num,denom}
		}

		// Note: we ignore Exposure Compensation, as it is informational. The
		// Fstop/Speed/ISO triple fully defines how much light should expose a pixel.
		
		if err := si.ExposureValue.Validate(); err != nil {
			return si, fmt.Errorf("image '%s' EV: %v", filename, err)
		}
	}

	// Re-open the file, now for the image data
	if reader, err := os.Open(filename); err != nil {
		return si, fmt.Errorf("open+r img '%s': %v", filename, err)
	} else if img, err := tiff.Decode(reader); err != nil {
		return si, fmt.Errorf("tiff loading '%s': %v", filename, err)
	} else {
		si.OrigImage = img
	}
	
	return si, nil
}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
