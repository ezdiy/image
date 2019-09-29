package openjpeg

// #include <string.h>
// #include <openjpeg-2.3/openjpeg.h>
import "C"
import (
	"errors"
	"io"
	"io/ioutil"
	"math"
	"unsafe"
)


//export writeFunc
func writeFunc(b unsafe.Pointer, count int, p unsafe.Pointer) (ret int) {
	buf := ((*[math.MaxInt32]byte)(b))[:count]
	c := (*codec)(p)
	ret, err := c.Write(buf[:])
	if ret == 0 && err != nil {
		return -1
	}
	return ret
}

//export readFunc
func readFunc(b unsafe.Pointer, count int, p unsafe.Pointer) int {
	buf := ((*[math.MaxInt32]byte)(b))[:count]
	c := (*codec)(p)
	cp := 0
	if c.magicPos < magicLen {
		cp = copy(buf[:count], c.magic[c.magicPos:])
		c.magicPos += cp
		buf = buf[cp:]
		if c.magicPos < magicLen {
			return cp
		}
	}
	got, err := c.Read(buf)
	got += cp
	if err != nil && got == 0 {
		return -1
	}
	return got
}

//export skipFunc
func skipFunc(n int64, p unsafe.Pointer) (w int64) {
	c := (*codec)(p)
	if c.WriteSeeker != nil {
		c.Seek(n, io.SeekCurrent)
		return n
	}
	s, ok := c.Reader.(io.Seeker)
	if ok {
		_, err := s.Seek(n, io.SeekCurrent)
		if err != nil {
			return -1
		}
		return n
	}
	w, _ = io.CopyN(ioutil.Discard, c.Reader, n)
	return
}

//export seekFunc
func seekFunc(n int64, p unsafe.Pointer) (ok C.OPJ_BOOL) {
	c := (*codec)(p)
	_, err := c.Seek(n, io.SeekStart)
	if err != nil {
		return 0
	}
	return 1
}

//export msgFunc
func msgFunc(msg *C.char, p unsafe.Pointer) {
	*(*error)(p) = errors.New(C.GoString(msg))
}