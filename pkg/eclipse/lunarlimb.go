package eclipse

import(
	"image"
	"image/color"
	"log"
	"math"
)

// The LunarLimb is the shadow/outline of the moon. We identify it and
// use it as a starting point for aligning images.
type LunarLimb struct {
	LuminalCenter image.Point // The luminance-weighted "center" of the image. Hopefully will be inside the limb.
	Brightness uint16         // A rough average of the brightness of the pixels in the limb (floodfill needs to know this)
	Bounds image.Rectangle    // A box around the limb
}

func (ll LunarLimb)Radius() int { return (ll.Bounds.Dx() + ll.Bounds.Dy())/4 }
func (ll LunarLimb)Center() image.Point { return RectCenter(ll.Bounds) }

func (ll *LunarLimb)Grow(p image.Point) {
	if ll.Bounds.Max.X == 0 {
		ll.Bounds.Min = p
		ll.Bounds.Max = p
	} else {
		ll.Bounds =	GrowRectangle(ll.Bounds, p)
	}
}

// FindLunarLimb returns a Rectangle that bounds the lunar limb, the
// outline of the moon. This is a fairly dumb routine; it finds the
// centroid of all the luminance in the image, assumes that is inside
// the lunar limb, and then floodfills out until it sees some
// bright pixels.
func FindLunarLimb(cfg Config, img image.Image) LunarLimb {
	ll := LunarLimb{}
	p := image.Point{}
	bounds := img.Bounds()

	ll.computeLuminalCenter(img)
	dci.StartNewFrame(bounds, ll.LuminalCenter)
	
	// Any pixel that is brighter than thresh is considered part of the
	// corona etc., i.e. outside the limb. We set this kinda high,
	// because some shots can have quite a lot of earthshine (luminance
	// inside the limb). But if the overall photo looks kinda dim,
	// reduce the thresh, else the corona will be so dim that the flood
	// will flow over it and cover the whole image.
	thresh := uint16(0x1000)
	if ll.Brightness < 0x0015 {
		thresh = uint16(0x0040)
	}

	seenMap := map[image.Point]bool{}
	seen := func(p image.Point) bool {
		_, exists := seenMap[p]
		return exists
	}
	
	// Floodfill out from the LuminalCenter
	toVisit := []image.Point{ll.LuminalCenter}
	for {
		if len(toVisit) == 0 { break }
		p, toVisit = toVisit[0], toVisit[1:]

		if seen(p) {
			continue
		}
		seenMap[p] = true

		// If we start seeing a bit of luminance, stop - this is the end of the lunar limb
		if gray := ColToGrayU16(img.At(p.X, p.Y)); gray > thresh {
			continue
		}

		ll.Grow(p)
		dci.Plot(p)

		if p.X > bounds.Min.X && !seen(image.Point{p.X-1,p.Y}) {
			toVisit = append(toVisit, image.Point{p.X-1, p.Y})
		}
		if p.Y > bounds.Min.Y && !seen(image.Point{p.X, p.Y-1}) {
			toVisit = append(toVisit, image.Point{p.X, p.Y-1})
		}
		if p.X < bounds.Max.X && !seen(image.Point{p.X+1,p.Y}) {
			toVisit = append(toVisit, image.Point{p.X+1, p.Y})
		}
		if p.Y < bounds.Max.Y && !seen(image.Point{p.X,p.Y+1}) {
			toVisit = append(toVisit, image.Point{p.X, p.Y+1})
		}
	}
	
	dci.PlotRectangle(ll.Bounds)
	if cfg.Verbosity > 0 {
		dci.Flush()
	}

	if ll.Radius() == 0 {
		log.Fatal("Could not locate lunar limb, stopping\n")
	}
	
	return ll
}

