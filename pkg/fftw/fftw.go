package fftw

// Wraps the fftw3 library for use in Golang, specifically as needed
// to solve a particular PDE as per the fattal02 implementation in
// PFSTMO.
//
// There are some golang bindings for FFTW3 around, but none that exposed
// the function fftw_plan_r2r_2d() that fattal02 was using. But cgo is awesome
// and easy so we just wrapped the FFTW3 lib directly.
//
// In your OS, install a C buildchain and the FFTW3 dev library:
//  $ sudo apt-get install build-essential
//  $ sudo apt-get install libfftw3-dev
//
// If you run into precision issues, because your underlying C
// platform doesn't think a C++ 'double' is the same as a Golang
// float64, read https://www.fftw.org/fftw3_doc/Precision.html and
// make changes to the library namein LDFLAGS, and all the `fftw_`
// prefixes to C types and functions in this file.

// #cgo LDFLAGS: -lm -lfftw3
// #include <fftw3.h>
import "C"

import(
	// "log"
	"math"
	"unsafe"

	"github.com/abworrall/eclipse-hdr/pkg/emath"
)

type FftwPlan struct {
	fftw_p C.fftw_plan // Creation & destruction of this not thread safe, would need a mutex
}

func (p *FftwPlan) Execute() *FftwPlan {
	C.fftw_execute(p.fftw_p)
	return p
}

func (p *FftwPlan) Destroy() {
	C.fftw_destroy_plan(p.fftw_p)
}

func NewFftwPlan(in, out emath.FloatGrid) *FftwPlan {
	var (
		n0_  = C.int(in.Dy()) // callsites in pde_fft.cpp pass height as n0
		n1_  = C.int(in.Dx())
		in_  = (*C.double)(unsafe.Pointer(in.Ptr2array()))
		out_ = (*C.double)(unsafe.Pointer(out.Ptr2array()))
	)
  p := C.fftw_plan_r2r_2d(n0_, n1_, in_, out_, C.FFTW_REDFT00, C.FFTW_REDFT00, C.FFTW_ESTIMATE);

	return &FftwPlan{p}
}


//////// Clones of routines in pde_fft.cpp, from the PFSTMO package

// returns T = EVy A EVx^tr
// note, modifies input data
func transform_ev2normal(A emath.FloatGrid) emath.FloatGrid {
	width  := A.Dx()
	height := A.Dy()
	T      := A.NewFromThis()
	
  // the discrete cosine transform is not exactly the transform needed
  // need to scale input values to get the right transformation
  for y:=1 ; y<height-1 ; y++ {
    for x:=1 ; x<width-1 ; x++ {
			A.Set(x,y,      A.Get(x,y)        * 0.25)
		}
	}
  for x:=1 ; x<width-1 ; x++ {
		A.Set(x,0,        A.Get(x,0)        * 0.5)
		A.Set(x,height-1, A.Get(x,height-1) * 0.5)
  }
  for y:=1 ; y<height-1 ; y++ {
		A.Set(0,y,        A.Get(0,y)        * 0.5)
		A.Set(width-1,y , A.Get(width-1,y)  * 0.5)
  }

  // executes 2d discrete cosine transform
	p := NewFftwPlan(A, T)
	p.Execute()
	p.Destroy()

	return T
}

// returns T = EVy^-1 * A * (EVx^-1)^tr
func transform_normal2ev(A emath.FloatGrid) emath.FloatGrid {
	width  := A.Dx()
	height := A.Dy()
	T      := A.NewFromThis()

  // executes 2d discrete cosine transform
	p := NewFftwPlan(A, T)
	p.Execute()
	p.Destroy()

  // need to scale the output matrix to get the right transform
  for y:=0 ; y<height ; y++ {
		for x:=0 ; x<width ; x++ {
			T.Set(x,y,       T.Get(x,y)        * (1.0/float64((height-1)*(width-1))))
		}
	}
  for x:=0 ; x<width ; x++ {
		T.Set(x,0,         T.Get(x,0)        * 0.5)
		T.Set(x,height-1,  T.Get(x,height-1) * 0.5)
  }
  for y:=0 ; y<height ; y++ {
		T.Set(0,y,         T.Get(0,y)        * 0.5)
		T.Set(width-1,y,   T.Get(width-1,y)  * 0.5)
	}

	return T
}

