package ta

/*
#cgo CFLAGS: -I${SRCDIR}/include
#include "ta_libc.h"
#include "ta_func.h"
*/
import "C"
import (
	"math"
	"unsafe"
)

//go:generate go run ./cmd/gen v0.6.4
//go:generate uv run ./scripts/gen_fixtures.py

func init() {
	if rc := C.TA_Initialize(); rc != C.TA_SUCCESS {
		panic(&TALibError{RetCode: int(rc), Message: retCodeMessage(int(rc))})
	}
}

// reuseOrAlloc returns outBuf[:n] if cap(outBuf) >= n, else allocates a new slice.
func reuseOrAlloc(outBuf []float64, n int) []float64 {
	if cap(outBuf) >= n {
		return outBuf[:n]
	}
	return make([]float64, n)
}

// fillNaN fills s with NaN values.
func fillNaN(s []float64) {
	nan := math.NaN()
	for i := range s {
		s[i] = nan
	}
}

// inPtr returns a C pointer to the first element of a float64 slice.
func inPtr(s []float64) *C.double {
	return (*C.double)(unsafe.Pointer(&s[0]))
}

// inPtrInt returns a C pointer to the first element of an int32 slice.
func inPtrInt(s []int32) *C.int {
	return (*C.int)(unsafe.Pointer(&s[0]))
}

// reuseOrAllocInt32 is reuseOrAlloc for int32 slices.
func reuseOrAllocInt32(outBuf []int32, n int) []int32 {
	if cap(outBuf) >= n {
		return outBuf[:n]
	}
	return make([]int32, n)
}

// fillNaNInt32 zeroes s (int32 has no NaN; use 0 as sentinel).
func fillNaNInt32(s []int32) {
	for i := range s {
		s[i] = 0
	}
}

func checkRC(rc C.TA_RetCode) {
	if rc != C.TA_SUCCESS {
		code := int(rc)
		panic(&TALibError{RetCode: code, Message: retCodeMessage(code)})
	}
}
