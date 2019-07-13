package estack

import(
	"fmt"
	"image"
)

// Maps filenames to [X,Y] coords. Note that origin is top-left, so Y axis goes downwards.
type AlignmentData map[string][2]int

func NewAlignmentData() AlignmentData {
	return map[string][2]int{}
}

func (s *Stack)AlignImages() error {
	for i:=0; i<len(s.Images); i++ {
		if vals,exists := s.Alignment[s.Images[i].Filename]; !exists {
			return fmt.Errorf("image '%s' not listed in alignment data", s.Images[i].Filename)
		} else {
			s.Images[i].Offset = image.Point{X:vals[0], Y:vals[1]}
		}
	}
	return nil
}

/*

	// Second series
	if strings.Contains(si.Filename, "5668") {
		si.Offset = image.Point{4,-11}
	} else if strings.Contains(si.Filename, "5669") {
		si.Offset = image.Point{6,-13}
	} else if strings.Contains(si.Filename, "5670") {
		si.Offset = image.Point{10,-16}
	} else if strings.Contains(si.Filename, "5671") {
		si.Offset = image.Point{12,-18}
	}

*/

// This is for the star in the first image series :)
//s.Bounds.Min = image.Point{X:1600, Y:2200}
//s.Bounds.Max = image.Point{X:1800, Y:2400}

// This is for the star in the second image series :)
//s.Bounds.Min = image.Point{X:1400, Y:2000}
//s.Bounds.Max = image.Point{X:2200, Y:2600}
