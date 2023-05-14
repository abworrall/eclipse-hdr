package emath

// Some basic affine transformations, used in image alignment

import(
	"fmt"
	"math"
	"golang.org/x/image/math/f64"  // Will be "image/math/f64" at some point, hopefully make this file redundant
)

// Use a local type so we can hang methods off it
type Aff3 f64.Aff3

// Cut-n-pasted from image@0.7.0/draw/scale:matMul
func (p Aff3)Mult(q Aff3) Aff3 {
	return Aff3{
		p[3*0+0]*q[3*0+0] + p[3*0+1]*q[3*1+0],
		p[3*0+0]*q[3*0+1] + p[3*0+1]*q[3*1+1],
		p[3*0+0]*q[3*0+2] + p[3*0+1]*q[3*1+2] + p[3*0+2],
		p[3*1+0]*q[3*0+0] + p[3*1+1]*q[3*1+0],
		p[3*1+0]*q[3*0+1] + p[3*1+1]*q[3*1+1],
		p[3*1+0]*q[3*0+2] + p[3*1+1]*q[3*1+2] + p[3*1+2],
	}
}

func Identity() Aff3 {
	return Aff3{1, 0, 0,   0, 1, 0}
}

func (m1 Aff3)Translate(tx, ty float64) Aff3 {
	return m1.Mult(Aff3{1, 0, tx,   0, 1, ty})
}

func (m1 Aff3)Rotate(thetaDeg float64) Aff3 {
	cosTheta := math.Cos(thetaDeg * math.Pi / 180.0)
	sinTheta := math.Sin(thetaDeg * math.Pi / 180.0)
	return m1.Mult(Aff3{cosTheta, -1*sinTheta, 0,    sinTheta, cosTheta, 0})
}

func RotateAbout(thetaDeg, x, y float64) Aff3 {
	// Remember they compose back to front - rightmost operations performed first
	return Identity().Translate(x, y).Rotate(thetaDeg).Translate(-1*x, -1*y)
}

// Actual 3x3 matrixes, used for color transforms
type Vec3 f64.Vec3
type Mat3 f64.Mat3

func (a Mat3)Mult(b Mat3) Mat3 {
	return Mat3{
		a[3*0+0]*b[3*0+0] + a[3*0+1]*b[3*1+0] + a[3*0+2]*b[3*2+0],
		a[3*0+0]*b[3*0+1] + a[3*0+1]*b[3*1+1] + a[3*0+2]*b[3*2+1],
		a[3*0+0]*b[3*0+2] + a[3*0+1]*b[3*1+2] + a[3*0+2]*b[3*2+2],

		a[3*1+0]*b[3*0+0] + a[3*1+1]*b[3*1+0] + a[3*1+2]*b[3*2+0],
		a[3*1+0]*b[3*0+1] + a[3*1+1]*b[3*1+1] + a[3*1+2]*b[3*2+1],
		a[3*1+0]*b[3*0+2] + a[3*1+1]*b[3*1+2] + a[3*1+2]*b[3*2+2],

		a[3*2+0]*b[3*0+0] + a[3*2+1]*b[3*1+0] + a[3*2+2]*b[3*2+0],
		a[3*2+0]*b[3*0+1] + a[3*2+1]*b[3*1+1] + a[3*2+2]*b[3*2+1],
		a[3*2+0]*b[3*0+2] + a[3*2+1]*b[3*1+2] + a[3*2+2]*b[3*2+2],
	}
}

func (m Mat3)Apply(v Vec3) Vec3 {
	return Vec3{
		(m[3*0+0]*v[0] + m[3*0+1]*v[1] + m[3*0+2]*v[2]),
	  (m[3*1+0]*v[0] + m[3*1+1]*v[1] + m[3*1+2]*v[2]),
	  (m[3*2+0]*v[0] + m[3*2+1]*v[1] + m[3*2+2]*v[2]),
	}
}

func (m Mat3)String() string {
	str := fmt.Sprintf("[%10f, %10f, %10f]\n", m[3*0+0], m[3*0+1], m[3*0+2])
	str += fmt.Sprintf("[%10f, %10f, %10f]\n", m[3*1+0], m[3*1+1], m[3*1+2])
	str += fmt.Sprintf("[%10f, %10f, %10f]\n", m[3*2+0], m[3*2+1], m[3*2+2])
	return str
}
func (v Vec3)String() string {
	return fmt.Sprintf("[%12.10f, %12.10f, %12.10f]", v[0], v[1], v[2])
}

// Places the vector on the diagonal of a matrix, then inverts it
func (v Vec3)InvertDiag() Mat3 {
	return Mat3{
		1.0 / v[0],           0,           0,
		0,           1.0 / v[1],           0,
		0,                    0,  1.0 / v[2],
	}
}

func (v *Vec3)FloorAt(min float64) {
	if v[0] < min { v[0] = min }
	if v[1] < min { v[1] = min }
	if v[2] < min { v[2] = min }
}

func (v *Vec3)CeilingAt(max float64) {
	if v[0] > max { v[0] = max }
	if v[1] > max { v[1] = max }
	if v[2] > max { v[2] = max }
}
