package estack

import(
	"fmt"
)

type Stack struct {
	Images        []StackedImage
	Configuration
}

func NewStack() Stack {
	return Stack{
		Images: []StackedImage{},
		Configuration: NewConfiguration(),
	}
}

func (s *Stack)Add(si StackedImage) {
	s.Images = append(s.Images, si)
}

func (s Stack)String() string {
	str := "Stack[\n"
	for _,si := range s.Images {
		str += fmt.Sprintf("  %s\n", si)
	}
	return str + "]\n"
}
