package eclipse

import(
	"image"
	"log"
	"gopkg.in/yaml.v2"

	"github.com/abworrall/eclipse-hdr/pkg/emath"
)

type Config struct {
	Verbosity                   int
	
	ManualOverrideAsShotNeutral emath.Vec3   // A white/neutral color in camera native RGB space
	ManualOverrideForwardMatrix emath.Mat3   // Maps white-balanced camera native RGB into XYZ(D50).

	DoEclipseAlignment          bool
	DoFineTunedAlignment        bool
	OutputWidthInSolarDiameters float64

	Fuser                       string
	Developer                   string
	Tonemapper                  string
	FuserLuminance              float64  // a var used by the fuser

	Alignments                  map[string]AlignmentTransform

	// Values we figure out elsewhere, and put here for access by rest of app
	CameraWhite                 emath.Vec3       // From a DNG file Layer{}, or overrides
	CameraToPCS                 emath.Mat3       // From a DNG file Layer{}, or overrides
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
