package jpeg

/*
#include <stdio.h>
#include <jpeglib.h>
#include <stdlib.h>

static void encodeYCbCr(j_compress_ptr c, JSAMPROW y, JSAMPROW cb, JSAMPROW cr, int ys, int cs) {
	JSAMPROW planes[3][DCTSIZE * c->max_v_samp_factor];
	JSAMPARRAY p[] = { &planes[0][0], &planes[1][0], &planes[2][0] };
	for (int r = 0; r < c->image_height; ) {
		for (int i = 0; i < DCTSIZE * c->max_v_samp_factor; i++) {
			planes[0][i] = y + ys * (r+i);
			planes[1][i] = cb + cs * (r/c->max_v_samp_factor+i);
			planes[2][i] = cr + cs * (r/c->max_v_samp_factor+i);
		}
		r += jpeg_write_raw_data(c, p, DCTSIZE * c->max_v_samp_factor);
	}
}

static void encodeScan(j_compress_ptr cinfo, unsigned char *buf, int stride) {
	unsigned char *outbufs[cinfo->image_height];
	for (int i = 0; i < cinfo->image_height; i++) {
		outbufs[i] = buf;
		buf += stride;
	}
	for (int i = 0; i < cinfo->image_height; )
		i += jpeg_write_scanlines(cinfo, &outbufs[i], cinfo->image_height-i);
}

#ifndef JPEG_C_PARAM_SUPPORTED
// The header could be just wrong, so make em weak.
typedef int J_BOOLEAN_PARAM;
typedef int J_FLOAT_PARAM;
typedef int J_INT_PARAM;
void  __attribute__((weak)) jpeg_c_set_bool_param(j_compress_ptr cinfo, J_BOOLEAN_PARAM param, boolean value) { }
void  __attribute__((weak)) jpeg_c_set_float_param(j_compress_ptr cinfo, J_FLOAT_PARAM param, float value) { }
void  __attribute__((weak)) jpeg_c_set_int_param(j_compress_ptr cinfo, J_INT_PARAM param, int value) { }
#endif

*/
import "C"
import (
	"github.com/ezdiy/image/util"
	"image"
	"image/color"
	"io"
	"runtime"
	"sync"
	"unsafe"
)

var encoderPool = &sync.Pool{}

type encoder struct {
	cInfo    C.struct_jpeg_compress_struct // must be first
	writeBuf [bufferSize]byte
	encoderTransient
}

type encoderTransient struct {
	NBWritten int
	*Options
	io.Writer // The underlying data stream
}

func (w *encoder) cleanup(abort bool) {
	if w.cInfo.err == nil {
		return // killed by errorPanic
	}
	if w.Options != nil && w.Options.NBWritten != nil {
		*w.Options.NBWritten += w.NBWritten
	}
	w.NBWritten = 0
	w.encoderTransient = encoderTransient{}
	if abort {
		C.jpeg_abort_compress(&w.cInfo)
	}
	encoderPool.Put(w)
}

func (w *encoder) setBuffer(n int) {
	w.NBWritten += n
	w.cInfo.dest.free_in_buffer = bufferSize
	w.cInfo.dest.next_output_byte = (*C.uchar)(unsafe.Pointer(&w.writeBuf[0]))
}

