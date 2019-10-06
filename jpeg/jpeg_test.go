package jpeg

import (
	"bytes"
	"image"
	"image/color"
	"runtime"
	"testing"
)

func one(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 640, 480))
	for i := 0; i < 640; i++ {
		for j := 0; j < 480; j++ {
			b := byte(i)
			img.Set(i, j, color.RGBA{179 & b, 128 + b, 64 - b, 255})
		}
	}
	buf := bytes.NewBuffer(nil)
	err := Encode(buf, img, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
}

func TestEncodeDecode(t *testing.T) {
	one(t)
	runtime.GC()
	runtime.GC()
	one(t)
	runtime.GC()
	runtime.GC()
}
