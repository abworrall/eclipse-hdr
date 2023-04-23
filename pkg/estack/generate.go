package estack

import(
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
)

func (s *Stack)FuseExposures() {
	inBbox := s.InputArea
	outBbox := image.Rectangle{Max:image.Point{inBbox.Dx(), inBbox.Dy()}}  // output coords: [0,0] -> [max,max]

	s.OutputArea = outBbox
	
	s.Pixels = make([][]*PixelWorkspace, outBbox.Dx())	
	for x := 0; x < outBbox.Dx(); x++ {
		s.Pixels[x] = make([]*PixelWorkspace, outBbox.Dy())
	}

	log.Printf("FuseExposures: iterating over %s\n", s.OutputArea)
	
	// Iterate over pixels, apply the given algo to fuse each one
	for x := inBbox.Min.X; x < inBbox.Max.X; x++ {
		for y := inBbox.Min.Y; y < inBbox.Max.Y; y++ {
			ws := s.GetPixelWorkspaceForInputAt(x,y)
			ws.FuseExposures()
			
			ws.OutputPos = image.Point{x-inBbox.Min.X, y-inBbox.Min.Y}
			s.Pixels[ws.OutputPos.X][ws.OutputPos.Y] = ws
		}
	}
}

func (s *Stack)Tonemap() {
	tmo := s.Configuration.GetTonemapper()
	tmo(s)
}

func (s *Stack)DevelopAndPublish() {
	s.CompositeLDR = image.NewRGBA64(s.OutputArea)

	for x := 0; x < s.OutputArea.Dx(); x++ {
		for y := 0; y < s.OutputArea.Dy(); y++ {
			ws := s.Pixels[x][y]
			ws.DNGDevelop()    // 3. Apply the DNG Develop algo                      (TonemappedRGB -> DevelopedRGB)
		}
	}

	for x := 0; x < s.OutputArea.Dx(); x++ {
		for y := 0; y < s.OutputArea.Dy(); y++ {
			ws := s.Pixels[x][y]

			ws.Publish()       // 4. Publish an output pixel (e.g. apply sRGB gamma) (DevelopedRGB -> Output)
			ws.ColorTweaks()   // 5. Final tweaks to pixels, for debugging           (Output -> Output)

			if DebugPixels != nil {
				if _, exists := DebugPixels[image.Point{x, y}]; exists {
					ws.DebugDump()
				}
			}

			s.CompositeLDR.Set(x, y, ws.Output)
		}
	}
}

func WritePNG(img image.Image, filename string) error {
	if writer, err := os.Create(filename); err != nil {
		return fmt.Errorf("open+w '%s': %v", filename, err)
	} else {
		defer writer.Close()
		return png.Encode(writer, img)
	}
}
