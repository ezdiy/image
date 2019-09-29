package util

import (
	"image"
	"image/color"
)

// Create a new Image of specified color model.
func NewImage(c color.Model, ssr image.YCbCrSubsampleRatio, r image.Rectangle) image.Image {
	switch c {
	case color.RGBAModel:
		return image.NewRGBA(r)
	case color.RGBA64Model:
		return image.NewRGBA64(r)
	case color.NRGBAModel:
		return image.NewNRGBA(r)
	case color.NRGBA64Model:
		return image.NewNRGBA64(r)
	case color.YCbCrModel:
		return image.NewYCbCr(r, ssr)
	case color.GrayModel:
		return image.NewGray(r)
	case color.Gray16Model:
		return image.NewGray16(r)
	}
	return nil
}

// Create a new image "in the image" of template, that is, of same color model.
func NewImageColorAs(template image.Image, r image.Rectangle) {
	NewImage(template.ColorModel(), GetSSR(template, image.YCbCrSubsampleRatio422), r)
}

// Get sub-sampling ratio if color space has one (0 = no sub-sampling).
func GetSSR(r image.Image, alt image.YCbCrSubsampleRatio) image.YCbCrSubsampleRatio {
	yuv := r.(*image.YCbCr)
	if yuv != nil {
		return yuv.SubsampleRatio
	}
	return alt
}

type SubImage interface {
	SubImage(r image.Rectangle) image.Image
}

func Crop(i image.Image, rectangle *image.Rectangle) image.Image {
	cr := i.(SubImage)
	if cr == nil {
		return nil
	}
	return cr.SubImage(*rectangle)
}
