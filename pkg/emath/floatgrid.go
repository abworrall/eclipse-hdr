package emath

import(
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"

	"github.com/fogleman/gg" // Move to https://pkg.go.dev/golang.org/x/image/font#Drawer sometime
)

// Mostly all cloned from tmo_fattal02.cpp, part of the PFSTMO package.

// A FloatGrid is a grid of floats, with some operations
type FloatGrid struct {
	stride int
	values []float64
}

func NewFloatGrid(w, h int) FloatGrid {
	return FloatGrid{
		stride: w,
		values: make([]float64, w*h),
	}
}

func (g1 *FloatGrid)NewFromThis() FloatGrid  { return NewFloatGrid(g1.Dx(), g1.Dy()) }
func (fg *FloatGrid)Set(x, y int, v float64) { fg.values[fg.stride*y + x] = v }
func (fg *FloatGrid)Get(x, y int) float64    { return fg.values[fg.stride*y + x] }
func (fg *FloatGrid)Dx() int                 { return fg.stride }
func (fg *FloatGrid)Dy() int                 { return len(fg.values) / fg.stride }
func (fg *FloatGrid)Ptr2array() *float64     { return &fg.values[0] } // needed for fftw3 C bindings

func (g1 *FloatGrid)Copy() *FloatGrid {
	g2 := FloatGrid{stride: g1.stride, values:make([]float64, len(g1.values))}
	copy(g2.values, g1.values)
	return &g2
}

func (g1 FloatGrid)GaussianBlur() FloatGrid {
	width := g1.Dx()
	height := g1.Dy()
	g2 := g1.NewFromThis()

	T  := g1.NewFromThis()

	//--- X blur, build up in T
	for y:=0; y<height; y++ {
		for x:=1; x<width-1; x++ {
			t := 2.0*g1.Get(x,y)
			t += g1.Get(x-1,y)
			t += g1.Get(x+1,y)
			T.Set(x, y, t/4.0)
		}
		T.Set(0, y,       (3.0*g1.Get(0,      y) + g1.Get(1,      y)) / 4.0)
		T.Set(width-1, y, (3.0*g1.Get(width-1,y) + g1.Get(width-2,y)) / 4.0)
	}

  //--- Y blur, read from T and generate output
	for x:=0; x<width; x++ {
    for y:=1; y<height-1; y++ {
      t := 2.0*T.Get(x,y)
      t += T.Get(x,y-1)
      t += T.Get(x,y+1)
			g2.Set(x, y, t/4.0)
    }
		g2.Set(x, 0,        (3.0*T.Get(x,       0) + T.Get(x,       1)) / 4.0)
		g2.Set(x, height-1, (3.0*T.Get(x,height-1) + T.Get(x,height-2)) / 4.0)
  }

	return g2
}

func (H *FloatGrid)CalculateGradients(depth int) (FloatGrid, float64) {
	G := H.NewFromThis()

	width := H.Dx()
	height := H.Dy()
	divider := math.Pow(2.0, float64(depth)+1)
	avgGrad := 0.0
	
	for y:=0; y<height; y++ {
		for x:=0; x<width-1; x++ {
			w := x-1
			e := x+1
			n := y-1
			s := y+1
			if x == 0        { w = 0 }
			if x == width-1  { e = x }
			if y == 0        { n = 0 }
			if y == height-1 { s = y }

      gx := (H.Get(w,y) - H.Get(e,y)) / divider
      gy := (H.Get(x,s) - H.Get(x,n)) / divider

      // note this implicitely assumes that H(-1)=H(0)
      // for the fft-pde slover this would need adjustment as H(-1)=H(1)
      // is assumed, which means gx=0.0, gy=0.0 at the boundaries
      // however, the impact is not visible so we ignore this here
      G.Set(x,y, math.Sqrt(gx*gx+gy*gy))
      avgGrad += G.Get(x,y)
    }
  }

  return G, (avgGrad / float64(width*height))
}

// DownSample returns a grid that is 1/4 of the size, averaging the values from the
// original.
func (g1 *FloatGrid)DownSample() FloatGrid {
	width := g1.Dx() / 2
	height := g1.Dy() / 2
	g2 := NewFloatGrid(width, height)
	
	for y:=0; y<height; y++ {
		for x:=0; x<width; x++ {
			p := g1.Get(2*x,   2*y)
			p += g1.Get(2*x+1, 2*y)
			p += g1.Get(2*x,   2*y+1)
			p += g1.Get(2*x+1, 2*y+1)
			g2.Set(x, y, p/4.0)
		}
	}

	return g2
}

// UpSampleInto populates a grid `B`, which is assumed be 2x as big,
// by simply copying each value from `A` four times into a 2x2 block
// of values in `B`
func (A *FloatGrid)UpSampleInto(B *FloatGrid) {
	awidth  := A.Dx()
	aheight := A.Dy()
	width   := B.Dx()
	height  := B.Dy()

	for y:=0; y<height; y++ {
		for x:=0; x<width; x++ {
      ax := x/2
      ay := y/2
			if ax >= awidth  { ax = awidth-1 }
			if ay >= aheight { ay = aheight-1 }
			B.Set(x, y, A.Get(ax, ay))
    }
	}
}

func (I *FloatGrid)FindMaxMinLumAtPercentile(minPrct, maxPrct float64) (float64, float64) {
	vI := []float64{}

  for i:=0 ; i<len(I.values) ; i++ {
		if val := I.values[i]; val != 0.0 {
			vI = append(vI, val)
		}
	}

	sort.Float64s(vI)

	iMin := int(minPrct * float64(len(vI)))
	iMax := int(maxPrct * float64(len(vI)))
	if iMin < 0        { iMin = 0 }
	if iMax >= len(vI) { iMax = len(vI)-1 }
	
  return vI[iMin], vI[iMax]
}

func (fg *FloatGrid)Stats() string {
	min := math.MaxFloat64
	max := -1.0  * min

	for i:=0 ; i<len(fg.values) ; i++ {
		if fg.values[i] > max { max = fg.values[i] }
		if fg.values[i] < min { min = fg.values[i] }
	}
	return fmt.Sprintf("fg[%dx%d, vals{%f,%f}]", fg.Dx(), fg.Dy(), min, max)
}

// ToImg saves a simple grayscale, based on the range of values in the grid, and gamma scaling the
// gray to look normal for human vision
func (fg *FloatGrid)ToImg(title, filename string) {
	min, max := 1000.0, -1000.0
	for i:=0; i<len(fg.values); i++ {
		if fg.values[i] > max { max = fg.values[i] }
		if fg.values[i] < min { min = fg.values[i] }
	}

	img := image.NewRGBA64(image.Rectangle{Max:image.Point{fg.Dx(), fg.Dy()}})
	for x:=0; x<fg.Dx(); x++ {
		for y:=0; y<fg.Dy(); y++ {
			lum := fg.Get(x,y)
			gray := GammaExpand_F64 ((lum - min) / (max - min))
			col := color.RGBA64{uint16(gray * 65535.0), uint16(gray * 65535.0), uint16(gray * 65535.0), 0xFFFF}
			img.Set(x, y, col)
		}
	}

	dc := gg.NewContextForImage(img)
	dc.SetRGB(1,1,1)
	dc.DrawString(title, 50, 50)
	dc.SavePNG(filename)
}
