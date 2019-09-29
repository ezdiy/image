package jpeg

//#include <jpeglib.h>
import "C"
import (
	"io"
	"unsafe"
)

//export skipInputData
func skipInputData(self unsafe.Pointer, n C.long) {
	r := (*reader)(self)
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
	r := (*reader)(self)
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
	r.dInfo.src.next_input_byte = (*C.JOCTET)(&r.readBuf[0])
	r.dInfo.src.bytes_in_buffer = (C.size_t)(got)
	r.NBRead += got
	return true
}


//export outputBuffer
func outputBuffer(self unsafe.Pointer) bool {
	w := (*writer)(self)
	inBuf := bufferSize - int(w.cInfo.dst.free_in_buffer)
	if inBuf > 0 {
		wrote, err := w.Write(w.writeBuf[:inBuf])
		if err != nil {
			panic(err)
		}
		if wrote < inBuf {
			throw("truncated write, %d < %d", wrote, inBuf)
		}
	}
	w.NBWritten += inBuf
	w.cInfo.dst.free_in_buffer = bufferSize
	w.cInfo.dst.next_output_byte = &w.cInfo.writeBuf[0]
	return true
}

type callbacks struct {
	err C.struct_jpeg_error_mgr
	src C.struct_jpeg_source_mgr
	dst C.struct_jpeg_destination_mgr
}

// a bit of gymnastics as go doesn't like seeing its own pointers there
func makeCallbacks(wb *byte) *callbacks {
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
			free_in_buffer:      bufferSize,
			next_output_byte:    (*C.JOCTET)(unsafe.Pointer(wb)),
		},
	}
	C.jpeg_std_error(&cb.err)
	return cb
}
