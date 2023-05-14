package emath

import "math"

// Some functions that only operate on basic types, that are useful

// https://www.sjbrown.co.uk/posts/gamma-correct-rendering/ - "linear RGB to sRGB"
// Each channel in `v` is assumed to be in the range [0,1]
func GammaExpand_sRGB(v Vec3) Vec3 {
	return Vec3{
		GammaExpand_F64(v[0]),
		GammaExpand_F64(v[1]),
		GammaExpand_F64(v[2]),
	}
}

func GammaExpand_F64(f float64) float64 {
	if f <= 0.0031308 {
		return 12.92 * f
	}
	return 1.055 * math.Pow(f, 1.0/2.4) - 0.055
}

