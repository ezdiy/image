//+build cgo

package jpeg

/*
#cgo CFLAGS: -O2 -fomit-frame-pointer
#cgo windows AND amd64 CFLAGS: -Iinclude
#cgo !windows OR !amd64 pkg-config: libjpeg
#include <stdio.h>
#include <jpeglib.h>
void nop(void *p) {};
extern boolean fillInputBuffer(j_decompress_ptr cinfo);
extern void skipInputData(j_decompress_ptr cinfo, long n);
extern boolean outputBuffer(j_decompress_ptr cinfo);
extern void errorPanic(const char *msg);

void errorHandler(j_common_ptr cptr) {
	char buf[JMSG_LENGTH_MAX];
	(*cptr->err->format_message)(cptr, buf);
	jpeg_destroy(cptr);
	free(cptr->err);
	cptr->err = NULL;
	errorPanic(buf);
}


*/
import "C"
import (
	"fmt"
	"errors"
	"strings"
	"unsafe"
)

const (
	dctSize    = 8
	bufferSize = 1 << 18
	errPrefix  = "libjpeg: "
)

type cleanup interface{ cleanup(abort bool) }

// libjpeg doesn't support normal error propagation from callbacks,
// so we abuse panic for a bit.
func errHandle(err *error, closer cleanup) {
	r := recover()
	if r == nil {
		return
	}
	msg, ok := r.(string)
	// not a thrown string, or not our prefix, panic for real now
	if !ok || !strings.HasPrefix(msg, errPrefix) {
		panic(r)
	}
	if err != nil {
		*err = errors.New(msg)
	}
	if closer != nil {
		closer.cleanup(true)
	}
}

func throw(s string, arg ...interface{}) {
	panic(fmt.Sprintf(errPrefix+s, arg...))
}

func alignto(n, a int) int {
	return (n + a - 1) & -a
}

// a bit of gymnastics as go doesn't like seeing its own pointers there
func makeCallbacks() *callbacks {
	cb := (*callbacks)(unsafe.Pointer(C.malloc(C.size_t(unsafe.Sizeof(callbacks{})))))
	*cb = callbacks{
		src: C.struct_jpeg_source_mgr{
			init_source:       (*[0]byte)(C.nop),
			term_source:       (*[0]byte)(C.nop),
			resync_to_restart: (*[0]byte)(C.jpeg_resync_to_restart),
			fill_input_buffer: (*[0]byte)(C.fillInputBuffer),
			skip_input_data:   (*[0]byte)(C.skipInputData),
		},
		dst: C.struct_jpeg_destination_mgr{
			init_destination:    (*[0]byte)(C.nop),
			empty_output_buffer: (*[0]byte)(C.outputBuffer),
			term_destination:    (*[0]byte)(C.outputBuffer),
		},
	}
	C.jpeg_std_error(&cb.err)
	cb.err.error_exit = (*[0]byte)(C.errorHandler)
	return cb
}

func alignedBuf(n int) []byte {
	buf := make([]byte, n)
	return buf[(32-(uintptr(unsafe.Pointer(&buf[0]))&31))&31:][:n]
}

func bool2c(b bool) (ret C.uchar) {
	if b {
		ret = 1
	}
	return
}
