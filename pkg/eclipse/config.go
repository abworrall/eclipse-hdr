package eclipse

import(
	"image"
	"log"
	"gopkg.in/yaml.v2"

	"github.com/abworrall/eclipse-hdr/pkg/emath"
)

type Config struct {
	Verbosity                   int
	
	AsShotNeutral               emath.Vec3
	ForwardMatrix               emath.Mat3

	DoEclipseAlignment          bool
	DoFineTunedAlignment        bool
	Alignments                  map[string]AlignmentTransform

	OutputWidthInSolarDiameters float64
	
	Fuser                       string
	Developer                   string
	Tonemapper                  string
	FuserLuminance              float64  // a var used by the fuser
	
	// Cloned from the FusedImage, to be more available. FIXME.
	InputArea                   image.Rectangle
	OutputArea                  image.Rectangle
}

func newConfigFromYaml(b []byte) (Config, error) {
	c := NewConfig()
	err := yaml.Unmarshal(b, &c)
	return c, err
}

func (c Config)AsYaml() string {
	b, err := yaml.Marshal(c)
	if err != nil {
		log.Fatal("Can't marshal config yaml: %v\n", err)
	}
	return string(b)
}

func NewConfig() Config {
	return Config{
		// These values are found in the output of `dng_validate.exe -v`.
		// You want to pick the ForwardMatrix that corresponds to the D65
		// illuminant. These default values will be overriden by the
		// config.yaml
		AsShotNeutral: emath.Vec3{0.5010, 1.0000, 0.7014},
		ForwardMatrix: emath.Mat3{
			0.6227,   0.3389,   0.0026,
			0.2548,   0.9378,  -0.1926,
			0.0156,  -0.1330,   0.9425,
		},
		Alignments: map[string]AlignmentTransform{},
	}
}

func (c Config)GetFuser() PixelFunc {
	switch c.Fuser {
	case "mostexposed": return FuseByPickMostExposed
	case "sector":      return FuseBySector
	case "avg":         return FuseByAverage
	default:
		log.Fatalf("no Fuser strategy named '%s'", c.Fuser)
		return nil
	}
}

func (c Config)GetDeveloper() PixelFunc {
	switch c.Developer {
	case "layer": return DevelopByLayer
	case "dng":   return DevelopByDNG
	case "wb":    return DevelopByWhiteBalanceOnly
	case "":      return DevelopByNone
	default:
		log.Fatalf("no Developer strategy named '%s'", c.Developer)
		return nil
	}
}
