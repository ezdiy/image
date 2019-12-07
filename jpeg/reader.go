//+build cgo

package jpeg

/*
#include <stdio.h>
#include <stdlib.h>
#include <jpeglib.h>
// Gray and YCbCr decode planes directly, advancing by subsample scaled stride for each.
// buf will be laid out one component plane after another. Initial strides are of downsampled_width,
// aligned to 32 byte due to SIMD. Buffer should also start at 32 aligne address as well.
static int decodeRaw(j_decompress_ptr dinfo, unsigned char *buf, int *strides) {
#define imcuRows(plane) (DCTSIZE * dinfo->comp_info[plane].v_samp_factor)
	int numPlanes = dinfo->num_components;

	// Construct the initial row vector for each plane.
	unsigned char **planes[numPlanes];
	for (int i = 0; i < numPlanes; i++) {
		int stride = strides[i];
		int pRows = imcuRows(i);
		planes[i] = alloca(pRows * sizeof(void*));
		for (int j = 0; j < pRows; j++) {
			planes[i][j] = buf;
			buf += stride;
		}
		// Skip over to next plane. -1 because loop above consumed one imcu.
		buf += (strides[i] *= pRows) * (dinfo->total_iMCU_rows-1);
	}

	int nImcu = 0;
	int maxRows = dinfo->max_v_samp_factor * DCTSIZE;
	while (dinfo->output_scanline < dinfo->output_height) {
		int got = jpeg_read_raw_data(dinfo, (JSAMPARRAY*)planes, maxRows);
		for (int j = 0; j < numPlanes; j++) { // for each component
			int pRows = imcuRows(j);
			for (int i = 0; i < pRows; i++) // for rows in it
				planes[j][i] += strides[j]; // advance by stride of one imcu for this plane
		}
		nImcu++;
	}
	return nImcu;
}

// Decode scanlines, for use with non-planar formats and weird subsampling ratios.
static int decodeScan(j_decompress_ptr dinfo, unsigned char *buf, int stride) {
	unsigned char *outbufs[dinfo->rec_outbuf_height];
	int outlen, nLines = 0;
	while ((outlen = dinfo->output_height - dinfo->output_scanline) > 0) {
		if (outlen > dinfo->rec_outbuf_height) {
			outlen = dinfo->rec_outbuf_height;
		}
		for (int i = 0; i < outlen; i++) {
			outbufs[i] = buf;
			buf += stride;
		}
		nLines += jpeg_read_scanlines(dinfo, (JSAMPROW *)outbufs, outlen);
	}
	return nLines;
}

*/
import "C"
import (
	"github.com/ezdiy/image/util"
	"image"
	"image/color"
	"io"
	"log"
	"runtime"
	"sync"
	"unsafe"
)

// Holds all decoder state.
var decoderPool = &sync.Pool{}

type decoder struct {
	dInfo            C.struct_jpeg_decompress_struct // must be first
	readBuf          [bufferSize]byte
	decoderTransient // reset on each pool reuse
}

type decoderTransient struct {
	NBRead          int // Number of bytes read from the stream
	io.Reader           // The underlying data stream
	*DecoderOptions     // Options for decoder
}

// Clean up the decoder state for next reuse.
func (r *decoder) cleanup(abort bool) {
	if r.dInfo.err == nil {
		return // killed by errorPanic
	}
	if r.DecoderOptions != nil && r.DecoderOptions.NBRead != nil {
		*r.DecoderOptions.NBRead += r.NBRead
	}
	r.NBRead = 0
	r.decoderTransient = decoderTransient{}
	if abort {
		C.jpeg_abort_decompress(&r.dInfo)
	}
	decoderPool.Put(r)
}

func (r *decoder) setBuffer(n int) {
	r.dInfo.src.next_input_byte = (*C.JOCTET)(&r.readBuf[0])
	r.dInfo.src.bytes_in_buffer = (C.size_t)(n)
	r.NBRead += n
}

