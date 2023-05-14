package eclipse

import(
	"fmt"
	"image"
	"image/color"

	"github.com/mdouchement/hdr/hdrcolor"

	"github.com/abworrall/eclipse-hdr/pkg/ecolor"
)

type Pixel struct {
	OutputPos     image.Point                        // In output coords
	RawInputs   []color.Color
	In          []ecolor.CameraNative

	Fused         ecolor.CameraNative                // The single CameraNative pixel fused from the source images
	DevelopedRGB  hdrcolor.RGB                       // The white balanced, color-corrected HDR RGB value
	TonemappedRGB color.Color                        // The final LDR output, after HDR->LDR tonemapping

	LayerNumber   int                                // which layer used; or how many layers used
}

func (p Pixel)String() string {
	str := fmt.Sprintf("----- Pixel @(%d,%d)-----\n", p.OutputPos.X, p.OutputPos.Y)

	str += fmt.Sprintf("Raw Inputs:-\n")
	for i:=0; i<len(p.RawInputs); i++ {
		r, g, b, _ := p.RawInputs[i].RGBA()
		str += fmt.Sprintf("-- layer %d         : [      0x%04X,       0x%04X,       0x%04X]\n", i, r, g, b)
	}

	str += fmt.Sprintf("CameraNative  Inputs:-\n")
	for i:=0; i<len(p.In); i++ {
		str += fmt.Sprintf("-- layer %d         : %s\n", i, p.In[i])
	}
	str += fmt.Sprintf("\n")

	str += fmt.Sprintf("Fused              : %s (layer# %d)\n", p.Fused, p.LayerNumber)
	str += fmt.Sprintf("DevelopedRGB       : [%12.10f, %12.10f, %12.10f]\n",
		p.DevelopedRGB.R, p.DevelopedRGB.G, p.DevelopedRGB.B)

	if p.TonemappedRGB != nil {
		r, g, b, _ := p.TonemappedRGB.RGBA()
		str += fmt.Sprintf("Output(RGB64)      : [      0x%04X,       0x%04X,       0x%04X]\n", r, g, b)
		r, g, b = r>>8, g>>8, b>>8
		str += fmt.Sprintf("Output(RGB32)      : [%12d, %12d, %12d]\n", r, g, b)
	}
	str += fmt.Sprintf("\n")

	return str
}

