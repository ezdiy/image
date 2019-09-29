//+build cgo

package openjpeg

/*
#cgo CFLAGS: -O2 -fomit-frame-pointer
#cgo windows AND amd64 CFLAGS: -Iinclude
#cgo !windows OR !amd64 LDFLAGS: -lopenjp2
#ifndef OPJ_STATIC
#define OPJ_STATIC
#endif
#include <openjpeg-2.3/openjpeg.h>
OPJ_SIZE_T readFunc(void *p_buffer, OPJ_SIZE_T p_nb_bytes, void *p_user_data);
OPJ_SIZE_T writeFunc(void *p_buffer, OPJ_SIZE_T p_nb_bytes, void *p_user_data);
OPJ_OFF_T skipFunc(OPJ_OFF_T p_nb_bytes, void *p_user_data);
OPJ_SIZE_T seekFunc(OPJ_OFF_T p_nb_bytes, void *p_user_data);
void msgFunc(const char *msg, void *p);
 */
import "C"
import (
	"bytes"
	"github.com/ezdiy/image/util"
	"image"
	"image/color"
	"io"
	"math"
	"runtime"
	"unsafe"
)

const bufSize = 1<<16
const magicLen = 12
var (
	rfc3745Magic = []byte("\x00\x00\x00\x0c\x6a\x50\x20\x20\x0d\x0a\x87\x0a")
	jp2Magic     = []byte("\x0d\x0a\x87\x0a")
	j2kMagic     = []byte("\xff\x4f\xff\x51")
)


type codec struct {
	codec    *C.opj_codec_t
	stream   *C.opj_stream_t
	image 	 *C.opj_image_t
	magicPos int
	magic    [magicLen]byte
	err 	 error
	io.Reader
	io.WriteSeeker
}

func (c *codec) destroy() {
	C.opj_stream_destroy(c.stream)
	C.opj_destroy_codec(c.codec)
	C.opj_image_destroy(c.image)
	c.stream = nil
	c.codec = nil
	c.image = nil
}


func newCodec(isRead int) (c *codec) {
	c = &codec{}
	c.stream = C.opj_stream_create(bufSize, C.OPJ_BOOL(isRead))
	C.opj_stream_set_user_data_length(c.stream, C.OPJ_UINT64(math.MaxInt64))
	C.opj_stream_set_user_data(c.stream, unsafe.Pointer(c), nil)
	C.opj_stream_set_read_function(c.stream, (*[0]byte)(C.readFunc))
	C.opj_stream_set_write_function(c.stream, (*[0]byte)(C.writeFunc))
	if isRead == 0 {
		C.opj_stream_set_seek_function(c.stream, (*[0]byte)(C.seekFunc))
	}
	C.opj_stream_set_skip_function(c.stream, (*[0]byte)(C.skipFunc))
	runtime.SetFinalizer(c, func(p interface{}) {
		p.(*codec).destroy()

	})
	return
}

func (c *codec) parseHeader(config *image.Config) (ok bool) {
	if _, c.err = io.ReadFull(c, c.magic[:]); c.err != nil {
		return
	}
	format := C.OPJ_CODEC_FORMAT(C.OPJ_CODEC_JPT)
	if bytes.Equal(c.magic[:], rfc3745Magic) || bytes.Equal(c.magic[:4], jp2Magic) {
		format = C.OPJ_CODEC_JP2
	} else if bytes.Equal(c.magic[:2], j2kMagic) {
		format = C.OPJ_CODEC_J2K
	}
	c.codec = C.opj_create_decompress(format)
	C.opj_set_error_handler(c.codec, (*[0]byte)(C.msgFunc), unsafe.Pointer(&c.err))
	C.opj_set_warning_handler(c.codec, (*[0]byte)(C.msgFunc), unsafe.Pointer(&c.err))
	C.opj_set_info_handler(c.codec, (*[0]byte)(C.msgFunc), unsafe.Pointer(&c.err))
	var par C.opj_dparameters_t
	C.opj_set_default_decoder_parameters(&par)
	if C.opj_setup_decoder(c.codec, &par) == 0 {
		return
	}
	if C.opj_read_header(c.stream, c.codec, &c.image) == 0 {
		return
	}
	if config != nil {
		switch c.image.color_space {
		case C.OPJ_CLRSPC_SRGB:
			config.ColorModel = color.RGBAModel
		case C.OPJ_CLRSPC_GRAY:
			config.ColorModel = color.GrayModel
		case C.OPJ_CLRSPC_SYCC:
			if c.getSSR() == util.YCbCrSubsampleRatioUnknown {
				return false
			}
			config.ColorModel = color.YCbCrModel
		case C.OPJ_CLRSPC_CMYK:
			config.ColorModel = color.CMYKModel
		default:
			return false
		}
		config.Width = int(c.image.x1)
		config.Height = int(c.image.y1)
		c.destroy()
	}
	return true
}

