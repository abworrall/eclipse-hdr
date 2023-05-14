package eclipse

import(
	"fmt"
	"image"
	"log"
	"math"
	"strings"
	"sync"

	"golang.org/x/image/draw"      // replace by "image/draw" at some point
	"golang.org/x/image/math/f64"  // replace by "image/math/f64" at some point

	"github.com/abworrall/eclipse-hdr/pkg/emath"
)

// An AlignmentTransform maps a pixel location in a later layer to a
// pixel location in the base layer, that corresponds to the same point
// in the sky.
//
// If you use an equatorial mount, this is all redundant.
type AlignmentTransform struct {
	Name            string

	TranslateByX    float64
	TranslateByY    float64
	RotationCenterX float64
	RotationCenterY float64
	RotateByDeg     float64

	ErrorMetric     float64
}

func (xform AlignmentTransform)String() string {
	str := fmt.Sprintf("Align[%s (%6.2f,%6.2f)", xform.Name, xform.TranslateByX, xform.TranslateByY)
	if xform.RotateByDeg != 0.0 {
		str += fmt.Sprintf(", %5.2fdeg", xform.RotateByDeg)
	}
	if xform.ErrorMetric != 0.0 {
		str += fmt.Sprintf(", err:%6.0f", xform.ErrorMetric)
	}
	return str + "]"
}

func (xform AlignmentTransform)XFormImage(src image.Image) image.Image {
	dst := image.NewRGBA(src.Bounds())
	draw.CatmullRom.Transform(dst, f64.Aff3(xform.ToMatrix()), src, src.Bounds(), draw.Src, nil)
	return dst
}

func (at AlignmentTransform)ToMatrix() emath.Aff3 {
	// Step 1: translate so lunar limb centers are coincident
	m := emath.Identity().Translate(at.TranslateByX, at.TranslateByY)

	// [TBD] Step 2: scale (about lunar center) so that lunar radius is the same

	// Step 3: rotate (about lunar center) so that coronas match
	if at.RotateByDeg != 0 {
		mR := emath.RotateAbout(at.RotateByDeg, at.RotationCenterX, at.RotationCenterY)
		m = mR.Mult(m)
	}

	return m
}

// AlignLayer figures out the transform that aligns `l2` to `l1`. it
// then uses it to generate l2.Image, which will be pixel-aligned
// with l1.Image.
func AlignLayer(cfg Config, l1, l2 *Layer) {
	// To get us in the ballpark, just map the center of the lunar
	// limbs. This works better than you'd think, given that the lunar
	// limb is itself moving relative to the sun (it's only there for
	// the duration of totality !)
	cent1 := l1.LunarLimb.Center()
	cent2 := l2.LunarLimb.Center()

	// Translate s2's lunar limb so that its center lines up with s1's lunar limb center
	xform := AlignmentTransform{
		Name: strings.ReplaceAll(fmt.Sprintf("%s-%s", l1.Filename(), l2.Filename()), ".tif", ""),
		RotationCenterX: float64(cent1.X), // this rotationcenter is a bit approximate
		RotationCenterY: float64(cent1.Y),
		TranslateByX: float64(cent1.X-cent2.X),
		TranslateByY: float64(cent1.Y-cent2.Y),
	}

	if cfg.DoFineTunedAlignment {
		xform = AlignLayerFine(cfg, l1, l2, xform)
		cfg.Alignments[xform.Name] = xform

	} else if xf, exists := cfg.Alignments[xform.Name]; exists {
		log.Printf("Using fine alignment from config file: %s\n", xf)
		xform = xf
	}

	l2.AlignmentTransform = xform
	l2.Image = xform.XFormImage(l2.LoadedImage)
}

