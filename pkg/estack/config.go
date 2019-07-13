package estack

import(
	"fmt"
	"image"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

/* Example config file ...

rendering:
  outputfilename: out.png
  gamma: 0.25
  maxcandles: 512
  combinerstrategy: quad
  bounds:
    min:
      x: 500
      y: 500
    max:
      x: 1500
      y: 1500

alignment:
  5662.tif: [  0,  0]
  5663.tif: [ -5,  0]
  5664.tif: [ -7,  3]
  5665.tif: [-10,  5]
  5666.tif: [-14,  6]

*/

type RenderOptions struct {
	// Values from command line
	OutputFilename      string
	Gamma               float64
	MaxCandles          float64
	CombinerStrategy    string
	RadialExponent      float64
	SolarRadiusPixels   int

	// Values we derive/compute
	Combiner            CombinerFunc
	Bounds              image.Rectangle  // The part of the base image we process

	// Hack values
	SelectJustThisLayer int // used by a hack combiner
}

type Configuration struct {
	Rendering    RenderOptions
	Alignment    AlignmentData
	Exclude      []string
}

func NewConfiguration() Configuration {
	return Configuration{
		Alignment: NewAlignmentData(),
		Exclude: []string{},
	}
}

func LoadConfiguration(filename string) (Configuration, error) {
	c := NewConfiguration()

	if contents,err := ioutil.ReadFile(filename); err != nil {
		return c, fmt.Errorf("read '%f': %v", filename, err)
	} else if err := yaml.Unmarshal([]byte(contents), &c); err != nil {
		return c, fmt.Errorf("parse '%f': %v", filename, err)
	}

	return c, c.FinalizeConfiguration()
}

// FinalizeConfiguration does sanity checks and other post-processing
func (c *Configuration)FinalizeConfiguration() error {
	if c.Rendering.CombinerStrategy == "" {
		c.Rendering.CombinerStrategy = "hdr"
	}

	switch c.Rendering.CombinerStrategy {
	// These three are for debugging, and figuring out alignments
	case "distinct":   c.Rendering.Combiner = MergeDistinct
	case "quad":       c.Rendering.Combiner = MergeQuadrantsLuminance
	case "bullseye":   c.Rendering.Combiner = MergeBullseye

	case "average":    c.Rendering.Combiner = MergeAverage
	case "hdr":        c.Rendering.Combiner = MergeHDR
	case "bestexposed":c.Rendering.Combiner = MergeBestExposed
	default:
		return fmt.Errorf("no CombinationStrategy named'%s'", c.Rendering.CombinerStrategy)
	}
	
	return nil
}
