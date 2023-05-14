package main

import(
	"flag"
	"log"

	"github.com/abworrall/eclipse-hdr/pkg/eclipse"
)

var(
	fVerbosity int
	fOutputWidth float64
	fDoEclipseAlignment bool
	fDoFineTunedAlignment bool
	fFuser string
	fDeveloper string
	fTonemapper string
	fFuserLuminance float64
)

func init() {
	flag.IntVar(&fVerbosity, "v", 0, "how verbose to get")
	flag.Float64Var(&fOutputWidth, "width", 4, "width of output image, in solar diameters")

	flag.BoolVar(&fDoEclipseAlignment, "aligneclipse", true, "assume pics are of an eclipse, and try to align them")
	flag.BoolVar(&fDoFineTunedAlignment, "alignfinetune", false, "do a very slow pass to finetune image alignment")

	flag.StringVar(&fFuser, "fuser", "mostexposed", "how to fuse the exposures into one HDR exposure")
	flag.StringVar(&fDeveloper, "developer", "dng", "how to develop the color (prior to tonemapping)")
	flag.StringVar(&fTonemapper, "tonemapper", "all", "how to tonemap from HDR to LDR: "+eclipse.ListTonemappers())
	flag.Float64Var(&fFuserLuminance, "fuserluminance", 0.8, "layer discarded during fusion if pixel>this (0.0->1.0) ")
	flag.Parse()

	log.Printf("eclipse-hdr starting\n")
}

func main() {
	img := eclipse.NewFusedImage()
	if err := img.LoadFilesAndDirs(flag.Args()...); err != nil {
		log.Fatal(err)
	}

	img.Config.Fuser = fFuser
	img.Config.Developer = fDeveloper
	img.Config.Tonemapper = fTonemapper
	img.Config.OutputWidthInSolarDiameters = fOutputWidth
	img.Config.DoEclipseAlignment = fDoEclipseAlignment
	img.Config.DoFineTunedAlignment = fDoFineTunedAlignment
	img.Config.Verbosity = fVerbosity
	img.Config.FuserLuminance = fFuserLuminance

	if img.Verbosity > 0 {
		log.Printf("Final configuration:-\n\n%s\n", img.Config.AsYaml())
	}

	img.Align()
	img.Fuse()
	img.WriteToHDR("fused.hdr")
	img.Tonemap()
}
