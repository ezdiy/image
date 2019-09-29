package jpeg

import (
	"errors"
	"fmt"
	"io"
)

// libjpeg doesn't support normal error propagation from callbacks

func errHandle(err *error, closer io.Closer) {
	r := recover()
	if r == nil {
		return
	}
	*err = r.(error)
	// not a thrown error, panic for real now
	if *err == nil {
		panic(r)
	}
	if closer != nil {
		_ = closer.Close()
	}
}

func throw(s string, arg ...interface{}) {
	panic(errors.New(fmt.Sprintf(s, arg...)))
}

