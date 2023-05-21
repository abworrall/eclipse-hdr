package eclipse

import(
	"image"
	"image/color"
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/mdouchement/hdr/codec/rgbe"
	"github.com/mdouchement/hdr/hdrcolor"

	"github.com/abworrall/eclipse-hdr/pkg/ecolor"
)

// FusedImage holds the image layers, and fuses them into a single
// image. Implements the image.Image interface.
type FusedImage struct {
	Layers           []Layer // Ordered, ascending EV (descending "number of photons needed to fully expose")
	Config

	InputArea          image.Rectangle  // Which pixels we care about in the (aligned) source images
	OutputArea         image.Rectangle  // The bounding box for the output

	Pixels           []Pixel
}

var DebugPixels = []image.Point{}

// Implement image.Image
func (fi FusedImage)ColorModel() color.Model       { return hdrcolor.RGBModel }
func (fi FusedImage)Bounds() image.Rectangle       { return fi.OutputArea }
func (fi FusedImage)At(x, y int) color.Color       { return fi.HDRAt(x,y) }

// Implement hdr.Image
func (fi FusedImage)HDRAt(x, y int) hdrcolor.Color { return fi.Pix(x,y).DevelopedRGB }
func (fi FusedImage)Size() int                     { return fi.Bounds().Dx() * fi.Bounds().Dy() }

// Pixel access
func (fi *FusedImage)Pix(x, y int) Pixel           { return fi.Pixels[x * fi.OutputArea.Dy() + y ] }
func (fi *FusedImage)PixRW(x, y int) *Pixel        { return &(fi.Pixels[x * fi.OutputArea.Dy() + y ]) }

func NewFusedImage() FusedImage {
	return FusedImage{
		Layers: []Layer{},
		Config: NewConfig(),
	}
}

func (fi FusedImage)String() string {
	str := fmt.Sprintf("FusedImage %s [\n", fi.OutputArea)
	for _, l := range fi.Layers {
		str += fmt.Sprintf("  %s\n", l)
	}
	return str + "]\n"
}

func (fi *FusedImage)AddLayer(l Layer) {
	fi.Layers = append(fi.Layers, l)
	sort.Slice(fi.Layers, func(i, j int) bool { return fi.Layers[i].EV < fi.Layers[j].EV })	
}

// Align does all the work to figure out how to align the various
// layers, and generates the final transformed image for each layer.
func (fi *FusedImage)Align() {
	log.Printf("Aligning image layers")

	if fi.Config.DoEclipseAlignment {
		for i:=0; i<len(fi.Layers); i++ {
			fi.Layers[i].LunarLimb = FindLunarLimb(fi.Config, fi.Layers[i].LoadedImage)
		}
		fi.InputArea  = fi.CalculateInputArea()
		fi.Config.InputArea = fi.InputArea // aligner needs this
		
		// Figure out the transforms to map points from the base/first image to the other images
		for i:=1; i<len(fi.Layers); i++ {
			AlignLayer(fi.Config, &fi.Layers[0], &fi.Layers[i])
		}

		if fi.Config.DoFineTunedAlignment {
			log.Printf("Fine tune alignments:-\n\n%s\n", fi.Config.AsYaml())
		}
		
	} else {
		fi.InputArea = fi.Layers[0].Image.Bounds() // default to whole image
	}

	// Figure out which area of the input we're going to process, in both input coords and output coords
	fi.OutputArea = image.Rectangle{ Max:image.Point{fi.InputArea.Dx(), fi.InputArea.Dy()} } 
	fi.Config.OutputArea = fi.OutputArea // Copy it into the config, so PixelFuncs can see it, sigh

	log.Printf("Layers loaded and aligned: %s", fi)
}

// Fuse looks at the various layers for each pixel, and figures out a
// final merged value for that pixel. There are a few algorithms to
// pick from. Then it normalizes the brightness, so each pixel has the
// same EV. Finally it does color development, white balance etc.
func (fi *FusedImage)Fuse() {
	log.Printf("Fusing image layers over %s", fi.OutputArea)
	fi.Pixels = make([]Pixel, fi.OutputArea.Dx() * fi.OutputArea.Dy())
	
	globalIllumAtMax := 0.0
	for x:=0; x<fi.OutputArea.Dx(); x++ {
		for y:=0; y<fi.OutputArea.Dy(); y++ {

			p := fi.PixRW(x, y) // Get a pointer to the Pixel, so we can mutate it

			p.OutputPos = image.Point{x, y}
			p.RawInputs = make([]color.Color, len(fi.Layers))
			p.In = make([]ecolor.CameraNative, len(fi.Layers))

			// Gather the inputs from all the layers
			for i:=0; i<len(fi.Layers); i++ {
				p.RawInputs[i] = fi.Layers[i].Image.At(x + fi.InputArea.Min.X, y + fi.InputArea.Min.Y)
				p.In[i] = ecolor.NewCameraNative(p.RawInputs[i], fi.Layers[i].ExposureValue.IlluminanceAtMaxExposure)
			}

			// Now run the fuser
			fuser := fi.Config.GetFuser()
			fuser(fi.Config, p)

			if p.Fused.IllumAtMax > globalIllumAtMax {
				globalIllumAtMax = p.Fused.IllumAtMax
			}
		}
	}

	for x:=0; x<fi.OutputArea.Dx(); x++ {
		for y:=0; y<fi.OutputArea.Dy(); y++ {
			p := fi.PixRW(x, y)

			p.Fused.AdjustIllumAtMax(globalIllumAtMax) 	 // Adjust all the pixels to the same max illuminance.
			developer := fi.Config.GetDeveloper()
			developer(fi.Config, p)                      // "Develop" the pixel (white balance etc.)
		}
	}

	for _, pt := range DebugPixels {
		log.Printf("%s", fi.Pix(pt.X, pt.Y))
	}
}

// WriteToHDR outputs a HDR image. You can load this into photoshop or other HDR tools.
func (fi *FusedImage)WriteToHDR(filename string) error {
	if writer, err := os.Create(filename); err != nil {
		return fmt.Errorf("FusedImage.WriteToHDR, open+w '%s': %v", filename, err)
	} else {
		defer writer.Close()
		err := rgbe.Encode(writer, fi)
		if err != nil {
			log.Printf("FusedImage.WriteToHDR, encoding RGBE file: %v\n", err)
		}
		return err
	}
}

func (fi *FusedImage)CalculateInputArea() image.Rectangle {
	// Figure out which area of the input we're going to process, in both input coords and output coords
	center    := fi.Layers[0].LunarLimb.Center()
	radiusPix := fi.Layers[0].LunarLimb.Radius() + 3
	width     := int( float64(radiusPix) * fi.Config.OutputWidthInSolarDiameters)
	bounds    := image.Rectangle{
		Min: image.Point{center.X - width, center.Y - width},
		Max: image.Point{center.X + width, center.Y + width},
	}

	// if !bounds.In(fi.Layers[0].LoadedImage.Bounds()) {}

	return bounds
}
