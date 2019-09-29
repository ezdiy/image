package util

import (
	"image"
	"image/color"
)

// Create a new Image of specified color model.
func NewImage(c color.Model, r image.Rectangle) image.Image {
	switch c {
	case color.RGBAModel:
		return image.NewRGBA(r)
	case color.RGBA64Model:
		return image.NewRGBA64(r)
	case color.NRGBAModel:
		return image.NewNRGBA(r)
	case color.NRGBA64Model:
		return image.NewNRGBA64(r)
	case color.GrayModel:
		return image.NewGray(r)
	case color.Gray16Model:
		return image.NewGray16(r)
	case color.CMYKModel:
		return image.NewCMYK(r)
	case color.AlphaModel:
		return image.NewAlpha(r)
	case color.Alpha16Model:
		return image.NewAlpha16(r)
	}
	return nil
}

func NewImageSSR(c color.Model, ssr image.YCbCrSubsampleRatio, r image.Rectangle) image.Image {
	if c == color.YCbCrModel {
		return image.NewYCbCr(r, ssr)
	}
	return NewImage(c, r)
}

// Create a new image "in the image" of template, that is, of same color model.
func NewImageColorAs(template image.Image, r image.Rectangle) {
	NewImageSSR(template.ColorModel(), GetSSR(template, image.YCbCrSubsampleRatio444), r)
}

// Get sub-sampling ratio if color space has one (0 = no sub-sampling).
func GetSSR(r image.Image, alt image.YCbCrSubsampleRatio) image.YCbCrSubsampleRatio {
	yuv := r.(*image.YCbCr)
	if yuv != nil {
		return yuv.SubsampleRatio
	}
	return alt
}

func GetPixStride(i image.Image) ([]byte, int) {
	switch i.(type) {
	case *image.RGBA:
		return i.(*image.RGBA).Pix, i.(*image.RGBA).Stride
	case *image.RGBA64:
		return i.(*image.RGBA64).Pix, i.(*image.RGBA64).Stride
	case *image.NRGBA:
		return i.(*image.NRGBA).Pix, i.(*image.NRGBA).Stride
	case *image.NRGBA64:
		return i.(*image.NRGBA64).Pix, i.(*image.NRGBA64).Stride
	case *image.Gray:
		return i.(*image.Gray).Pix, i.(*image.Gray).Stride
	case *image.Gray16:
		return i.(*image.Gray16).Pix, i.(*image.Gray16).Stride
	case *image.CMYK:
		return i.(*image.CMYK).Pix, i.(*image.CMYK).Stride
	case *image.Alpha:
		return i.(*image.Alpha).Pix, i.(*image.Alpha).Stride
	case *image.Alpha16:
		return i.(*image.Alpha16).Pix, i.(*image.Alpha16).Stride
	}
	return nil, -1
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

// Indices match the enum. Format is vertical * 16 + horizontal.
var yvh = [6]byte{0x11, 0x12, 0x22, 0x21, 0x14, 0x24}

const YCbCrSubsampleRatioUnknown = -1

// Translate vertical-horizontal chroma divisors to Go's sub-sampling rate.
func VHDiv2SSR(v, h int) image.YCbCrSubsampleRatio {
	b := byte((v << 4) | h)
	for i := 0; i < len(yvh); i++ {
		if yvh[i] == b {
			return image.YCbCrSubsampleRatio(i)
		}
	}
	return YCbCrSubsampleRatioUnknown
}

// Translate Go's sub-sampling rate to V/H divisors.
func SSR2VHDiv(i image.YCbCrSubsampleRatio) (v, h int) {
	return int(yvh[i] >> 4), int(yvh[i] & 15)
}
