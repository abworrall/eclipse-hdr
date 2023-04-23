package estack

import(
	"fmt"
	"log"
	"math"
)

// Implement Fattal '02, "Gradient Domain High Dynamic Range Compression"
// This is a straightforward port of the C++ implementation in the PFSTMO package

type Fattal02 struct {
	// Algo parameters
	DetailLevel int
	Noise       float64
	Alpha       float64
	Beta        float64
	Gamma       float64
	BlackPoint  float64
	WhitePoint  float64
	Saturation  float64

	// Data structures; they all operate solely on a single channel, relating to luminance.
	// They are calculated in this order.
	logLuminance   FloatGrid  // H,        luminance in log space: log(lum)
	pyramid      []FloatGrid  //           the Gaussian pyramid of H
	gradients    []FloatGrid  //           the gradients corresponding to each layer in the pyramid
	avgGrad      []float64    //           (and the average gradient for each layer)
	attenuation    FloatGrid  // PHI (FI), the 2D gradient attentuation function
	divG           FloatGrid  // DivG
	u              FloatGrid  // U,        the solution to the PDE: laplace(U) = DivG
	output         FloatGrid  // L,        the exponentiated luminance, after all the gradient attenutation magic
}

// Implements the fattal02 algorithm for HDR tone mapping. This is a global algo, needs access
// to whole HDR image.

func (f02 *Fattal02)Run(s *Stack) {
	f02.CreateLogLuminanceGrid(s, s.Pixels)
	f02.CreateGuassianPyramid()
	f02.CalculateGradients()
  f02.CalculateAttenuationMatrix()
	f02.CalculateDivergence()
	f02.u = solve_pde_fft(f02.divG, false)
	f02.CreateExponentiatedLuminance()
	f02.AdjustLuminances(s.Pixels)
}

// Now we have the final adjusted luminance values in `f02.output`, complete
// the tonemapping of the input image by adjust the original RGB
// values with the new luminance
func (f02 *Fattal02)AdjustLuminances(pixels [][]*PixelWorkspace) {
	width := f02.Width()
	height := f02.Height()

	MaxOf2 := func(a, b float64) float64 {
		if a > b { return a }
		return b
	}
	
	// C_out = (C_in / L_before)^s * L_after  (C are colours, L are luminances, s is magic number)
	for y:=0; y<height; y++ {
		for x:=0; x<width; x++ {			
			epsilon  := 1.0 * 1e-4
			C_in     := pixels[x][y].FusedRGB
			L_before := MaxOf2( pixels[x][y].FusedXYZ[1], epsilon )
			L_after  := MaxOf2( f02.output.Get(x,y), epsilon )
			C_after  := MyVec3{
				math.Pow( MaxOf2((C_in.F64[0] / L_before), 0.0), f02.Saturation ) * L_after,
				math.Pow( MaxOf2((C_in.F64[1] / L_before), 0.0), f02.Saturation ) * L_after,
				math.Pow( MaxOf2((C_in.F64[2] / L_before), 0.0), f02.Saturation ) * L_after,
			}
			
			pixels[x][y].TonemappedRGB.F64 = [3]float64(C_after)
		}
	}
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

	L.ToImg("007-exponentiated", "007-exponentiated.png")
	f02.output = L
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

	Gx.ToImg("005-divGx", "005-divGx.png")
	Gy.ToImg("005-divGy", "005-divGy.png")

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

	divG.ToImg("005-divG", "005-divG.png")
	f02.divG = divG
}

func (f02 *Fattal02)CalculateAttenuationMatrix() {
	nLevels := f02.NumLevels()
	phi := make([]FloatGrid, nLevels)

	noise := f02.Noise

	// Initialize topmost layer in phi
	phi[nLevels-1] = f02.gradients[nLevels-1].NewFromThis()
	for k:=0; k<len(phi[nLevels-1].values); k++ {
		phi[nLevels-1].values[k] = 1.0
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

		phi[k].ToImg("", fmt.Sprintf("004-attenuation%02d.png", k))
	}

	f02.attenuation = phi[0]
}

func (f02 *Fattal02)CalculateGradients() {
	f02.gradients = make([]FloatGrid, f02.NumLevels())
	f02.avgGrad =   make([]float64,   f02.NumLevels())
	
	for k:=0; k<f02.NumLevels(); k++ {
    f02.gradients[k], f02.avgGrad[k] = f02.pyramid[k].CalculateGradients(k)
		f02.gradients[k].ToImg("", fmt.Sprintf("003-gradient%02d.png", k))
	}
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

	pyramid := make ([]FloatGrid, nLevels)	
	pyramid[0] = *(f02.logLuminance.Copy())
	pyramid[0].ToImg("", "002-pyramid00.png")
	
	for k := 1;  k < nLevels; k++ {
		lowerLayerBlurred := pyramid[k-1].GaussianBlur()
		pyramid[k] = lowerLayerBlurred.DownSample()		
		pyramid[k].ToImg("", fmt.Sprintf("002-pyramid%02d.png", k))
	}

	f02.pyramid = pyramid
}

func (f02 *Fattal02)CreateLogLuminanceGrid(s *Stack, pixels [][]*PixelWorkspace) {
	maxLum, minLum := -100.0, 100.0
	for x := 0; x < len(pixels); x++ {
		for y := 0; y < len(pixels[x]); y++ {
			lum := pixels[x][y].FusedXYZ[1]
			if lum < minLum { minLum = lum }
			if lum > maxLum { maxLum = lum }
		}
	}

	log.Printf("Creating log(Lum) grid - global illuminance range: [%f,%f]*%.0f \n",
		minLum, maxLum, pixels[0][0].FusedRGB.IlluminanceAtMax)

	width, height := s.OutputArea.Dx(), s.OutputArea.Dy()
	lumGrid    := NewFloatGrid(width, height)
	H          := NewFloatGrid(width, height)

	for x := 0; x < len(pixels); x++ {
		for y := 0; y < len(pixels[x]); y++ {
			lum := pixels[x][y].FusedXYZ[1]
			logLum := math.Log( 100.0 * (lum-minLum)/(maxLum-minLum) + 0.0001 ) // black values = log(0+0.0001) = -9.2
			//if logLum < 0 { logLum = 1 }

			lumGrid.Set(x, y, lum)
			H.Set(x, y, logLum)
		}
	}	

	lumGrid.ToImg("001-luminance", "001-luminance.png")
	H.ToImg("001-log(luminance)", "001-logLuminance.png")
	f02.logLuminance = H
}

func (f02 *Fattal02)Width()     int { return f02.logLuminance.Dx() }
func (f02 *Fattal02)Height()    int { return f02.logLuminance.Dy() }
func (f02 *Fattal02)NumLevels() int { return len(f02.pyramid) }

