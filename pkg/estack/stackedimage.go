package estack

import (
	"fmt"
	"image"
)
		
// A StackedImage is an Image with extra methods that allow us to do exposure stacking.
type StackedImage struct {
	Filename string

	image.Image // The pixel data

	ExposureValue // The exposure value for the photo
	Offset image.Point // How offset this photo is from the first in the series
}

func (si StackedImage)String() string {
	return fmt.Sprintf("image '%s' - %s offset%v", si.Filename, si.ExposureValue, si.Offset)
}
