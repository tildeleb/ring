// Copyright © 2016,2017,2019 Lawrence E. Bakst. All rights reserved.

// Experimental, investigation into a concurrent ring buffer, safe to call from goroutines.
// Uses overflow trick that I did not know about 10 years ago.
// Currently slower than using a channel by about 3X because of atomic instructions.
// Now only 2X slower.

package ring

import (
	_ "fmt"
	"runtime"
	"sync/atomic"
	"unsafe"
)

// Must be 64 bits in size
type State struct {
	pidx uint16 // put index
	gidx uint16 // get index
	pad  int32  // could change the above to 32 bit
}

// Stats unused because of performance
type Stats struct {
	Puts           int64
	Gets           int64
	PutsFull       int64
	GetsEmpty      int64
	SwapsSucceeded int64
	SwapsFailed    int64
	PutSpins       int64
	GetSpins       int64
}

type Config struct {
	mask  uint16
	shift uint8
	size  uint8
}

// Ring implements a ring buffer that stores ints.
type Ring struct {
	State
	Config
	Stats
	Values []int
}

// New creates a new ring buffer with 2^size entries.
func New(size int) (r *Ring) {
	r = &Ring{Config: Config{mask: uint16(1<<uint(size)) - 1, shift: uint8(size), size: uint8(size + 1)}}
	r.Values = make([]int, 1<<uint(size))
	return
}

// Put adds a value to the ring buffer. Returns full if there is no space.
// Perhaps Put should overwrite instead of returning full.
func (r *Ring) Put(value int) (full bool) {
	var ui uint64
	var re, ore State

	for {
		ui = atomic.LoadUint64((*uint64)(unsafe.Pointer(&r.State)))
		re = *(*State)(unsafe.Pointer(&ui))
		ore = re
		switch {
		case re.pidx&r.mask == re.gidx&r.mask && re.pidx>>r.shift != re.gidx>>r.shift:
			full = true
			runtime.Gosched()
			//r.PutsFull++
			return
		case re.pidx&r.mask == re.gidx&r.mask && re.pidx>>r.shift == re.gidx>>r.shift:
			fallthrough // empty
		default:
			r.Values[re.pidx&r.mask] = value
			re.pidx++
			if re.pidx > uint16(1<<r.size)-1 {
				re.pidx = 0
			}
			swapped := atomic.CompareAndSwapUint64((*uint64)(unsafe.Pointer(r)), *(*uint64)((unsafe.Pointer)(&ore)), *(*uint64)((unsafe.Pointer)(&re))) //store(r, &ore, &re)
			if swapped {
				//atomic.AddInt64(&r.SwapsSucceeded, 1)
				r.Puts++
				return
			}
			//atomic.AddInt64(&r.SwapsFailed, 1)
		}
		//r.PutSpins++
	}
}

// Get a value from the ring buffer. Returns empty if there are no values left.
func (r *Ring) Get() (value int, empty bool) {
	var ui uint64
	var re, ore State

	for {
		ui = atomic.LoadUint64((*uint64)(unsafe.Pointer(&r.State))) // atomically read the pointer
		re = *(*State)(unsafe.Pointer(&ui))                         // make two copies
		ore = re
		switch {
		case re.pidx&r.mask == re.gidx&r.mask && re.pidx>>r.shift == re.gidx>>r.shift:
			empty = true
			runtime.Gosched()
			//r.GetsEmpty++
			return
		case re.pidx&r.mask == re.gidx&r.mask && re.pidx>>r.shift != re.gidx>>r.shift:
			fallthrough // full
		default:
			v := r.Values[re.gidx&r.mask]
			re.gidx++
			if re.gidx > uint16(1<<r.size)-1 {
				re.gidx = 0
			}
			swapped := atomic.CompareAndSwapUint64((*uint64)(unsafe.Pointer(r)), *(*uint64)((unsafe.Pointer)(&ore)), *(*uint64)((unsafe.Pointer)(&re))) //store(r, &ore, &re)
			if swapped {
				//atomic.AddInt64(&r.SwapsSucceeded, 1)
				value = v
				r.Gets++
				return
			}
			//atomic.AddInt64(&r.SwapsFailed, 1)
		}
		//r.GetSpins++
	}
}
