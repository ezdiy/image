package util

import (
	"image"
	"image/color"
	"image/draw"
	"math/rand"
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

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func IsGray(img image.Image, fuzz int) bool {
	if _, ok := img.(*image.Gray); ok {
		return true
	}
	if fuzz == -1 || fuzz > 255 {
		return true
	}
	// Otherwise sample few pixels and check if they're gray
	for i := 0; i < 256; i++ {
		r,g,b,_ := img.At(rand.Intn(img.Bounds().Dx()), rand.Intn(img.Bounds().Dy())).RGBA()
		rg := abs(int(r-g))
		rb := abs(int(r-b))
		if abs(rg-rb) > fuzz {
			return false
		}
	}
	return true
}

// Attempt conversion of the input image into greyscale. Fuzz is
// is a threshold when a color picture is considered greyscale,
// Higher fuzz will accept more of color variance.
// If picture is above fuzz, "too colorish", nil is returned.
// If fuzz threshold is -1, the conversion is done always.
func ToGray(img image.Image, fuzz int) (gr *image.Gray) {
	// Already greyscale, no need to convert.
	if im, ok := img.(*image.Gray); ok {
		return im
	}
	if !IsGray(img, fuzz) {
		return nil
	}
	// Fastest: For YUV image, we can just take the Y component
	if yuv, ok := img.(*image.YCbCr); ok {
		return &image.Gray{
			Rect:yuv.Rect,
			Pix:yuv.Y,
			Stride:yuv.YStride,
		}
	}
	// Fast: RGB, bias-average
	var pix []byte
	var stride int
	if rgba, ok := img.(*image.RGBA); ok {
		pix, stride = rgba.Pix, rgba.Stride
	} else if nrgba, ok := img.(*image.NRGBA); ok {
		pix, stride = nrgba.Pix, nrgba.Stride
	}
	if pix != nil {
		opix := make([]byte, len(pix)/4)
		gr = &image.Gray{
			Pix:opix,
			Stride:stride/4,
		}
		for i, j := 0, 0; i < len(opix); i++ {
			pp := pix[j:][:4]
			opix[i] = byte((uint32(pp[0])*19595+uint32(pp[1])*38470+uint32(pp[2])*7471+32768)>>24)
			j += 4
		}
		return
	}
	// Slow: use draw
	return ToModel(img, color.GrayModel).(*image.Gray)
}

func ToModel(img image.Image, c color.Model) (gr image.Image) {
	if img.ColorModel() == c {
		return img
	}
	b := img.Bounds()
	gr = NewImage(c, image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(gr.(draw.Image), gr.Bounds(), img, b.Min, draw.Src)
	return
}