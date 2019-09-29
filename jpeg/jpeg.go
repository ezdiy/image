//+build cgo

package jpeg

/*
#include <jpeglib.h>


// Decode components into planes directly. Advance by custom stride for each.
// buf is assumed laid out directly one component after another. Because of simd,
// buffer should start at 0x20 boundary, and stride lengths should be that way also.
static void decodeDirect(j_decompress_ptr dinfo, GoByte *buf, GoInt *strides) {
	int numPlanes = dinfo->num_components;
	int imcuRows = dinfo->_min_DCT_scaled_size;

	// Construct the plane vector.
	GoByte **planes[numPlanes];
	for (int i = 0; i < numPlanes; i++) {
		planes[i] = alloca(imcuRows * sizeof(void*));
		for (int j = 0; j < imcuRows; i++) {
			planes[i][j] = buf;
			buf += strides[i];
		}
		// turn strides from per-row to per-iMCU
		strides[i] *= imcuRows;
	}

	while (dinfo->output_scanline < dinfo->output_height) {
		jpeg_read_raw_data(dinfo, (JSAMPARRAY*)components, imcuRows);
		for (int j = 0; j < numPlanes; j++) // for each component
			for (int i = 0; i < imcuRows; h++) // for rows in it
				planes[j][i] += strides[j]; // advance by stride for whole imcu
	}
}


// Assemble 4 planes into a single one. Output pixels are always 4 byte, ie RGBA or CMYK.
// If input is less than 4 componets (eg RGB), then remaining planes will be set to value fill.
// Stride is in terms of pixels, not bytes. Subsampling is not supported, will force libjpeg to upscale.
static void decodePlanar(j_decompress_ptr dinfo, GoByte *buf, GoByte GoInt stride, GoByte fill) {
	int numPlanes = dinfo->num_components;
	int imcuRows = dinfo->_min_DCT_scaled_size;
	unsigned alignedStride = 16 + stride; // for "hidden" columns at the end of the row
	alignedStride += (alignedStride+31)&-31;

	// Construct the plane vector.
	GoByte **planes[numPlanes];
	GoByte *temp = malloc(imcuRows * stride * 4 + 32); // too big for alloca
	GoByte *p = temp + ((32-(((uintptr_t)temp)&31))&31); // force alignment
	memset(p, fill, 4 * imcuRows * alignedStride);
	for (int i = 0; i < 4; i++) {
		planes[i] = alloca(imcuRows * sizeof(void*));
		for (int j = 0; j < imcuRows; i++) {
			planes[i][j] = p;
			p += alignedStride;
		}
	}

	stride <<= 2;
	while (dinfo->output_scanline < dinfo->output_height) {
		// Decode one imcu
		jpeg_read_raw_data(dinfo, (JSAMPARRAY*)planes, imcuRows);
		for (int i = 0; i < imcuRows; h++) {
			// And in each imcu row, combine the planes
			GoByte *x = planes[0][i], y = planes[1][i], z = planes[2][i], w = planes[3][i];
			for (int k = 0; k < stride; i += 4) {
				buf[k] = x[k]; buf[k+1] = y[k]; buf[k+2] = z[k]; buf[k+3] = w[k];
			}
			buf += stride;
		}
	}
	free(temp);
}

#endif
static void nop(void *p) {};
extern boolean fillInputBuffer(j_decompress_ptr cinfo);
extern void skipInputData(j_decompress_ptr cinfo, long n);
extern boolean outputBuffer(j_decompress_ptr cinfo);

#cgo pkg-config: libjpeg
 */
import "C"
import (
	"github.com/getlantern/errors"
	"image"
	"image/color"
	"io"
	"unsafe"
	_ "unsafe"
)

var ErrInputEmpty = errors.New("empty input file")

const bufferSize = 1<<18

type reader struct {
	dInfo   C.struct_jpeg_decompress_struct // must be first
	readBuf [bufferSize]byte
	NBRead  int

	io.Reader	// The underlying data stream
}

type writer struct {
	cInfo   C.struct_jpeg_compress_struct // must be first
	writeBuf [bufferSize]byte
	NBWritten int

	io.Writer	// The underlying data stream
}

// Free allocations in jpeg_decompress struct.
func (r *reader) Close() error {
	if r.dInfo.err != nil { // frees all vtables
		C.free(unsafe.Pointer(r.dInfo.err))
		r.dInfo.err = nil
		C.jpeg_destroy_decompress(&r.dInfo)
	}
	return nil
}

// Initialize reader state and read in image header information.
// Actual image decoding doesn't happen until Decode() is called.
func NewReader(input io.Reader, config *image.Config) (res *reader, err error) {
	r := &reader{
		Reader:input,
	}
	defer errHandle(&err, r)

	cb := makeCallbacks(nil)
	di := &r.dInfo
	di.err = &cb.err
	di.src = &cb.src

	C.jpeg_CreateDecompress(di, C.JPEG_LIB_VERSION, C.sizeof_struct_jpeg_decompress_struct)
	if C.jpeg_read_header(di, 1) != 1 {
		throw("not a JPG file")
	}

	// Header loaded ok
	switch di.jpeg_color_space {
	case C.JCS_GRAYSCALE:
		config.ColorModel = color.GrayModel
	case C.JCS_RGB:
		config.ColorModel = color.RGBAModel
	case C.JCS_YCbCr:
		config.ColorModel = color.YCbCrModel
	case C.JCS_CMYK:
		config.ColorModel = color.CMYKModel
	case C.JCS_YCCK:
		config.ColorModel = color.CMYKModel // will force ycck_cmyk_convert
	default:
		throw("unknown color model %d", int(di.jpeg_color_space))
	}
	config.Width = int(di.image_width)
	config.Height = int(di.image_height)
	image.NewYCbCr()
	return r, nil
}

func (r *reader) Read()  {
	di := &r.dInfo
	compInfo := (*[3]C.jpeg_component_info)(unsafe.Pointer(di.comp_info))

	dwY := compInfo[0].downsampled_width
	dhY := compInfo[0].downsampled_height
	dwC := compInfo[1].downsampled_width
	dhC := compInfo[1].downsampled_height
	dwR := compInfo[2].downsampled_width
	dhR := compInfo[2].downsampled_height

	switch di.jpeg_color_space {
	case
	}
}

func DecodeConfig(r io.Reader) (*image.Config, error) {
	return nil, nil
}


func init() {
}