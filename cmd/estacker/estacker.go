package main

import(
	"flag"
	"log"
	"os"

	"github.com/abworrall/estacker/pkg/estack"
)

var(
	Log *log.Logger

	fOutputFilename string
	fOutputWidth float64
	fAlignEclipse bool
	fAlignFineTune bool
	fDNGDevelop bool
	fFuserStrategy string
	fTonemapperStrategy string
	fColorTweakerStrategy string
	fGamma bool
)

func init() {
	flag.StringVar(&fOutputFilename, "o", "out.png", "name of output image file")
	flag.Float64Var(&fOutputWidth, "width", 5, "width of output image, in solar diameters")
	flag.BoolVar(&fAlignEclipse, "aligneclipse", true, "assume pics are of an eclipse, and try to align them")
	flag.BoolVar(&fAlignFineTune, "alignfinetune", false, "do a very slow pass to finetune image alignment")
	flag.BoolVar(&fDNGDevelop, "develop", true, "apply DNG color corrections (ForwardMatrix etc)")

	flag.StringVar(&fFuserStrategy, "fuser", "mostexposed", "how to fuse the exposures into one HDR exposure")
	flag.StringVar(&fTonemapperStrategy, "tonemapper", "fattal02", "how to tonemap luminances to output pixels")
	flag.StringVar(&fColorTweakerStrategy, "colorer", "", "how to color pixels in the final image")
	flag.BoolVar(&fGamma, "gamma", true, "Apply sRGB standard gamma expansion on final image")
	flag.Parse()

	Log = log.New(os.Stdout,"", log.Ldate|log.Ltime)//log.Lshortfile
	log.Printf("Starting\n")
}

func main() {
	s := estack.NewStack()
	if err := s.Load(flag.Args()...); err != nil {
		Log.Fatal(err)
	}

	// Override the config file with command line args, if relevant
	if fOutputFilename != "" { s.Rendering.OutputFilename = fOutputFilename }
	if fOutputWidth > 0.0 { s.Rendering.OutputWidthInSolarDiameters = fOutputWidth }
	if fFuserStrategy != "" { s.Rendering.FuserStrategy = fFuserStrategy }
	if fTonemapperStrategy != "" { s.Rendering.TonemapperStrategy = fTonemapperStrategy }
	if fColorTweakerStrategy != "" { s.Rendering.ColorTweakerStrategy = fColorTweakerStrategy }

	// Just set the bool vars
	s.Rendering.AlignEclipse = fAlignEclipse
	s.Rendering.AlignmentFineTune = fAlignFineTune
	s.Rendering.DNGDevelop = fDNGDevelop
	s.Rendering.ApplyGammaExpansion = fGamma

	if err := s.AlignAllImages(); err != nil {
		log.Fatalf("AlignImages failed, err: %v\n", err)
	}

	log.Printf("Images loaded: %s", s)
	//log.Printf("Final configuration:-\n\n%s\n", s.Configuration.AsYaml())

	s.FuseExposures()

	s.Playtime() // FIXME

	s.Tonemap()
	s.DevelopAndPublish()
	
	estack.WritePNG(s.CompositeLDR, s.Rendering.OutputFilename)
	log.Printf("LDR output file written '%s'\n", s.Rendering.OutputFilename)
}
