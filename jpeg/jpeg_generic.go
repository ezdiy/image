//+build !cgo

package jpeg

import (
	"image"
	"image/jpeg"
	"io"
)

// That feel when no cgo.

func Transform(w io.Writer, r io.Reader, o *Options) (err error) {
	// Nothing cropped, so just pipe the image as-is.
	if o.Rectangle == nil {
		_, err = io.Copy(w, r)
		return
	}
	// Cropping, will have to re-code.
	i, e := Decode(r)
	if e != nil {
		return e
	}
	return Encode(w, i, o)
}

func Encode(w io.Writer, m image.Image, o *Options) error {
	return jpeg.Encode(w, m, o)
}

func Decode(r io.Reader) (image.Image, error) {
	return jpeg.Decode(r)
}

func DecodeConfig(r io.Reader) (image.Config, error) {
	return jpeg.DecodeConfig(r)
}

// no need to register image format as the import of image/jpeg does that already
