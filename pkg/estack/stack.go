package estack

import(
	"image"
	"image/color"
	"fmt"
	"log"
	"sort"
)

// A Stack of images to be combined. The images are sorted in order of
// ascending ExposureValue - the least noisy / slowest shutter speed /
// most likely to over-expose image comes first.
type Stack struct {
	Images           []StackedImage
	Configuration

	InputArea          image.Rectangle  // Which pixels we care about in the source images
	OutputArea         image.Rectangle  // The bounding box for the output

	Pixels        [][]*PixelWorkspace
	CompositeLDR      *image.RGBA64
}

func NewStack() Stack {
	return Stack{
		Images: []StackedImage{},
		Configuration: NewConfiguration(),
	}
}

func (s Stack)String() string {
	str := "Stack[\n"
	for _, si := range s.Images {
		str += fmt.Sprintf("  %s\n", si)
	}
	str += "]\n"
	return str
}

func (s *Stack)Add(si StackedImage) {
	s.Images = append(s.Images, si)
	sort.Slice(s.Images, func(i, j int) bool { return s.Images[i].EV < s.Images[j].EV })	
}

func (s *Stack)AlignAllImages() error {
	if ! s.Configuration.Rendering.AlignEclipse {
		s.NullAlignAllImages()
		return nil
	}
		
	// Compute the lunar limbs (bounding boxes around the moon)
	for i:=0; i<len(s.Images); i++ {
		s.Images[i].LunarLimb = FindLunarLimb(s.Images[i])
	}

	center := s.Images[0].LunarLimb.Center()
	width := int( float64(s.GetSolarRadiusPixels()) * s.Configuration.Rendering.OutputWidthInSolarDiameters)
	s.InputArea = image.Rectangle{
		Min: image.Point{center.X - width, center.Y - width},
		Max: image.Point{center.X + width, center.Y + width},
	}

	AlignBaseImage(s, &s.Images[0])

	// Figure out the transforms to map points from the base/first image to the other images
	for i:=1; i<len(s.Images); i++ {
		AlignStackedImage(s, &s.Images[0], &s.Images[i])
	}

	//s.DumpLayerImages()
	
	return nil
}

func (s *Stack)NullAlignAllImages() {
	// No transforms, set input window to whole image
	for i:=0; i<len(s.Images); i++ {
		s.Images[i].XImage = s.Images[i].OrigImage
	}

	s.InputArea = s.Images[0].XImage.Bounds()
	log.Printf("No alignment or lunar detection performed; input area == original image dimensions == %s\n", s.InputArea)

	if true {
		s.InputArea.Max = RectCenter(s.InputArea)
		log.Printf("*** Trimming input to top left quadrant ***\n")
	}
}

func (s *Stack)DumpLayerImages() {
	for i:=0; i<len(s.Images); i++ {
		bounds := s.Images[i].XImage.Bounds()
		img := image.NewRGBA(bounds)

		for x:=bounds.Min.X; x<bounds.Max.X; x++ {
			for y:=bounds.Min.Y; y<bounds.Max.Y; y++ {
				// Fake up a pixel, so we can pretend to fuse it and get comparable output values
				rgb := s.Images[i].XImage.At(x,y)
				ws := PixelWorkspace{
					Config:                   s.Configuration,
					RawInputs:              []color.Color{rgb},
					Inputs:                 []BalancedCameraNativeRGB{},
					DimmestExposureInStack:   s.Images[len(s.Images)-1].ExposureValue,
				}
				wbRgb := ws.NewBalancedCameraNativeRGB(rgb, s.Images[i].ExposureValue)
				ws.Inputs = append(ws.Inputs, wbRgb)
				
				ws.FuseExposures() // Consistent color across all layers

				img.Set(x, y, color.RGBA64{
					uint16(ws.FusedRGB.F64[0] * float64(0xFFFF)),
					uint16(ws.FusedRGB.F64[1] * float64(0xFFFF)),
					uint16(ws.FusedRGB.F64[2] * float64(0xFFFF)),
					0xffff,
				})
			}
		}

		WritePNG(img, fmt.Sprintf("layer-processed-%02d.png", i))
		WritePNG(s.Images[i].XImage, fmt.Sprintf("layer-%02d.png", i))
	}
}


// These lookups work post-alignment, and are based on the first image's lunar limb

func (s *Stack)GetSolarRadiusPixels() int {
	return s.Images[0].LunarLimb.Radius() + 3 // The solar radius is a tiny bit bigger than the lunar radius.
}

func (s *Stack)GetViewportCenter() image.Point { return RectCenter(s.InputArea) }

func RectCenter(b image.Rectangle) image.Point {
	return image.Point{(b.Min.X + b.Max.X) / 2, (b.Min.Y + b.Max.Y) / 2}
}
