package estack

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/rwcarlsen/goexif/exif"
	"golang.org/x/image/tiff"
)

// {{{ LoadStackDir

// A 'stack dir' should contain a stack.yaml file (with config), and a bunch of TIFF files.
func LoadStackDir(d string) (Stack,error) {
	s := NewStack()
	
	files, err := ioutil.ReadDir(d)
	if err != nil {
		return s, fmt.Errorf("reading stackdir '%s': %v", d, err)
	}

	cfg,err := LoadConfiguration(filepath.Join(d, "stack.yaml"))
	if err != nil {
		return s, fmt.Errorf("Config error: %v", err)
	}
	s.Configuration = cfg
	fmt.Printf("(loaded config)\n");

	excluded :=map[string]int{}
	for _,f := range cfg.Exclude { excluded[f] = 1 }
	fmt.Printf("(excluding images %v)\n", excluded);	
	
	for _, f := range files {
		if _,exists := excluded[f.Name()]; exists {
			continue
		}
		ext := filepath.Ext(f.Name())
		if ext == ".tif" || ext == ".TIF" {
			if si, err := LoadTIFF(filepath.Join(d, f.Name())); err != nil {
				return s, fmt.Errorf("Loading failed: %v", err)
			} else {
				si.Filename = f.Name()
				s.Add(si)
				fmt.Printf("(loaded image %s)\n", f.Name());
			}
		}
	}
	
	// Default bounds to the first input image
	if cfg.Rendering.Bounds.Dx() == 0 || cfg.Rendering.Bounds.Dy() == 0 {
			s.Configuration.Rendering.Bounds = s.Images[0].Bounds() // 4298 x 3280
	}

	return s,nil
}

// }}}
// {{{ LoadTIFF

func LoadTIFF(filename string) (StackedImage, error) {
	si := StackedImage{Filename: filename}

	imgFilename := filename
	
	// First, try to load the EXIF metadata.
	if reader, err := os.Open(filename); err != nil {
		return si, fmt.Errorf("open+r exif '%s': %v", filename, err)

	} else if ex, err := exif.Decode(reader); err != nil {
		return si, fmt.Errorf("exif parsing '%s': %v", filename, err)

	} else {
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

		// TODO: extract exposure compensation value
		
		if err := si.ExposureValue.Validate(); err != nil {
			return si, fmt.Errorf("image '%s' EV: %v", filename, err)
		}
	}

	// Re-open the file, now for the image data
	if reader, err := os.Open(imgFilename); err != nil {
		return si, fmt.Errorf("open+r img '%s': %v", imgFilename, err)
	} else if img, err := tiff.Decode(reader); err != nil {
		return si, fmt.Errorf("tiff loading '%s': %v", imgFilename, err)
	} else {
		si.Image = img
	}

	return si, nil
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
