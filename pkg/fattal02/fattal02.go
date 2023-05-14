package fattal02

// Implement Fattal '02, "Gradient Domain High Dynamic Range Compression"

import(
	"image"
	"image/color"
	"fmt"
	"math"

	"github.com/mdouchement/hdr"
	"github.com/mdouchement/hdr/hdrcolor"

	"github.com/abworrall/eclipse-hdr/pkg/emath"
	"github.com/abworrall/eclipse-hdr/pkg/fftw"
)

// Fattal02 is a straightforward port of the C++ implementation from
// the PFSTMO package. It relies on the fftw3 library, and uses cgo to
// link to it.
type Fattal02 struct {
	// Algo parameters
	DetailLevel    int
	Noise          float64
	Alpha          float64
	Beta           float64
	Gamma          float64
	BlackPoint     float64
	WhitePoint     float64
	Saturation     float64

	// Our extra params
	GammaExpand    bool        // whether to perform sRGB gamma expansion on final output
	DumpGrids      bool        // whether to write greyscale image files for the intermediate grids
	
	Input          hdr.Image   // HDR image
	Output         image.Image // LDR image
	
	// intermediate data structures; they all operate solely on a single channel, relating to luminance.
	// They are calculated in this order.
	logLuminance   emath.FloatGrid  // H,        luminance in log space: log(lum)
	pyramid      []emath.FloatGrid  //           the Gaussian pyramid of H
	gradients    []emath.FloatGrid  //           the gradients corresponding to each layer in the pyramid
	avgGrad      []float64    //           (and the average gradient for each layer)
	attenuation    emath.FloatGrid  // PHI (FI), the 2D gradient attentuation function
	divG           emath.FloatGrid  // DivG
	u              emath.FloatGrid  // U,        the solution to the PDE: laplace(U) = DivG
	outputLum      emath.FloatGrid  // L,        the exponentiated luminance, after all the gradient attenutation magic
}

func (f02 *Fattal02)Width()     int { return f02.Input.Bounds().Dx() }
func (f02 *Fattal02)Height()    int { return f02.Input.Bounds().Dy() }
func (f02 *Fattal02)NumLevels() int { return len(f02.pyramid) }

func NewDefaultFattal02(img hdr.Image) *Fattal02 {
	return &Fattal02{
		// The PFSTMO parameters - see https://www.mankier.com/1/pfstmo_fattal02
		// These are the default values when using the FFT solver
		DetailLevel: 3,
		Noise:       0.002,
		Alpha:       1.0,
		Beta:        0.9,
		Gamma:       0.8,
		BlackPoint:  0.1,
		WhitePoint:  0.5,
		Saturation:  0.8,

		Input:       img,
	}
}

// Implement mdouchement/hdr/tmo:ToneMappingOperator
func (f02 *Fattal02)Perform() image.Image {
	f02.CreateLogLuminanceGrid()
	f02.CreateGuassianPyramid()
	f02.CalculateGradients()
  f02.CalculateAttenuationMatrix()
	f02.CalculateDivergence()

	f02.u = fftw.SolvePdeFft(f02.divG, false)
	f02.MaybeDumpGrid(f02.u, "006-solved-PDE", "006-solved-PDE.png")

	f02.CreateExponentiatedLuminance()
	f02.FillOutputImage()

	return f02.Output
}

func (f02 *Fattal02)MaybeDumpGrid(f emath.FloatGrid, comment, filename string) {
	if f02.DumpGrids {
		f.ToImg(comment, filename)
	}
}

func (f02 *Fattal02)CreateLogLuminanceGrid() {
	maxLum  := -100.0
	minLum  :=  100.0
	bounds  := f02.Input.Bounds()
	width   := bounds.Dx()
	height  := bounds.Dy()
	lumGrid := emath.NewFloatGrid(width, height)

	for x:=bounds.Min.X; x<bounds.Max.X; x++ {
		for y:=bounds.Min.Y; y<bounds.Max.Y; y++ {
			rgb := f02.Input.HDRAt(x, y)
			xyz := hdrcolor.XYZModel.Convert(rgb)
			_, lum, _, _ := xyz.(hdrcolor.Color).HDRXYZA()

			if lum < minLum { minLum = lum }
			if lum > maxLum { maxLum = lum }
			lumGrid.Set(x, y, lum)
		}
	}

	// log.Printf("Creating log(Lum) grid - global illuminance range: [%f,%f]\n", minLum, maxLum)

	H := emath.NewFloatGrid(width, height)

	for x:=0; x<width; x++ {
		for y:=0; y<height; y++ {
			lum := lumGrid.Get(x, y)
			logLum := math.Log( 100.0 * (lum-minLum)/(maxLum-minLum) + 0.0001 ) // black values = log(0+0.0001) = -9.2
			H.Set(x, y, logLum)
		}
	}	

	f02.MaybeDumpGrid(lumGrid, "001-luminance", "001-luminance.png")
	f02.MaybeDumpGrid(H, "001-log(luminance)", "001-logLuminance.png")
	f02.logLuminance = H
}

