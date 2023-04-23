package estack

/* Example config file ...

The ForwardMatrix is a 3x3, a row at a time; the example below corresponds to:
		MyMat3{
			0.6227,   0.3389,   0.0026,
			0.2548,   0.9378,  -0.1926,
			0.0156,  -0.1330,   0.9425,
		}

asshotneutral:
- 0.501
- 1
- 0.7014
forwardmatrix:
- 0.6227
- 0.3389
- 0.0026
- 0.2548
- 0.9378
- -0.1926
- 0.0156
- -0.133
- 0.9425
rendering:
  outputfilename: out.png
  outputwidthinsolardiameters: 3
  aligneclipse: true
  alignmentfinetune: false
  applygammaexpansion: true
  fuserstrategy: mostexposed
  tonemapperstrategy: linear
  colortweakerstrategy: ""

*/

import(
	"log"
	"gopkg.in/yaml.v2"
)

type RenderOptions struct {
	OutputFilename              string
	OutputWidthInSolarDiameters float64

	AlignEclipse                bool
	AlignmentFineTune           bool

	DNGDevelop                  bool
	ApplyGammaExpansion         bool

	FuserStrategy               string
	TonemapperStrategy          string
	ColorTweakerStrategy        string
}

type Configuration struct {
	AsShotNeutral      MyVec3
	ForwardMatrix      MyMat3

	Rendering          RenderOptions

	Alignments         map[string]AlignmentTransform
}

func NewConfigurationFromYaml(b []byte) (Configuration, error) {
	c := NewConfiguration()
	err := yaml.Unmarshal(b, &c)
	return c, err
}

func (c Configuration)AsYaml() string {
	b, err := yaml.Marshal(c)
	if err != nil {
		log.Fatal("Can't marshal config yaml: %v\n", err)
	}
	return string(b)
}

func NewConfiguration() Configuration {
	return Configuration{
		// These values are found in the text output of `dng_validate.exe
		// -v`. Note I'm picking the second illuminant, D65, since its
		// direct sunlight; so we use ForwardMatrix2. I'm just hardwiring
		// them in here.
		AsShotNeutral: MyVec3{0.5010, 1.0000, 0.7014},
		ForwardMatrix: MyMat3{
			0.6227,   0.3389,   0.0026,
			0.2548,   0.9378,  -0.1926,
			0.0156,  -0.1330,   0.9425,
		},
		Alignments: map[string]AlignmentTransform{},
	}
}

func (c *Configuration)GetFuser() PixelFunc {
	switch c.Rendering.FuserStrategy {
	case "mostexposed": return FuseBySingleMostExposed
	case "sector":      return FuseBySector
	case "avg":         return FuseByAverage
	default:
		log.Fatalf("no FuserStrategy named '%s'", c.Rendering.FuserStrategy)
		return nil
	}
}

func (c *Configuration)GetTonemapper() GlobalTonemapper {
	switch c.Rendering.TonemapperStrategy {
	case "fattal02":  return TonemapFattal02
	case "linear":    return TonemapLinear
	default:
		log.Fatalf("no ToneMapperStrategy named '%s'", c.Rendering.TonemapperStrategy)
		return nil
	}
}

func (c *Configuration)GetColorTweaker() PixelFunc {	
	switch c.Rendering.ColorTweakerStrategy {
	case "layer": return ColorTweakByLayer
	case "":      return nil
	default:
		log.Fatalf("no ColorTweakStrategy named '%s'", c.Rendering.ColorTweakerStrategy)
		return nil
	}
}
