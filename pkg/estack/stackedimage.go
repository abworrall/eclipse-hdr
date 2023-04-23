package estack

import (
	"fmt"
	"image"
	"path/filepath"
)
		
// A StackedImage is an Image with extra methods that allow us to do exposure stacking.
type StackedImage struct {
	LoadFilename       string
	OrigImage          image.Image  // The original photo image
	ExposureValue                   // The exposure value for the photo

	LunarLimb                       // Our guess at where the moon is in the photo
	AlignmentTransform              // How to map a point from the base image into this image

	// A pixel location at XImage[x,y] should refer to the same sky position across all stackedimages
	XImage             image.Image
}

func (si StackedImage)String() string {
	return fmt.Sprintf("%s: %s, xform%s, lunar radius %d, lunar brightness 0x%004x",
		si.Filename(), si.ExposureValue.String(),
		si.AlignmentTransform, si.LunarLimb.Radius(), si.LunarLimb.Brightness)
}

func (si StackedImage)Filename() string {
	return filepath.Base(si.LoadFilename)
}
