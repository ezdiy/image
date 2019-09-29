package openjpeg

import (
	"image/png"
	"log"
	"os"
	"testing"
)

func xTestDecode(t *testing.T) {
	c := newCodec(1)
	c.Reader, _ = os.Open("e:/test2.jp2")
	err := c.parseHeader(nil)
	log.Println(err)
	img := c.decode()
	outf, _ := os.Create("e:/of.png")
	png.Encode(outf, img)
	log.Println(c.err)
}

func TestEncode(t *testing.T) {
	c := newCodec(0)
	ingpng, _ := os.Open("e:/of.png")
	img, err := png.Decode(ingpng)
	if img == nil {
		t.Fatal(err)
	}
	outf, _ := os.Create("e:/of.jp2")
	c.WriteSeeker = outf
	c.encode(img, &Options{
		BPP:4,
		Ratio:[]float32{15},
		NResolutions: 2,
	})
	log.Println(c.err)
}
