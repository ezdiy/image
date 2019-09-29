package openjpeg

import (
	"errors"
	"image"
	"io"
)

func Encode(o io.WriteSeeker, img image.Image, opt *Options) (err error) {
	c := newCodec(0)
	c.WriteSeeker = o
	if !c.encode(img, opt) {
		err = c.err
		if err == nil {
			return errors.New("encode failed")
		}
	}
	return
}

func DecodeConfig(r io.Reader) (cfg image.Config, err error) {
	c := newCodec(1)
	c.Reader = r
	if !c.parseHeader(&cfg) {
		err = c.err
		if err == nil {
			err = errors.New("parseHeader failed")
			return
		}
	}
	return
}

func Decode(r io.Reader) (img image.Image, err error) {
	c := newCodec(1)
	c.Reader = r
	if !c.parseHeader(nil) {
		if err == nil {
			err = errors.New("parseHeader failed")
			return
		}
	}
	img = c.decode()
	err = c.err
	return
}

func init() {
	image.RegisterFormat("jpeg2000", string(rfc3745Magic), Decode, DecodeConfig)
	image.RegisterFormat("jpeg2000", string(jp2Magic), Decode, DecodeConfig)
	image.RegisterFormat("jpeg2000", string(j2kMagic), Decode, DecodeConfig)
}
