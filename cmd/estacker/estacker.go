package main

import(
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/abworrall/estacker/pkg/estack"
)

var(
	Log *log.Logger

	fStackDir string
	fOutput string
	fCombinerStrategy string
	fMaxCandles float64
	fGamma  float64

	fAlignSlideshow bool
)

func init() {
	Log = log.New(os.Stdout,"", log.Ldate|log.Ltime)//log.Lshortfile
	Log.Printf("(starting estacker)\n")

	//flag.StringVar(&fConfigFile, "stack.yaml", "", "a YAML file of config data")
	flag.StringVar(&fStackDir, "d", ".", "the dir with the TIFFs and the stack.yaml")

	flag.StringVar(&fOutput, "output", "", "name of output image")

	flag.StringVar(&fCombinerStrategy, "combine", "", "how to combine images")
	flag.Float64Var(&fMaxCandles, "maxcandles", 0, "The level of illumination we consider 'white'")
	flag.Float64Var(&fGamma, "gamma", 0, "The encoding gamma level (<1)")
	flag.BoolVar(&fAlignSlideshow, "alignslideshow", false, "generate an alignment slideshow")
	flag.Parse() // https://gobyexample.com/command-line-flags
}

func alignSlideshow(in estack.Stack) {
	for layer:=0; layer<len(in.Images); layer++ {
		s := in
		s.Rendering.Combiner = estack.SelectFromOneLayer
		s.Rendering.SelectJustThisLayer = layer
		s.Rendering.OutputFilename = fmt.Sprintf("align-layer-%02d.png", layer)
		if err := s.RenderOutputFile(); err != nil {
			Log.Fatal("Rendering failed: %v\n", err)
		}
		Log.Printf("Output file written '%s'\n", s.Rendering.OutputFilename)
	}
}

func main() {
	s,err := estack.LoadStackDir(fStackDir)
	if err != nil {
		Log.Fatal(err)
	}

	// Override the config file with command line args, if relevant
	if fOutput != "" { s.Rendering.OutputFilename = fOutput }
	if fGamma > 0.0 { s.Rendering.Gamma = fGamma }
	if fMaxCandles > 0.0 { s.Rendering.MaxCandles = fMaxCandles }
	if fCombinerStrategy != "" {
		s.Rendering.CombinerStrategy = fCombinerStrategy
	}
	if err := s.Configuration.FinalizeConfiguration(); err != nil {
		Log.Fatalf("bad config: %v\n", err)
	}

	if err := s.AlignImages(); err != nil {
		Log.Fatalf("AlignImages: %v\n", err)
	}
	Log.Printf("(images aligned)\n")

	Log.Printf("loaded a stackdir:-\n%s", s)

	if fAlignSlideshow {
		alignSlideshow(s)
		return
	}
	
	if err := s.RenderOutputFile(); err != nil {
		Log.Fatal("Rendering failed: %v\n", err)
	}
	Log.Printf("Output file written '%s'\n", s.Rendering.OutputFilename)
}
