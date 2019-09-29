package main

import (
	"fmt"
	"github.com/chai2010/webp"
	"github.com/ezdiy/image/jpeg"
	"github.com/ezdiy/image/openjpeg"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
	"image"
	"image/gif"
	"image/png"
	"io"
	"log"
	"os"
	"path/filepath"
)

type encFun func(w io.Writer, i image.Image) error

const rfmts = "[jpg|jp2|png|webp|tiff|bmp|gif]"
const wfmts = "[jpg|jp2|png|webp|tiff|bmp|gif]"
func main() {
	encTab := map[string]encFun{
		".png": png.Encode,
		".bmp": bmp.Encode,
		".jp2": func(w io.Writer, i image.Image) error { return openjpeg.Encode(w.(io.WriteSeeker), i, nil) },
		".gif": func(w io.Writer, i image.Image) error { return gif.Encode(w, i, nil) },
		".jpg": func(w io.Writer, i image.Image) error { return jpeg.Encode(w, i, nil) },
		".tiff": func(w io.Writer, i image.Image) error { return tiff.Encode(w, i, nil) },
		".webp": func(w io.Writer, i image.Image) error { return webp.Encode(w, i, nil) },
	}
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: imgconv input.%s output.%s\n",rfmts,wfmts)
		return
	}
	in, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	img, inFmt, err := image.Decode(in)
	log.Printf("Decoded %s: %dx%d %s\n", in.Name(), img.Bounds().Dx(), img.Bounds().Dy(), inFmt)
	_ = in.Close()
	out, err := os.Create(os.Args[2])
	ofmt := filepath.Ext(out.Name())
	log.Printf("Encoding to into %s\n", out.Name())
	if encFun, ok := encTab[ofmt]; ok {
		err := encFun(out, img)
		if err != nil {
			panic(err)
		}
		log.Printf("Done")
	} else {
		log.Printf("Unknown output format %s\n", ofmt)
	}
	return
}