// computeLuminalCenter finds the 'centre of mass' for the image
// illumination. We expect this to be somewhere inside the lunar limb,
// so we can use it as a startpoint for the flood fill.
//
// It ignores dim pixels (img noise) and very bright
// pixels (they tend to pull too far one direction) - what we hope
// is left are the corona pixels.
//
// It also figures out a brightness value that is the average gray
// color of pixels in the lunar limb. The floodfiller uses this so it
// can handle images with a very bright (or very dim) initial corona
// boundary.
func (ll *LunarLimb)computeLuminalCenter(img image.Image) {
	sumX, sumY, n := 0,0,0
	b := img.Bounds()
	for x:= b.Min.X; x<=b.Max.X; x++ {
		for y:= b.Min.Y; y<=b.Max.Y; y++ {
			gray := ColToGrayU16(img.At(x,y))
			if gray > 0x0300 && gray < 0xfff0 {
				sumX += x
				sumY += y
				n++
			}
		}
	}
	if n == 0 {
		return
	}

	ll.LuminalCenter.X = sumX/n
	ll.LuminalCenter.Y = sumY/n

	for i:=-5; i<5; i++ {
		ll.Brightness += ColToGrayU16(img.At(ll.LuminalCenter.X+i, ll.LuminalCenter.Y))  // [0, 0xFFFF]
	}
	ll.Brightness /= 10
}

// Col2GrayU16 maps a color into a gray value in the range [0, 0xFFFF]. If we had more
// of a handle on the color, maybe we'd map it to XYZ and pick out the luminance; but
// this works just fine.
func ColToGrayU16(c color.Color) uint16 {
	r, g, b, _ := c.RGBA() // channel values in range [0, 0xFFFF]
	gray := float64(r) * 0.2989 + float64(g) * 0.5870 + float64(b) * 0.1140
	if gray > 0xFFFF { gray = 0xFFFF }

	return uint16(gray)
}


//// All this dci stuff is for debugging - dumping out an image that overlays all the lunar limbs

var dci DebugCompositeImage

type DebugCompositeImage struct {
	fillMap *image.RGBA
	currFrame int
	center image.Point
	maxFrames int
}

func  (dci *DebugCompositeImage)PickColor() color.RGBA64 {
	plotColors := []color.RGBA64{
		color.RGBA64{0xa000, 0, 0, 0xffff},
		color.RGBA64{0, 0xa000, 0, 0xffff},
		color.RGBA64{0, 0, 0xa000, 0xffff},
		color.RGBA64{0x7000, 0x7000, 0, 0xffff},
		color.RGBA64{0x7000, 0, 0x7000, 0xffff},
		color.RGBA64{0, 0x7000, 0x7000, 0xffff},
		color.RGBA64{0xb000, 0x3000, 0x7000, 0xffff},
	}
	return plotColors[dci.currFrame % len(plotColors)]
}

func (dci *DebugCompositeImage)StartNewFrame(bounds image.Rectangle, center image.Point) {
	if dci.fillMap == nil {
		dci.fillMap = image.NewRGBA(bounds)
		dci.center = center
		dci.maxFrames = 5
	} else {
		dci.currFrame++
	}
	dci.PlotMarker(center)
}

func (dci *DebugCompositeImage)Plot(p image.Point) {
	thetaRadians := math.Atan2(float64(p.Y-dci.center.Y), float64(p.X-dci.center.X))
	thetaDegrees := 180 + thetaRadians * 180.0 / math.Pi
	segment := int(thetaDegrees / 12)
	if (segment % dci.maxFrames) != dci.currFrame {
		return
	}
	dci.fillMap.Set(p.X, p.Y, dci.PickColor())
}

func (dci *DebugCompositeImage)PlotRectangle(r image.Rectangle) {
	col := dci.PickColor()
	for x:=r.Min.X; x<=r.Max.X; x++ {
		dci.fillMap.Set(x, r.Min.Y, col)
		dci.fillMap.Set(x, r.Max.Y, col)
	}
	for y:=r.Min.Y; y<=r.Max.Y; y++ {
		dci.fillMap.Set(r.Min.X, y, col)
		dci.fillMap.Set(r.Max.X, y, col)
	}
}

func (dci *DebugCompositeImage)PlotMarker(p image.Point) {
	dci.PlotRectangle(image.Rectangle{image.Point{p.X-2, p.Y-2}, image.Point{p.X+2, p.Y+2}})
	dci.PlotRectangle(image.Rectangle{image.Point{p.X-4, p.Y-4}, image.Point{p.X+4, p.Y+4}})
	dci.PlotRectangle(image.Rectangle{image.Point{p.X-6, p.Y-6}, image.Point{p.X+6, p.Y+6}})
}

func (dci *DebugCompositeImage)Flush() {
	WritePNG(dci.fillMap, "010-lunarlimb-composite.png")
}
