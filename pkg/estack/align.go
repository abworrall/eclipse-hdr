package estack

import(
	"fmt"
	"image"
	"log"
	"math"
	"strings"
	"sync"

	"golang.org/x/image/draw"      // replace by "image/draw" at some point
	"golang.org/x/image/math/f64"  // replace by "image/math/f64" at some point
)

// An AlignmentTransform maps a pixel location from an image to a
// pixel location in the base image, that corresponds to the same point
// in the sky.
//
// If you use an equatorial mount, then this is all redundant.
// Otherwise, your photos need aligning because the sun is always
// moving across the sky.
//
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

func (at AlignmentTransform)ToMatrix() MyAff3 {
	// Step 1: translate so lunar limb centers are coincident
	mat := MatIdentity().MatTranslate(at.TranslateByX, at.TranslateByY)
	
	// [TBD] Step 2: scale (about lunar center) so that lunar radius is the same

	// Step 3: rotate (about lunar center) so that coronas match
	if at.RotateByDeg != 0 {
		matR := MatRotateAbout(at.RotateByDeg, at.RotationCenterX, at.RotationCenterY)
		mat = matR.MatMult(mat)
	}

	return mat
}

// Kind of a no-op
func AlignBaseImage(s *Stack, s1 *StackedImage) {
	// Base image gets a null transform, so just make shallow copy of image
	s1.XImage = s1.OrigImage
}

// Aligns s2 to s1, so that s2.XImage corresponds to s1.Ximage
func AlignStackedImage(s *Stack, s1, s2 *StackedImage) {
	// To get us in the ballpark, just map the center of the lunar
	// limbs. This works better than you'd think, given that the lunar
	// limb is itself moving relative to the sun (it's only there for
	// the duration of totality !)
	cent1 := s1.LunarLimb.Center()
	cent2 := s2.LunarLimb.Center()

	// Translate s2's lunar limb so that its center lines up with s1's lunar limb center
	xform := AlignmentTransform{
		Name: strings.ReplaceAll(fmt.Sprintf("%s-%s", s1.Filename(), s2.Filename()), ".tif", ""),
		RotationCenterX: float64(cent1.X),
		RotationCenterY: float64(cent1.Y),
		TranslateByX: float64(cent1.X-cent2.X),
		TranslateByY: float64(cent1.Y-cent2.Y),
	}

	if s.Configuration.Rendering.AlignmentFineTune {
		xform = FineTuneAlignStackedImage(s, s1, s2, xform)
		s.Configuration.Alignments[xform.Name] = xform

	} else if xf, exists := s.Configuration.Alignments[xform.Name]; exists {
		log.Printf("Used precomputed alignment: %s: %s\n", xform.Name, xf)
		xform = xf
	}

	s2.AlignmentTransform = xform
	s2.XImage = xform.XFormImage(s2.OrigImage)
}

// FineTuneAlignStackedImage tries a wide range of possible finetune
// xforms in parallel, to find out which one fits best (i.e. has
// lowest error metric)
func FineTuneAlignStackedImage(s *Stack, s1, s2 *StackedImage, baseXform AlignmentTransform) AlignmentTransform {
	// The difference in radii; we explore x2 this amount
	radDelta := math.Abs(float64(s1.LunarLimb.Radius()) - float64(s2.LunarLimb.Radius()))
	if radDelta < 2.0 { radDelta = 2.0 }

	if false {
		xform1 := baseXform
		xform1.TranslateByX--
		xform1.TranslateByY--
		ImgDiff(s, s1, s2, "fake-6666", xform1)
		log.Fatal("done here")
	}
	
	best := baseXform
	width, step := 0.0, 0.0
	xforms := []AlignmentTransform{}

	log.Printf("Align finetune:\n")
	log.Printf(" -- orig  : %s\n", baseXform)

	// Start with some coarse translations around solar origin
	if true {
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
		best = ScoreXFormsConcurrently(s, s1, s2, xforms, "pass1a")
	}

	if true {
		// Now super-finetune the translation
		width = 2.0
		step = 0.25
		xforms = xforms[:0]
		for x:=-1*width; x<=width; x += step {
			for y:=-1*width; y<=width; y += step {
				xform := best
				xform.TranslateByX += x
				xform.TranslateByY += y
				xforms = append(xforms, xform)
			}
		}
		best = ScoreXFormsConcurrently(s, s1, s2, xforms, "pass1b")
	}

	// Now, try some rotations (all values in degrees)
	if true {
		rotWidth := 10.0 // should be 10
		rotStep  := 1.0
		xforms = xforms[:0]
		for theta := -1.0*(rotWidth/2.0); theta < rotWidth/2.0; theta += rotStep {
			xform := best
			// Note - the rotation center is not really well defined here :/
			xform.RotateByDeg = theta
			xforms = append(xforms, xform)
		}
		best = ScoreXFormsConcurrently(s, s1, s2, xforms, "pass2a")

		// Now super-finetune
		if true {
			rotWidth = 2.0 // should be 10
			rotStep  = 0.05
			xforms = xforms[:0]
			for theta := -1.0*(rotWidth/2.0); theta < rotWidth/2.0; theta += rotStep {
				xform := best
				// Note - the rotation center is not really well defined here
				xform.RotateByDeg += theta
				xforms = append(xforms, xform)
			}
			best = ScoreXFormsConcurrently(s, s1, s2, xforms, "pass2b")
		}

		if false {
			// If we have a rotation correction, try a further translation
			if best.RotateByDeg != 0.0 {
				width = 2.0
				step = 0.25
				xforms = xforms[:0]
				for x:=-1*width; x<=width; x += step {
					for y:=-1*width; y<=width; y += step {
						xform := best
						xform.TranslateByX += x
						xform.TranslateByY += y
						xforms = append(xforms, xform)
					}
				}
				best = ScoreXFormsConcurrently(s, s1, s2, xforms, "pass3")
			}
		}
	}

	log.Printf("Align finetune: orig  %s\n", baseXform)
	log.Printf("Align finetune: final %s\n", best)
	return best
}


type FineTuneJob struct {
	// Inputs for the job
	S          *Stack
	S1         *StackedImage
	S2         *StackedImage
	Name        string
	XForm       AlignmentTransform

	// Output
	ErrorMetric float64
}

// ScoreXFormsConcurrently uses a pool of goroutines to compute the
// error metrics for each of the proposed transform, and return the
// one with the lowest error.
func ScoreXFormsConcurrently(s *Stack, s1, s2 *StackedImage, xforms []AlignmentTransform, name string) AlignmentTransform {
	var wg sync.WaitGroup
	jobsChan    := make(chan FineTuneJob, len(xforms))
	resultsChan := make(chan FineTuneJob, len(xforms))

	// Kick off worker pool
	nWorkers := 20
	for i:=0; i<nWorkers; i++ {
		wg.Add(1)

		go func() {
			for job := range jobsChan {
				job.ErrorMetric = ImgDiff(job.S, job.S1, job.S2, job.Name, job.XForm)
				resultsChan<- job
				// log.Printf(" >> finetune [%s], xform %s, err: %6.0f\n", job.Name, job.XForm, job.ErrorMetric)
			}
			defer wg.Done()
		}()
	}
	
	// Feed in jobs
	for i, xform := range xforms {
		job := FineTuneJob{s, s1, s2, fmt.Sprintf("%s-%03d", name, i), xform, 0.0}
		jobsChan<- job
	}

	close(jobsChan)
	wg.Wait()
	close(resultsChan)

	// results processor
	bestResult := FineTuneJob{ErrorMetric: math.MaxFloat64}
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