func (c *codec) comp(i int) *C.opj_image_comp_t {
	if i >= int(c.image.numcomps) {
		return nil
	}
	return &((*[math.MaxInt32]C.opj_image_comp_t)(unsafe.Pointer(c.image.comps)))[i]
}

func (c *C.opj_image_comp_t) Data() []int32 {
	return (*[math.MaxInt32]int32)(unsafe.Pointer(c.data))[:]
}

func (c *codec) getSSR() (ssr image.YCbCrSubsampleRatio) {
	ssr = util.YCbCrSubsampleRatioUnknown
	if c.image.numcomps != 3 {
		return
	}
	c0, c1, c2 := c.comp(0), c.comp(1), c.comp(2)
	if c0.dx != 1 || c0.dy != 1 {
		return
	}
	if c1.dx != c2.dx || c1.dy != c2.dy {
		return
	}
	return util.VHDiv2SSR(int(c1.dy), int(c1.dx))
}

func (c *C.opj_image_comp_t) encodeComp(buf []byte, stride int) {
	d := uintptr(unsafe.Pointer(c.data))
	shr := uint(8-c.prec)&7
	dbuf := uintptr(unsafe.Pointer(&buf[0]))
	for i, j := uintptr(0), uintptr(0); i < uintptr(len(buf)); i += uintptr(stride) {
		*((*uint32)(unsafe.Pointer(d + j))) = uint32(*(*byte)(unsafe.Pointer(dbuf + i))) >> shr
		j += 4
	}
}

func (c *C.opj_image_comp_t) decodeComp(buf []byte, stride int) {
	var shl, shr uint
	// TODO: 16bit variants?
	if c.prec < 8 {
		shl = uint(8-c.prec)
	} else if c.prec > 8 {
		shr = uint(c.prec - 8)
	}

	d := uintptr(unsafe.Pointer(c.data))
	var sig uint32
	if c.sgnd != 0 {
		sig = (1<<(c.prec-1))-1
	}
	shr &= 31
	shl &= 31
	dbuf := uintptr(unsafe.Pointer(&buf[0]))
	for i, j := uintptr(0), uintptr(0); i < uintptr(len(buf)); i += uintptr(stride) {
		*(*byte)(unsafe.Pointer(dbuf + i)) = byte((*(*uint32)(unsafe.Pointer(d + j)) + sig) >> shr << shl)
		j += 4
	}
}

func (c *codec) decode() (img image.Image) {
	if C.opj_decode(c.codec, c.stream, c.image) == 0 || C.opj_end_decompress(c.codec, c.stream) == 0 {
		return
	}
	defer c.destroy()
	switch c.image.color_space {
	case C.OPJ_CLRSPC_SRGB:
		if c.image.numcomps < 3 {
			return
		}
		pic := image.NewRGBA(image.Rect(0,0,int(c.image.x1), int(c.image.y1)))
		pix := pic.Pix
		c.comp(0).decodeComp(pix, 4)
		c.comp(1).decodeComp(pix[1:], 4)
		c.comp(2).decodeComp(pix[2:], 4)
		for i := 3; i < len(pix); i += 4 {
			pix[i] = 0xff
		}
		img = pic
	case C.OPJ_CLRSPC_GRAY:
		pic := image.NewGray(image.Rect(0,0,int(c.image.x1), int(c.image.y1)))
		c.comp(0).decodeComp(pic.Pix, 1)
		img = pic
	case C.OPJ_CLRSPC_SYCC:
		ssr := c.getSSR()
		if ssr == util.YCbCrSubsampleRatioUnknown {
			return
		}
		pic := image.NewYCbCr(image.Rect(0,0,int(c.image.x1), int(c.image.y1)), ssr)
		c.comp(0).decodeComp(pic.Y, 1)
		c.comp(1).decodeComp(pic.Cr, 1)
		c.comp(2).decodeComp(pic.Cb, 1)
		img = pic
	case C.OPJ_CLRSPC_CMYK:
		if c.image.numcomps < 4 {
			return
		}
		pic := image.NewCMYK(image.Rect(0,0,int(c.image.x1), int(c.image.y1)))
		pix := pic.Pix
		c.comp(0).decodeComp(pix, 4)
		c.comp(1).decodeComp(pix[1:], 4)
		c.comp(2).decodeComp(pix[2:], 4)
		c.comp(3).decodeComp(pix[3:], 4)
		img = pic
	default:
		return
	}
	return
}