// Decode an image with given options.
func DecodeImage(input io.Reader, opt *DecoderOptions) (img image.Image, err error) {
	// Decoders are reused, and freed only under memory pressure.
	r, ok := decoderPool.Get().(*decoder)
	if !ok {
		r = &decoder{}
		cb := makeCallbacks()
		di := &r.dInfo
		// If it errorPanics at this point, defer won't catch it, but it will go upstream.
		di.err = &cb.err
		C.jpeg_CreateDecompress(&r.dInfo, C.JPEG_LIB_VERSION, C.sizeof_struct_jpeg_decompress_struct)
		di.src = &cb.src
		runtime.SetFinalizer(r, func(r *decoder) {
			C.free(unsafe.Pointer(r.dInfo.err))
			C.jpeg_destroy_decompress(&r.dInfo)
		})
	}

	r.setBuffer(0)
	r.Reader = input
	defer errHandle(&err, r)

	di := &r.dInfo
	if opt == nil {
		opt = &DefaultDecoderOptions
	}
	r.DecoderOptions = opt

	if C.jpeg_read_header(&r.dInfo, 1) != 1 {
		throw("not a JPG file")
	}

	// Config requested
	config := opt.Config
	if config != nil {
		switch di.jpeg_color_space {
		case C.JCS_GRAYSCALE:
			config.ColorModel = color.GrayModel
		case C.JCS_YCbCr:
			config.ColorModel = color.YCbCrModel
		case C.JCS_RGB:
			config.ColorModel = color.NRGBAModel
		case C.JCS_CMYK:
			config.ColorModel = color.CMYKModel
		case C.JCS_YCCK:
			config.ColorModel = color.CMYKModel
		default:
			throw("unknown color model %d", int(di.jpeg_color_space))
		}
		config.Width = int(di.image_width)
		config.Height = int(di.image_height)
		// No decoding requested
		r.cleanup(true)
		return nil, nil
	}

	// Parse options
	di.dct_method = C.J_DCT_METHOD(opt.DCTMethod)
	di.do_fancy_upsampling = bool2c(!opt.NoFancyUpsampling)
	di.do_block_smoothing = bool2c(!opt.NoBlockSmoothing)

	// Heuristics to choose decoder based on decoder options. Whenever raw
	// decoder falls through, we attempt to use scanline one (if colorspace permits).
	switch di.jpeg_color_space {
	case C.JCS_GRAYSCALE:
		if r.HasModel(color.GrayModel) && !r.NoRawDecodingGray {
			img = r.tryGray()
		}
		if img == nil {
			if img = r.tryModel(color.GrayModel, C.JCS_GRAYSCALE); img == nil {
				if img = r.tryModel(color.NRGBAModel, C.JCS_EXT_RGBA); img == nil {
					img = r.tryModel(color.RGBAModel, C.JCS_EXT_RGBA)
				}
			}
		}
	case C.JCS_YCbCr:
		if r.HasModel(color.YCbCrModel) {
			img = r.tryYCbCr()
		}
		if img == nil {
			if img = r.tryModel(color.NRGBAModel, C.JCS_EXT_RGBA); img == nil {
				if img = r.tryModel(color.RGBAModel, C.JCS_EXT_RGBA); img == nil {
					img = r.tryModel(color.GrayModel, C.JCS_GRAYSCALE)
				}
			}
		}
	case C.JCS_RGB:
		if img = r.tryModel(color.NRGBAModel, C.JCS_EXT_RGBA); img == nil {
			if img = r.tryModel(color.RGBAModel, C.JCS_EXT_RGBA); img == nil {
				img = r.tryModel(color.GrayModel, C.JCS_GRAYSCALE)
			}
		}
	case C.JCS_CMYK:
	case C.JCS_YCCK:
		img = r.tryModel(color.CMYKModel, C.JCS_CMYK)
	default:
		throw("unknown color model %d", int(di.jpeg_color_space))
	}
	C.jpeg_finish_decompress(&r.dInfo)
	r.cleanup(false)
	return img, nil
}

// Compatible API to read color model and dimensions only.
func DecodeConfig(r io.Reader) (cfg image.Config, err error) {
	_, err = DecodeImage(r, &DecoderOptions{Config: &cfg})
	return
}

// Compatible API, decodes with default options.
func Decode(i io.Reader) (img image.Image, err error) {
	return DecodeImage(i, nil)
}

// Attempt a raw decode given a list of strides for each component.
func (r *decoder) rawDecode(buf []byte, strides []int32) {
	r.dInfo.raw_data_out = 1
	C.jpeg_start_decompress(&r.dInfo)

	// Decompress
	iMCUs := int32(C.decodeRaw(&r.dInfo, (*C.uchar)(unsafe.Pointer(&buf[0])), (*C.int)(unsafe.Pointer(&strides[0]))))

	// Check that we didn't screw up the dimensions calc
	var got int
	for _, v := range strides {
		got += int(v * iMCUs)
	}
	if got != len(buf) {
		log.Fatalf("misaligned decode %d != %d. process memory corrupted!\n", got, len(buf))
	}
}

