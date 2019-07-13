package estack

// The goroutine nonsense to process the stack and generate output pixels. The output image
// data structure is not thread safe.

import(
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"sync"
	"time"
)

var(
	nWorkers = 24
)

// An ImgWrite is the instruction that a sliveworker sends to the writeworker
type ImgWrite struct {
	c color.Color
	x,y int
}

// {{{ s.RenderOutputFile

func (s *Stack)RenderOutputFile() error {	
	if img,err := s.GenerateOutputImage(); err != nil {
		return err
	} else if writer,err := os.Create(s.Rendering.OutputFilename); err != nil {
		return fmt.Errorf("open+w '%s': %v", s.Rendering.OutputFilename, err)
	} else {
		defer writer.Close()
		return png.Encode(writer, img)
	}
}

// }}}
// {{{ s.GenerateOutputImage

func (s Stack)GenerateOutputImage() (image.Image, error) {	
	tStart := time.Now();

	var wg sync.WaitGroup
	var writesChan = make(chan ImgWrite, 32)

	img := image.NewRGBA(s.Rendering.Bounds) // This is the subarea we're rendering
	go writeWorker(img, writesChan)

	width := img.Bounds().Dx()
	height := img.Bounds().Dy()

	jobWidth := width / nWorkers
	remainder := (jobWidth % nWorkers)
	
	wg.Add(nWorkers)

	for i:=0; i<nWorkers; i++ {
		width := jobWidth
		if i == nWorkers-1 { width += remainder } // last worker picks up remainder

		// Describe the vertical stripe (of s.Bounds) each worker will get to work on
		min := image.Point{
			X: s.Rendering.Bounds.Min.X + i*jobWidth,
			Y: s.Rendering.Bounds.Min.Y,
		}
		max := image.Point{
			X: min.X+width,
			Y: s.Rendering.Bounds.Max.Y,
		}
		
		go func(area image.Rectangle) {
			s.generateSliceWorker(writesChan, area)
			defer wg.Done()
		}(image.Rectangle{Min:min, Max:max})
	}

	wg.Wait()
	close(writesChan)
	
	fmt.Printf("(renderoptions: %#v)\n", s.Rendering)
	fmt.Printf("(%dx%d - took %s, with %d workers)\n", width, height, time.Since(tStart), nWorkers)

	for _,hist := range Hists {
		fmt.Printf("%s\n", hist)
	}
	
	return img, nil
}

// }}}

// {{{ writeWorker

// This goroutine gates write access to the output image (the data structure is not thread safe)
func writeWorker(img *image.RGBA, writesChan <-chan ImgWrite) {
	for {
		write, moreToCome := <-writesChan
		if !moreToCome { return }
		img.Set(write.x, write.y, write.c)
	}
}

// }}}
// {{{ generateSliceWorker

// This goroutine iterates over the pixels in its subarea, processes them, and submits them
// to the writeWorker.
func (s Stack) generateSliceWorker(writes chan<-ImgWrite, area image.Rectangle) {
	for x := area.Min.X; x < area.Max.X; x++ {
		for y := area.Min.Y; y < area.Max.Y; y++ {
			writes<- ImgWrite{s.CombineImagesAt(x,y), x, y}
		}
	}
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