// AlignLayerFine tries a wide range of possible finetune xforms in
// parallel, to find out which one fits best (i.e. has lowest error
// metric).
func AlignLayerFine(cfg Config, l1, l2 *Layer, baseXform AlignmentTransform) AlignmentTransform {
	// The difference in radii found in the images; we start off by
	// exploring x2 this amount. We can't need more than that, as the
	// lunarlimbs need to line up.
	radDelta := math.Abs(float64(l1.LunarLimb.Radius()) - float64(l2.LunarLimb.Radius()))
	if radDelta < 2.0 { radDelta = 2.0 }
	
	// Step 0. We start with a translation that superimposes the centre of the lunarlimbs.
	best := baseXform
	width, step := 0.0, 0.0
	xforms := []AlignmentTransform{}

	log.Printf("Align finetune:\n")
	log.Printf(" -- orig  : %s\n", baseXform)

	// Step 1. Try various whole-pixel translations. We pick an area to
	// look in that's based on the difference in lunar radii in the
	// images; more or less, these need to line up, so we don't need to
	// explore any further.
	width = radDelta
	step = 1.0
	xforms = xforms[:0]
	for x:=-1*width; x<=width; x += step {
		for y:=-1*width; y<=width; y += step {
			xform := best
			xform.TranslateByX += x
			xform.TranslateByY += y
			xforms = append(xforms, xform)
		}
	}
	best = scoreXFormsConcurrently(cfg, l1, l2, xforms, "pass1a")

	// Step 2. In a much smaller area, explore fractional pixel
	// translations. This relies on Catmull Rom interpolation.
	width = 2.0
	step = 0.10
	xforms = xforms[:0]
	for x:=-1*width; x<=width; x += step {
		for y:=-1*width; y<=width; y += step {
			xform := best
			xform.TranslateByX += x
			xform.TranslateByY += y
			xforms = append(xforms, xform)
		}
	}
	best = scoreXFormsConcurrently(cfg, l1, l2, xforms, "pass1b")

	// Step 3. Now we think we have the images centred on each other,
	// try some coarse rotations. (This will only be useful if the
	// images were separated by quite a lot of time)
	rotWidth := 10.0
	rotStep  := 1.0
	xforms = xforms[:0]
	for theta := -1.0*(rotWidth/2.0); theta < rotWidth/2.0; theta += rotStep {
		xform := best
		// Note - the rotation center is not really well defined here :/
		xform.RotateByDeg = theta
		xforms = append(xforms, xform)
	}
	best = scoreXFormsConcurrently(cfg, l1, l2, xforms, "pass2a")

	// Step 4. Try a smaller amount of fine-grained rotations.
	rotWidth = 2.0 // should be 10
	rotStep  = 0.05
	xforms = xforms[:0]
	for theta := -1.0*(rotWidth/2.0); theta < rotWidth/2.0; theta += rotStep {
		xform := best
		// Note - the rotation center is not really well defined here
		xform.RotateByDeg += theta
		xforms = append(xforms, xform)
	}
	best = scoreXFormsConcurrently(cfg, l1, l2, xforms, "pass2b")

	log.Printf("Align finetune: orig  %s\n", baseXform)
	log.Printf("Align finetune: final %s\n", best)
	return best
}


type fineTuneJob struct {
	// Inputs for the job
	C           Config
	L1         *Layer
	L2         *Layer
	Name        string
	XForm       AlignmentTransform

	// Output
	ErrorMetric float64
}

// ScoreXFormsConcurrently uses a pool of goroutines to compute the
// error metrics for each of the proposed transform, and return the
// one with the lowest error.
func scoreXFormsConcurrently(cfg Config, l1, l2 *Layer, xforms []AlignmentTransform, name string) AlignmentTransform {
	var wg sync.WaitGroup
	jobsChan    := make(chan fineTuneJob, len(xforms))
	resultsChan := make(chan fineTuneJob, len(xforms))

	// Kick off worker pool
	nWorkers := 20
	for i:=0; i<nWorkers; i++ {
		wg.Add(1)

		go func() {
			for job := range jobsChan {
				job.ErrorMetric = ImgDiff(job.C, job.L1, job.L2, job.Name, job.XForm)
				resultsChan<- job
				// log.Printf(" >> finetune [%s], xform %s, err: %6.0f\n", job.Name, job.XForm, job.ErrorMetric)
			}
			defer wg.Done()
		}()
	}
	
	// Feed in jobs
	for i, xform := range xforms {
		job := fineTuneJob{cfg, l1, l2, fmt.Sprintf("%s-%03d", name, i), xform, 0.0}
		jobsChan<- job
	}

	close(jobsChan)
	wg.Wait()
	close(resultsChan)

	// results processor
	bestResult := fineTuneJob{ErrorMetric: math.MaxFloat64}
	for result := range resultsChan {
		if result.ErrorMetric < bestResult.ErrorMetric {
			bestResult = result
		}
	}

	xform := bestResult.XForm
	xform.ErrorMetric = bestResult.ErrorMetric

	log.Printf(" -- %s: %s (%d tried)\n", name, xform, len(xforms))

	return xform
}
