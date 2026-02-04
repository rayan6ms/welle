package object

import (
	"fmt"
	"math"
)

type Image struct {
	Width  int
	Height int
	Data   []uint8
}

func (*Image) Type() Type { return IMAGE_OBJ }
func (i *Image) Inspect() string {
	return fmt.Sprintf("image[%dx%d]", i.Width, i.Height)
}

func NewImage(width, height int) (*Image, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("image_new expects positive width/height")
	}
	data := make([]uint8, width*height*4)
	return &Image{Width: width, Height: height, Data: data}, nil
}

func (i *Image) SetPixel(x, y, r, g, b, a int) error {
	if x < 0 || y < 0 || x >= i.Width || y >= i.Height {
		return fmt.Errorf("image_set out of bounds")
	}
	if r < 0 || r > 255 || g < 0 || g > 255 || b < 0 || b > 255 || a < 0 || a > 255 {
		return fmt.Errorf("image_set expects channels in 0..255")
	}
	idx := (y*i.Width + x) * 4
	i.Data[idx] = uint8(r)
	i.Data[idx+1] = uint8(g)
	i.Data[idx+2] = uint8(b)
	i.Data[idx+3] = uint8(a)
	return nil
}

func (i *Image) Fill(r, g, b, a int) error {
	if r < 0 || r > 255 || g < 0 || g > 255 || b < 0 || b > 255 || a < 0 || a > 255 {
		return fmt.Errorf("image_fill expects channels in 0..255")
	}
	for idx := 0; idx < len(i.Data); idx += 4 {
		i.Data[idx] = uint8(r)
		i.Data[idx+1] = uint8(g)
		i.Data[idx+2] = uint8(b)
		i.Data[idx+3] = uint8(a)
	}
	return nil
}

func (i *Image) FadeToWhite(amount float64) error {
	if amount < 0 || amount > 1 {
		return fmt.Errorf("image_fade_white expects amount in 0..1")
	}
	a := amount
	for idx := 0; idx < len(i.Data); idx++ {
		old := float64(i.Data[idx])
		v := math.Round(old*(1-a) + 255*a)
		if v < 0 {
			v = 0
		}
		if v > 255 {
			v = 255
		}
		i.Data[idx] = uint8(v)
	}
	return nil
}

func (i *Image) FillRect(x, y, w, h, r, g, b, a int) error {
	if x < 0 || y < 0 || w <= 0 || h <= 0 || x+w > i.Width || y+h > i.Height {
		return fmt.Errorf("image_fill_rect out of bounds")
	}
	if r < 0 || r > 255 || g < 0 || g > 255 || b < 0 || b > 255 || a < 0 || a > 255 {
		return fmt.Errorf("image_fill_rect expects channels in 0..255")
	}
	for yy := y; yy < y+h; yy++ {
		idx := (yy*i.Width + x) * 4
		for xx := 0; xx < w; xx++ {
			i.Data[idx] = uint8(r)
			i.Data[idx+1] = uint8(g)
			i.Data[idx+2] = uint8(b)
			i.Data[idx+3] = uint8(a)
			idx += 4
		}
	}
	return nil
}

func (i *Image) Fade(amount float64) error {
	if amount < 0 || amount > 1 {
		return fmt.Errorf("image_fade expects amount in 0..1")
	}
	scale := 1 - amount
	for idx := 0; idx < len(i.Data); idx++ {
		v := math.Round(float64(i.Data[idx]) * scale)
		if v < 0 {
			v = 0
		}
		if v > 255 {
			v = 255
		}
		i.Data[idx] = uint8(v)
	}
	return nil
}