type Options struct {
	BPP          int
	Ratio        []float32
	PSNR         []float32
	NResolutions int
}

func (c *codec) encode(img image.Image, o *Options) (ok bool) {
//	c.stream = C.opj_stream_create_file_stream(C.CString("e:/off.jp2"), 0x100000, 0)
	defer c.destroy()
	var cspc C.OPJ_COLOR_SPACE
	var cpar [4]C.opj_image_cmptparm_t
	var ncomp int
	switch img.ColorModel() {
	case color.GrayModel:
		cspc = C.OPJ_CLRSPC_GRAY
		ncomp = 1
	case color.RGBAModel:
		cspc = C.OPJ_CLRSPC_SRGB
		ncomp = 3
	case color.CMYKModel:
		cspc = C.OPJ_CLRSPC_CMYK
		ncomp = 4
	case color.YCbCrModel:
		cspc = C.OPJ_CLRSPC_SYCC
		ncomp = 3
	default:
		return
	}
	if o == nil {
		o = &Options{}
	}
	bpp := C.uint(o.BPP)
	if bpp == 0 {
		bpp = 8
	}
	for i := 0; i < ncomp; i++ {
		cpar[i].prec = bpp
		cpar[i].bpp = bpp
		cpar[i].dx = 1
		cpar[i].dy = 1
		cpar[i].w = C.uint(img.Bounds().Dx())
		cpar[i].h = C.uint(img.Bounds().Dy())
	}
	if yimg, yuv := img.(*image.YCbCr); yuv {
		v,h := util.SSR2VHDiv(yimg.SubsampleRatio)
		for i := 1; i < 3;i ++ {
			cpar[i].w /= C.uint(h)
			cpar[i].h /= C.uint(v)
			cpar[i].dx = C.uint(h)
			cpar[i].dy = C.uint(v)
		}
	}

	c.image = C.opj_image_create(C.OPJ_UINT32(ncomp), &cpar[0], cspc)
	if c.image == nil {
		return
	}
	c.image.x0 = 0
	c.image.y0 = 0
	c.image.x1 = C.uint(img.Bounds().Dx())
	c.image.y1 = C.uint(img.Bounds().Dy())

	switch cspc {
	case C.OPJ_CLRSPC_GRAY:
		ig := img.(*image.Gray)
		c.comp(0).encodeComp(ig.Pix, 1)
	case C.OPJ_CLRSPC_SRGB:
		ig := img.(*image.RGBA)
		c.comp(0).encodeComp(ig.Pix, 4)
		c.comp(1).encodeComp(ig.Pix[1:], 4)
		c.comp(2).encodeComp(ig.Pix[2:], 4)
	case C.OPJ_CLRSPC_CMYK:
		ig := img.(*image.CMYK)
		c.comp(0).encodeComp(ig.Pix, 4)
		c.comp(1).encodeComp(ig.Pix[1:], 4)
		c.comp(2).encodeComp(ig.Pix[2:], 4)
		c.comp(2).encodeComp(ig.Pix[3:], 4)
	case C.OPJ_CLRSPC_SYCC:
		ig := img.(*image.YCbCr)
		c.comp(0).encodeComp(ig.Y, 1)
		c.comp(1).encodeComp(ig.Cb, 1)
		c.comp(2).encodeComp(ig.Cr, 1)
	}
	var par C.opj_cparameters_t
	c.codec = C.opj_create_compress(C.OPJ_CODEC_JP2)
	if c.codec == nil {
		return
	}
	C.opj_set_default_encoder_parameters(&par)
	if o.Ratio != nil {
		par.tcp_numlayers = C.int(len(o.Ratio))
		for i, v := range o.Ratio {
			par.tcp_rates[i] = C.float(v)
		}
		par.cp_disto_alloc = 1
	} else if o.PSNR != nil {
		par.tcp_numlayers = C.int(len(o.PSNR))
		for i, v := range o.PSNR {
			par.tcp_distoratio[i] = C.float(v)
		}
		par.cp_fixed_quality = 1
	}
	if o.NResolutions != 0 {
		par.numresolution = C.int(o.NResolutions)
	}

	if C.opj_setup_encoder(c.codec, &par, c.image) == 0 {
		return
	}
	if C.opj_start_compress(c.codec, c.image, c.stream) == 0 {
		return
	}
	if C.opj_encode(c.codec, c.stream) == 0 || C.opj_end_compress(c.codec, c.stream) == 0 {
		return
	}
	return true
}