// returns the eigenvalues of the 1d laplace operator
//
func get_lambda(n int) []float64 {
	v := make([]float64, n)
  for i:=0; i<n; i++ {
		u := math.Sin( float64(i)/float64(2*(n-1)) * math.Pi )
		v[i] = -4.0 * u * u
	}
	return v
}

// makes boundary conditions compatible so that a solution exists
func make_compatible_boundary(F emath.FloatGrid) {
	width  := F.Dx()
	height := F.Dy()

	sum := 0.0
  for y:=1 ; y<height-1 ; y++ {
    for x:=1 ; x<width-1 ; x++ {
      sum += F.Get(x,y)
		}
	}
  for x:=1 ; x<width-1 ; x++ {
    sum += 0.5 * (F.Get(x,0) + F.Get(x,height-1))
	}
  for y:=1 ; y<height-1 ; y++ {
    sum += 0.5 * (F.Get(0,y) + F.Get(width-1,y))
	}
  sum += 0.25*(F.Get(0,0) + F.Get(0,height-1) + F.Get(width-1,0) + F.Get(width-1,height-1))

	add := -1.0 * sum / float64(height+width-3)

	// log.Printf("FFT boundary - is %16f, need 0.0 to be solvable; adding %16f\n", sum, add)

  for x:=0 ; x<width ; x++ {
		F.Set(x,0,         F.Get(x,0)        + add)
		F.Set(x,height-1,  F.Get(x,height-1) + add)
  }
  for y:=1 ; y<height-1 ; y++ {
		F.Set(0,y,         F.Get(0,y)        + add)
		F.Set(width-1,y,   F.Get(width-1,y)  + add)
  }
}

// solves Laplace U = F with Neumann boundary conditions
// if adjust_bound is true then boundary values in F are modified so that
// the equation has a solution, if adjust_bound is set to false then F is
// not modified and the equation might not have a solution but an
// approximate solution with a minimum error is then calculated.
// note, input data F might be modified
func SolvePdeFft(F emath.FloatGrid, adjustBound bool) emath.FloatGrid {
  // log.Printf("solve_pde_fft: solving Laplace U = F (where F will be DivG) ...\n")
	
	width  := F.Dx()
	height := F.Dy()
	
  // activate parallel execution of fft routines
  //C.fftw_init_threads()
  //C.fftw_plan_with_nthreads(10)

  // in general there might not be a solution to the Poisson pde
  // with Neumann boundary conditions unless the boundary satisfies
  // an integral condition, this function modifies the boundary so that
  // the condition is exactly satisfied
  if adjustBound {
    // log.Printf("solve_pde_fft: checking boundary conditions\n")
    make_compatible_boundary(F)
  }

  // transforms F into eigenvector space: Ftr = 
  // log.Printf("solve_pde_fft: transform F to ev space (fft)\n")
	F_tr := transform_normal2ev(F)

  // log.Printf("solve_pde_fft: F_tr(0,0) = %f (must be zero for solution to exist)\n", F_tr.Get(0,0))

  // in the eigenvector space the solution is very simple
  // log.Printf("solve_pde_fft: solve in eigenvector space\n")
	U_tr := F_tr.NewFromThis()
	l1 := get_lambda(height)
	l2 := get_lambda(width)
  for y:=0 ; y<height ; y++ {
    for x:=0 ; x<width ; x++ {
      if x==0 && y==0 {
				U_tr.Set(x,y,  0.0) // any value ok, only adds a const to the solution
			} else {
				U_tr.Set(x,y,  F_tr.Get(x,y) / (l1[y] + l2[x]))
			}
    }
	}

  // transforms U_tr back to the normal space
  // log.Printf("solve_pde_fft: transform U_tr to normal space (fft)\n")
  U := transform_ev2normal(U_tr)

  // the solution U as calculated will satisfy something like int U = 0
  // since for any constant c, U-c is also a solution and we are mainly
  // working in the logspace of (0,1) data we prefer to have
  // a solution which has no positive values: U_new(x,y)=U(x,y)-max
  // (not really needed but good for numerics as we later take exp(U))
	max := 0.0
  for y:=0 ; y<height ; y++ {
    for x:=0 ; x<width ; x++ {
			if val := U.Get(x,y); val > max {
				max = val
			}
		}
  }
  // log.Printf("solve_pde_fft: removing constant (%f) from solution\n", max)
  for y:=0 ; y<height ; y++ {
    for x:=0 ; x<width ; x++ {
			val := U.Get(x, y)
			val -= max
			U.Set(x, y, val)
		}
	}

  // log.Printf("solve_pde_fft: done\n")

	return U
}
