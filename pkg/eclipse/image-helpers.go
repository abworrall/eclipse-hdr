package eclipse

// A few helper routines for golang's image libraries

import(
	"fmt"
	"image"
	"image/png"
	"os"
)

func RectCenter(b image.Rectangle) image.Point {
	return image.Point{(b.Min.X + b.Max.X) / 2, (b.Min.Y + b.Max.Y) / 2}
}

func GrowRectangle(r image.Rectangle, p image.Point) image.Rectangle {
	if p.X < r.Min.X {
		r.Min.X = p.X
	} else if p.X > r.Max.X {
		r.Max.X = p.X
	}

	if p.Y < r.Min.Y {
		r.Min.Y = p.Y
	} else if p.Y > r.Max.Y {
		r.Max.Y = p.Y
	}

	return r
}

func WritePNG(img image.Image, filename string) error {
	if writer, err := os.Create(filename); err != nil {
		return fmt.Errorf("open+w '%s': %v", filename, err)
	} else {
		defer writer.Close()
		return png.Encode(writer, img)
	}
}