func (f02 *Fattal02)CreateGuassianPyramid() {
	width  := f02.Width()
	height := f02.Height()

	// Figure out depth
	nLevels := 0
	minDim := height
	if width < minDim { minDim = width }
	for minDim >= 8 {
		minDim /= 2
		nLevels++
	}

	pyramid := make ([]emath.FloatGrid, nLevels)	
	pyramid[0] = *(f02.logLuminance.Copy())
	f02.MaybeDumpGrid(pyramid[0], "", "002-pyramid00.png")
	
	for k := 1;  k < nLevels; k++ {
		lowerLayerBlurred := pyramid[k-1].GaussianBlur()
		pyramid[k] = lowerLayerBlurred.DownSample()		
		f02.MaybeDumpGrid(pyramid[k], "", fmt.Sprintf("002-pyramid%02d.png", k))
	}

	f02.pyramid = pyramid
}

func (f02 *Fattal02)CalculateGradients() {
	f02.gradients = make([]emath.FloatGrid, f02.NumLevels())
	f02.avgGrad =   make([]float64,   f02.NumLevels())
	
	for k:=0; k<f02.NumLevels(); k++ {
    f02.gradients[k], f02.avgGrad[k] = f02.pyramid[k].CalculateGradients(k)
		f02.MaybeDumpGrid(f02.gradients[k], "", fmt.Sprintf("003-gradient%02d.png", k))
	}
}

func (f02 *Fattal02)CalculateAttenuationMatrix() {
	nLevels := f02.NumLevels()
	phi := make([]emath.FloatGrid, nLevels)

	noise := f02.Noise

	// Initialize topmost layer in phi
	phi[nLevels-1] = f02.gradients[nLevels-1].NewFromThis()

	for x:=0; x<phi[nLevels-1].Dx(); x++ {
		for y:=0; y<phi[nLevels-1].Dy(); y++ {
			phi[nLevels-1].Set(x, y, 1.0)
		}
	}

	// Walk down the pyramid from the top layer
	for k:=nLevels-1; k>=0; k-- {
		width  := f02.gradients[k].Dx()
		height := f02.gradients[k].Dy()

		// only apply gradients to levels>=detail_level but at least to the coarsest
		if(k>=f02.DetailLevel || k==nLevels-1) {
      for y:=0; y<height; y++ {
        for x:=0; x<width; x++ {
					grad  := f02.gradients[k].Get(x,y)
					a     := f02.Alpha * f02.avgGrad[k]
					value := 1.0

          if grad > 1e-4 {
            value = a/(grad+noise) * math.Pow((grad+noise)/a, f02.Beta)
					}
					newValue := phi[k].Get(x,y) * value
					phi[k].Set(x,y, newValue)
				}
			}
		}

		// create next level down, by upsampling curr level
    if k > 0 {
			upsampled := f02.gradients[k-1].NewFromThis()
      phi[k].UpSampleInto(&upsampled)
      phi[k-1] = upsampled.GaussianBlur()
		}

		f02.MaybeDumpGrid(phi[k], "", fmt.Sprintf("004-attenuation%02d.png", k))
	}

	f02.attenuation = phi[0]
}


