package eclipse

import (
	"fmt"
	"image"
	"path/filepath"
)

// A Layer holds an image.Image loaded from an input file, with extra stuff that allow us to fuse the exposures
type Layer struct {
	LoadFilename       string
	LoadedImage        image.Image  // The original photo image
	ExposureValue                   // The exposure value for the photo

	LunarLimb                       // Our guess at where the moon is in the photo
	AlignmentTransform              // How to map a point from the base image into this image

	// _This_ image is aligned across layers, so a pixel at [x,y] relates to the same bit of sky on every layer
	image.Image
}

func (l Layer)String() string {
	return fmt.Sprintf("%s: %s, xform%s, lunar radius %d, lunar brightness 0x%004x",
		l.Filename(), l.ExposureValue.String(), l.AlignmentTransform, l.LunarLimb.Radius(), l.LunarLimb.Brightness)
}

func (l Layer)Filename() string {
	return filepath.Base(l.LoadFilename)
}
