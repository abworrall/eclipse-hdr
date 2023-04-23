package estack

import(
	"fmt"
	"image"
	"image/color"
	"log"
	"os"

	"github.com/mdouchement/hdr"
	"github.com/mdouchement/hdr/hdrcolor"
	"github.com/mdouchement/hdr/codec/rgbe"
)

// ImageStack is an image composed by exposure stacking some raw images.
// To be treated as an image, the pixelworkspaces must have completed the `fuse` step
type ImageStack struct {
	Stack  // If we proceed, copy all fields into here directly, retire Stack{}
}

// Implement golang's image.Image interface
func (is ImageStack)ColorModel() color.Model { return hdrcolor.RGBModel } //color.RGBA64Model }
func (is ImageStack)Bounds() image.Rectangle { return is.OutputArea }
func (is ImageStack)At(x, y int) color.Color { return is.HDRAt(x,y) }

// Implement hdr/hdrcolor.Color interface (which is a superset of the image/color.Color interface)
func (ws *PixelWorkspace)ToHDRColor() hdrcolor.Color { return ws.FusedRGB.ToHDRColor() }
func (ws *PixelWorkspace)ToHDRColorXYZ() hdrcolor.Color {
	return hdrcolor.XYZ{ws.FusedXYZ[0], ws.FusedXYZ[1], ws.FusedXYZ[2]}
}

// Implement hdr.Image interface
func (is ImageStack)HDRAt(x, y int) hdrcolor.Color { return is.Pixels[x][y].ToHDRColor() }
func (is ImageStack)Size() int                     { return is.Bounds().Dx() * is.Bounds().Dy() }

// Helper funcs - we use hdr's bundled RGB color impl
func (wbRgb BalancedCameraNativeRGB)ToHDRColor() hdrcolor.Color {
	return hdrcolor.RGB{wbRgb.F64[0], wbRgb.F64[1], wbRgb.F64[2]}
}




func (s *Stack)Playtime() {
	img := ImageStack{*s}

	WritePNG(img, "100-dump.png")

	err := WriteHDR(img, "102-dump.hdr")
	log.Printf("HDR write: er=%v\n", err)
}

func WriteHDR(img hdr.Image, filename string) error {
	if writer, err := os.Create(filename); err != nil {
		return fmt.Errorf("open+w '%s': %v", filename, err)
	} else {
		defer writer.Close()
		return rgbe.Encode(writer, img)
	}
}