func Encode(o io.Writer, img image.Image, opt *Options) error {
	// Alloc from pool
	w, ok := encoderPool.Get().(*encoder)
	if !ok {
		w = &encoder{}
		cb := makeCallbacks()
		ci := &w.cInfo
		ci.err = &cb.err
		C.jpeg_CreateCompress(&w.cInfo, C.JPEG_LIB_VERSION, C.sizeof_struct_jpeg_compress_struct)
		ci.dest = &cb.dst
		runtime.SetFinalizer(w, func(r *encoder) {
			C.free(unsafe.Pointer(r.cInfo.err))
			C.jpeg_destroy_compress(&r.cInfo)
		})
	}
	w.setBuffer(0)
	w.Writer = o

	if opt == nil {
		opt = &DefaultEncoderOptions
	}

	// Setup image
	ci := &w.cInfo
	ci.image_width = C.JDIMENSION(img.Bounds().Dx())
	ci.image_height = C.JDIMENSION(img.Bounds().Dy())

	cm := img.ColorModel()
	if cm == color.YCbCrModel {
		im := img.(*image.YCbCr)
		ci.input_components = 3
		ci.in_color_space = C.JCS_YCbCr
		w.parseOptions(opt)
		c := (*[3]C.jpeg_component_info)(unsafe.Pointer(ci.comp_info))
		yv, yh := util.SSR2VHDiv(im.SubsampleRatio)
		c[0].v_samp_factor, c[0].h_samp_factor = C.int(yv), C.int(yh)
		c[1].v_samp_factor, c[1].h_samp_factor = 1, 1
		c[2].v_samp_factor, c[2].h_samp_factor = 1, 1
		ci.raw_data_in = C.TRUE
		C.jpeg_start_compress(&w.cInfo, C.TRUE)
		C.encodeYCbCr(&w.cInfo,
			(*C.uchar)(unsafe.Pointer(&im.Y[0])),
			(*C.uchar)(unsafe.Pointer(&im.Cb[0])),
			(*C.uchar)(unsafe.Pointer(&im.Cr[0])),
			C.int(im.YStride),
			C.int(im.CStride))
	} else {
		switch cm {
		case color.RGBAModel:
		case color.NRGBAModel:
			ci.input_components = 4
			ci.in_color_space = C.JCS_EXT_RGBA
		case color.GrayModel:
			ci.input_components = 1
			ci.in_color_space = C.JCS_GRAYSCALE
		case color.CMYKModel:
			ci.input_components = 4
			ci.in_color_space = C.JCS_CMYK
		}
		w.parseOptions(opt)
		if opt.Gamma != 0 {
			ci.input_gamma = C.double(opt.Gamma)
		}
		ci.data_precision = 8
		C.jpeg_start_compress(&w.cInfo, C.TRUE)
		pix, stride := util.GetPixStride(img)
		C.encodeScan(&w.cInfo, (*C.uchar)(&pix[0]), C.int(stride))
	}

	C.jpeg_finish_compress(&w.cInfo)
	w.cleanup(false)
	return nil
}

func (w *encoder) parseOptions(opt *Options) {
	ci := &w.cInfo
	ext := opt.Ext
	if ext == nil {
		ext = ExtOptions{}
	}

	// Set profile first
	if prof, ok := ext[OptCompressProfile]; ok {
		w.setParam(OptCompressProfile, prof)
	}

	// Apply defaults from profile
	C.jpeg_set_defaults(&w.cInfo)

	// Not progressive, so disable scans
	if opt.NoProgressive {
		w.setParam(OptScans, false)
	}

	// Now apply GUID params
	for k, v := range ext {
		w.setParam(k, v)
	}

	// If 0, defaults to 75
	if opt.Quality > 0 {
		C.jpeg_set_quality(&w.cInfo, C.int(opt.Quality), bool2c(opt.ForceBaseline))
	}

	// The rest of the options are final override
	ci.dct_method = C.J_DCT_METHOD(opt.DCTMethod)
	ci.smoothing_factor = C.int(opt.SmoothingFactor)
	ci.optimize_coding = bool2c(!opt.FastHufftab)
	ci.do_fancy_downsampling = bool2c(!opt.NoFancyDownsampling)
	ci.arith_code = bool2c(opt.ArithmeticCoding)

	// Copy quant tables if specified
	if opt.QuantTables != nil {
		var tables [2][64]C.uint
		for i := 0; i < 2; i++ {
			for j := 0; j < 64; j++ {
				tables[i][j] = C.uint(opt.QuantTables[i][j])
			}
		}
		C.jpeg_add_quant_table(&w.cInfo, 0, &tables[0][0], C.int(opt.Quality), bool2c(opt.ForceBaseline))
		C.jpeg_add_quant_table(&w.cInfo, 1, &tables[1][0], C.int(opt.Quality), bool2c(opt.ForceBaseline))
	}

	// TODO: multi-scan scripts
}

func (w *encoder) setParam(n uint64, v interface{}) {
	g := uint32(n)
	switch n >> 32 {
	case 0:
		b, ok := v.(byte)
		if !ok && v.(bool) {
			b = 1
		}
		C.jpeg_c_set_bool_param(&w.cInfo, C.J_BOOLEAN_PARAM(g), C.uchar(b))
	case 1:
		f, ok := v.(float32)
		if !ok {
			f = float32(v.(float64))
		}
		C.jpeg_c_set_float_param(&w.cInfo, C.J_FLOAT_PARAM(g), C.float(f))
	case 2:
		C.jpeg_c_set_int_param(&w.cInfo, C.J_INT_PARAM(g), C.int(v.(int)))
	}
}
