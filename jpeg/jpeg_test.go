package jpeg

import (
	"image"
	"log"
	"os"
	"testing"
)

func TestDecode(t *testing.T) {
	f, _ := os.Open("e:/wtf.jpg")
	var cfg image.Config
	NewReader(f, &cfg)
	log.Println(cfg.Width)
	log.Println(cfg.Height)
	log.Printf("%v\n",cfg.ColorModel)
}