// Attempt to raw decode grayscale picture.
func (r *decoder) tryGray() (img image.Image) {
	di := &r.dInfo
	ci := (*[3]C.jpeg_component_info)(unsafe.Pointer(di.comp_info))

	// Must have 1 component and no sub-sampling
	if di.num_components != 1 || ci[0].v_samp_factor != 1 || ci[0].h_samp_factor != 1 {
		return
	}

	Stride := alignto(int(ci[0].downsampled_width), 32)
	Height := alignto(int(ci[0].downsampled_height), dctSize)

	// Allocate the picture
	buf := alignedBuf(Stride * Height)
	img = &image.Gray{
		Pix:    buf,
		Stride: Stride,
		Rect:   image.Rect(0, 0, int(di.image_width), int(di.image_height)),
	}

	r.rawDecode(buf, []int32{int32(Stride)})
	return
}

// Attempt to decode YCbCr picture.
func (r *decoder) tryYCbCr() (img image.Image) {
	di := &r.dInfo
	ci := (*[3]C.jpeg_component_info)(unsafe.Pointer(di.comp_info))

	// Must have 3 components
	if di.num_components != 3 {
		return
	}

	// Must fit the MCU membership array.
	for i := 0; i < 3; i++ {
		// Ancient PSP9 and PS3 files with Y=4x4, C=2x2 for example.
		// Raise D_MAX_BLOCKS_IN_MCU in jconfig.h and recompile jpeglib if you need that.
		mcublks := uintptr(di.blocks_in_MCU + ci[i].h_samp_factor*ci[i].v_samp_factor)
		if mcublks > unsafe.Sizeof(di.MCU_membership)/C.sizeof_int {
			return
		}
	}

	yv, yh := ci[0].v_samp_factor, ci[0].h_samp_factor
	cv, ch := ci[1].v_samp_factor, ci[1].h_samp_factor

	// Sampling for both chroma must be same
	if ci[2].v_samp_factor != cv || ci[2].h_samp_factor != ch {
		return
	}

	// Luma must have (1 or more times) multiple of chroma samples more
	// TODO: what about not-of-2 powers?
	if yv%cv != 0 || yh%ch != 0 {
		return
	}

	// Scale down to ratio
	yv /= cv
	yh /= ch

	// Must be of known and whitelisted subsampling ratio
	ratio := util.VHDiv2SSR(int(yv), int(yh))
	if !r.HasSSR(ratio) {
		return
	}

	// Looks alright, compute sample dimensions
	YStride := alignto(int(ci[0].downsampled_width), 32)
	YHeight := alignto(int(ci[0].downsampled_height), int(ci[0].v_samp_factor*dctSize))
	CStride := alignto(int(ci[1].downsampled_width), 32)
	CHeight := alignto(int(ci[1].downsampled_height), int(ci[1].v_samp_factor*dctSize))
	YSize := YStride * YHeight
	CSize := CStride * CHeight
	bufSize := YSize + CSize*2

	// Allocate the picture
	buf := alignedBuf(bufSize)
	img = &image.YCbCr{
		Y:              buf[:YSize],
		Cb:             buf[YSize:][:CSize],
		Cr:             buf[YSize+CSize:][:CSize],
		SubsampleRatio: ratio,
		YStride:        YStride,
		CStride:        CStride,
		Rect:           image.Rect(0, 0, int(di.image_width), int(di.image_height)),
	}

	r.rawDecode(buf, []int32{int32(YStride), int32(CStride), int32(CStride)})
	return
}

// Decode using a scan line decoder with post-processing into target colorspace.
// This is slower and doesn't preserve source data in original form, but
// also much more robust for exotic files which can't be handled by the fairly naive
// raw decoder.
func (r *decoder) tryModel(model color.Model, cs C.J_COLOR_SPACE) (img image.Image) {
	// Is this colorspace enabled at all?
	if !r.HasModel(model) {
		return
	}

	// Set up output
	di := &r.dInfo
	di.out_color_space = cs
	C.jpeg_start_decompress(&r.dInfo)

	// Create image
	img = util.NewImage(model, image.Rect(0, 0, int(di.image_width), int(di.image_height)))
	pix, stride := util.GetPixStride(img)
	if pix == nil {
		return nil
	}

	// Decode all rows
	C.decodeScan(&r.dInfo, (*C.uchar)(unsafe.Pointer(&pix[0])), C.int(stride))
	return
}

func init() {
	image.RegisterFormat("jpeg", "\xff\xd8", Decode, DecodeConfig)
}
