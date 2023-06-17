package eclipse

import(
	"fmt"
	"log"

	"github.com/mdouchement/hdr/tmo"

	"github.com/abworrall/eclipse-hdr/pkg/fattal02"
)

var(
	Tonemappers = []string{"drago03", "durand", "fattal02", "icam06", "linear", "reinhard05"}
)

func ListTonemappers() string {
	return fmt.Sprintf("%v", Tonemappers)
}

func (fi *FusedImage)Tonemap() {
	if fi.Config.Tonemapper == "all" {
		log.Printf("Tonemapping (using all operators)")
		for _, name := range Tonemappers {
			op := fi.SetupTonemapper(name)
			fi.ApplyTonemapper(op, name)
		}
	} else {
		op := fi.SetupTonemapper(fi.Config.Tonemapper)
		fi.ApplyTonemapper(op, fi.Config.Tonemapper)
	}
}

func (fi *FusedImage)ApplyTonemapper(op tmo.ToneMappingOperator, name string) {
	log.Printf("Tonemapping: %s", name)
	newImg := op.Perform()
	
	WritePNG(newImg, fmt.Sprintf("tmo-%s.png", name))

	for x:=0; x<fi.Bounds().Dx(); x++ {
		for y:=0; y<fi.Bounds().Dy(); y++ {
			p := fi.PixRW(x, y)
			p.TonemappedRGB = newImg.At(x, y)
		}
	}	
}

// Tweak the tmo parameters to better handle eclipse photos. By default, they
// almost always overexpose on the small but important bright areas.
func (fi *FusedImage)SetupTonemapper(name string) tmo.ToneMappingOperator {
	switch name {
	case "drago03":
		op :=  tmo.NewDefaultDrago03(fi)
		op.Bias = 1.0            // Otherwise image overexposes, blows out the bright corona
		return op

	case "durand":
		return tmo.NewDefaultDurand(fi)

	case "fattal02":
		op := fattal02.NewDefaultFattal02(fi)
		op.WhitePoint  = 0.00001 // We want as close to zero overexposed pixels	as we can get
		//op.DetailLevel = 1       // If <3, attenuation grids retain and highlight noise
		op.GammaExpand = true    // image comes out too dark otherwise
		if fi.Config.Verbosity > 0 {
			op.DumpGrids   = true
		}
		return op

	case "icam06":
		op := tmo.NewDefaultICam06(fi)
		op.Contrast    = 0.65
		op.MaxClipping = 0.99999 // Otherwise image overexposes, blows out the bright corona
		return op

	case "linear":
		return tmo.NewLinear(fi)

	case "reinhard05":
		op := tmo.NewDefaultReinhard05(fi)
		op.Chromatic  = 0.005
		op.Light      = 0.005    // Otherwise image overexposes, blows out the bright corona
		return op
	}

	log.Fatalf("ToneMapper %q not recognized, wanted %s\n", name, ListTonemappers())
	return nil
}
