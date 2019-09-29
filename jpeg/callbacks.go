package jpeg

/*
#include <stdio.h>
#include <jpeglib.h>
extern char *format_message(j_common_ptr cinfo);
*/
import "C"
import (
	"io"
	"unsafe"
)

//export skipInputData
func skipInputData(self unsafe.Pointer, n C.long) {
	r := (*decoder)(self)
	skip := int(n)
	inBuf := int(r.dInfo.src.bytes_in_buffer)
	next := uintptr(unsafe.Pointer(r.dInfo.src.next_input_byte))
	// There's still more data in the input buffer than we want to skip
	if inBuf > skip {
		inBuf -= skip
		next += uintptr(skip)
	} else {
		skip -= inBuf
		// nothing in buffer left, roll over the input
		for skip > 0 {
			ts := skip
			if ts > bufferSize {
				ts = bufferSize
			}
			got, err := r.Read(r.readBuf[:ts])
			skip -= got
			if got == 0 || err != nil {
				panic(err)
			}
		}
		// reset buffer
		next = uintptr(unsafe.Pointer(&r.readBuf[0]))
		inBuf = 0
	}
	r.dInfo.src.next_input_byte = (*C.JOCTET)(unsafe.Pointer(next))
	r.dInfo.src.bytes_in_buffer = C.size_t(inBuf)
}

//export fillInputBuffer
func fillInputBuffer(self unsafe.Pointer) bool {
	r := (*decoder)(self)
	got, err := r.Read(r.readBuf[:])
	if got == 0 {
		if r.NBRead == 0 {
			if err == nil {
				err = io.EOF
			}
			panic(err)
		}
		// EOI
		r.readBuf[0] = 255
		r.readBuf[1] = 9
		got = 2
	}
	r.setBuffer(got)
	return true
}

//export outputBuffer
func outputBuffer(self unsafe.Pointer) bool {
	w := (*encoder)(self)
	inBuf := bufferSize - int(w.cInfo.dest.free_in_buffer)
	if inBuf > 0 {
		wrote, err := w.Write(w.writeBuf[:inBuf])
		if err != nil {
			panic(err)
		}
		if wrote < inBuf {
			throw("truncated write, %d < %d", wrote, inBuf)
		}
	}
	w.setBuffer(inBuf)
	return true
}

//export errorPanic
func errorPanic(msg *C.char) {
	throw("error_exit(): %s", C.GoString(msg))
}

type callbacks struct {
	err C.struct_jpeg_error_mgr // must be first
	src C.struct_jpeg_source_mgr
	dst C.struct_jpeg_destination_mgr
}
