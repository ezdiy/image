//+build !cgo

package jpeg

import (
	"github.com/ezdiy/image/util"
	"github.com/getlantern/errors"
	"image"
	"image/jpeg"
	"io"
)

// Transform JPG image from r into w, applying options.
// If possible, this operation is loss-less (re-coding only DCT coefficients).
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

// Encode image into w, while applying Options.
func Encode(w io.Writer, m image.Image, o *Options) error {
	oo := &jpeg.Options{
		Quality: 100,
	}
	if o != nil {
		oo.Quality = o.Quality
		if o.Rectangle != nil {
			m = util.Crop(m, o.Rectangle)
			if m == nil {
				return errors.New("image doesn't support cropping")
			}
		}
	}
	return jpeg.Encode(w, m, oo)
}

// Decode jpg image.
func Decode(r io.Reader) (image.Image, error) {
	return jpeg.Decode(r)
}

// Probe for jpg image.
func DecodeConfig(r io.Reader) (image.Config, error) {
	return jpeg.DecodeConfig(r)
}

// no need to register image format as the import of image/jpeg does that already