func (f02 *Fattal02)CalculateDivergence() {
	width  := f02.Width()
	height := f02.Height()

	H      := f02.logLuminance
	PHI    := f02.attenuation
	Gx     := f02.pyramid[0].NewFromThis()
	Gy     := f02.pyramid[0].NewFromThis()

  // the fft solver solves the Poisson pde but with slightly different
  // boundary conditions, so we need to adjust the assembly of the right hand
  // side accordingly (basically fft solver assumes U(-1) = U(1), whereas zero
  // Neumann conditions assume U(-1)=U(0)), see also divergence calculation
	for y:=0; y<height; y++ {
		for x:=0; x<width; x++ {
			// sets index+1 based on the boundary assumption H(N+1)=H(N-1)
			yp1 := y+1
			xp1 := x+1
			if y+1 >= height { yp1 = height-2 }
			if x+1 >= width  { xp1 = width -2 }

			// forward differences in H, so need to use between-points approx of PHI
			gx := (H.Get(xp1,y) - H.Get(x,y)) * 0.5*(PHI.Get(xp1,y) + PHI.Get(x,y))
			gy := (H.Get(x,yp1) - H.Get(x,y)) * 0.5*(PHI.Get(x,yp1) + PHI.Get(x,y))

			Gx.Set(x, y, gx)
			Gy.Set(x, y, gy)
		}
	}

	f02.MaybeDumpGrid(Gx, "005-divGx", "005-divGx.png")
	f02.MaybeDumpGrid(Gy, "005-divGy", "005-divGy.png")

  // calculate divergence
	divG := f02.pyramid[0].NewFromThis()
	for y:=0; y<height; y++ {
		for x:=0; x<width; x++ {
			val := Gx.Get(x,y) + Gy.Get(x,y)
			if x>0 { val -= Gx.Get(x-1, y) }
			if y>0 { val -= Gy.Get(x, y-1) }
			if x==0 { val += Gx.Get(x,y) } // for fftsolver
			if y==0 { val += Gy.Get(x,y) } // for fftsolver

			divG.Set(x, y, val)
    }
	}

	f02.MaybeDumpGrid(divG, "005-divG", "005-divG.png")
	f02.divG = divG
}

func (f02 *Fattal02)CreateExponentiatedLuminance() {
	width  := f02.Width()
	height := f02.Height()
	U      := f02.u
	L      := U.NewFromThis()

	for y:=0; y<height; y++ {
		for x:=0; x<width; x++ {
			L.Set(x,y,  math.Exp(f02.Gamma * U.Get(x,y)) - 1e-4)
		}
	}

  // remove percentile of min and max values and renormalize
  cut_min := 0.01 * f02.BlackPoint;
  cut_max := 1.0 - 0.01 * f02.WhitePoint;
	minLum, maxLum := L.FindMaxMinLumAtPercentile(cut_min, cut_max)

	for y:=0; y<height; y++ {
		for x:=0; x<width; x++ {
			val := (L.Get(x,y) - minLum) / (maxLum - minLum)
			if val <= 0.0 {
				val = 1e-4
			}
			L.Set(x,y, val)
		}
	}

	f02.MaybeDumpGrid(L, "007-exponentiated", "007-exponentiated.png")
	f02.outputLum = L
}

// Now we have the final adjusted luminance values in `f02.output`, complete
// the tonemapping of the input image by adjust the original RGB
// values with the new luminance
func (f02 *Fattal02)FillOutputImage() {
	width := f02.Width()
	height := f02.Height()

	MaxOf2 := func(a, b float64) float64 {
		if a > b { return a }
		return b
	}

	out := image.NewRGBA64(f02.Input.Bounds())

	// C_out = (C_in / L_before)^s * L_after  (C are colours, L are luminances, s is magic number)
	for y:=0; y<height; y++ {
		for x:=0; x<width; x++ {			
			epsilon  := 1.0 * 1e-4

			rgb      := f02.Input.HDRAt(x, y).(hdrcolor.RGB)
			xyz      := hdrcolor.XYZModel.Convert(rgb).(hdrcolor.XYZ)

			C_in     := rgb
			L_before := MaxOf2( xyz.Y, epsilon )
			L_after  := MaxOf2( f02.outputLum.Get(x,y), epsilon )
			C_after  := emath.Vec3{
				math.Pow( MaxOf2((C_in.R / L_before), 0.0), f02.Saturation ) * L_after,
				math.Pow( MaxOf2((C_in.G / L_before), 0.0), f02.Saturation ) * L_after,
				math.Pow( MaxOf2((C_in.B / L_before), 0.0), f02.Saturation ) * L_after,
			}

			if f02.GammaExpand {
				C_after = emath.GammaExpand_sRGB(C_after)
			}

			C_after.CeilingAt(1.0) // Clipping, else high vals wraparound

			out.Set(x, y, color.RGBA64{
				R: uint16(C_after[0] * float64(0xFFFF)),
				G: uint16(C_after[1] * float64(0xFFFF)),
				B: uint16(C_after[2] * float64(0xFFFF)),
				A: 0xFFFF,
			})
		}
	}

	f02.Output = out
}

