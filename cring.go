// Copyright © 2016 Lawrence E. Bakst. All rights reserved.

// Concurrent ring buffer, safe to call from goroutines
// Currently slower than using a channel by about 3X

package cring

import (
	"flag"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"leb.io/hrff"
	"leb.io/stats"
)

type cring struct {
	pidx  uint16
	cidx  uint16
	mask  uint16
	shift uint8
	size  uint8
	//fill  uint8
}

var values []int
var ps uint64
var gs uint64
var ss uint64
var sf uint64

func tdiff(begin, end time.Time) time.Duration {
	d := end.Sub(begin)
	return d
}

func New(size int) (cr *cring) {
	values = make([]int, 1<<uint(size))
	cr = &cring{pidx: 0, cidx: 0, mask: uint16(1<<uint(size)) - 1, shift: uint8(size), size: uint8(size + 1)}
	fmt.Printf("cr=%v\n", cr)
	return
}

func load(r *cring) cring {
	sp := (*uint64)(unsafe.Pointer(r))
	ui := atomic.LoadUint64(sp)
	crp := (*cring)(unsafe.Pointer(&ui))
	return *crp
}

func cp(dp *uint64, sp *uint64) {
	*dp = *sp
}

func store(r *cring, ov, nv *cring) bool {
	rp := (*uint64)(unsafe.Pointer(r))
	ovp := (*uint64)(unsafe.Pointer(ov))
	nvp := (*uint64)(unsafe.Pointer(nv))
	/*
		cp(rp, nvp)
		*rp = *nvp
		b := true
	*/

	b := atomic.CompareAndSwapUint64(rp, *ovp, *nvp)
	if b {
		atomic.AddUint64(&ss, 1)
	} else {
		atomic.AddUint64(&sf, 1)
	}
	return b
}

var pcnt int

func (r *cring) Put(v int) bool {
	var ui uint64
	var re, ore cring

	pcnt = 0
	for {
		//re := *(*cring)(unsafe.Pointer(uintptr(atomic.LoadUint64((*uint64)(unsafe.Pointer(r)))))) // load(r)
		//re := load(r)
		ui = atomic.LoadUint64((*uint64)(unsafe.Pointer(r)))
		re = *(*cring)(unsafe.Pointer(&ui))
		//re = (cring)((uintptr)(atomic.LoadUint64((*uint64)(unsafe.Pointer(r)))))
		ore = re
		switch {
		case re.pidx&re.mask == re.cidx&re.mask && re.pidx>>re.shift != re.cidx>>re.shift:
			// full
			return true
		case re.pidx&re.mask == re.cidx&re.mask && re.pidx>>re.shift == re.cidx>>re.shift:
			// empty
			fallthrough
		default:
			//fmt.Printf("pidx=%d ", re.pidx)
			values[re.pidx&re.mask] = v
			re.pidx++
			if re.pidx > uint16(1<<re.size)-1 {
				re.pidx = 0
			}
			b := atomic.CompareAndSwapUint64((*uint64)(unsafe.Pointer(r)), *(*uint64)((unsafe.Pointer)(&ore)), *(*uint64)((unsafe.Pointer)(&re))) //store(r, &ore, &re)
			if b {
				atomic.AddUint64(&ss, 1)
				return false
			}
			atomic.AddUint64(&sf, 1)
		}
		ps++
		pcnt++
		//fmt.Printf("PC ")
	}
}

var gcnt int

func (r *cring) Get() (int, bool) {
	var ui uint64
	var re, ore cring

	gcnt = 0
	for {
		//re := *(*cring)(unsafe.Pointer(uintptr(atomic.LoadUint64((*uint64)(unsafe.Pointer(r)))))) // load(r)
		ui = atomic.LoadUint64((*uint64)(unsafe.Pointer(r)))
		re = *(*cring)(unsafe.Pointer(&ui))
		ore = re
		switch {
		case re.pidx&re.mask == re.cidx&re.mask && re.pidx>>re.shift == re.cidx>>re.shift:
			// empty
			return 0, true
		case re.pidx&re.mask == re.cidx&re.mask && re.pidx>>re.shift != re.cidx>>re.shift:
			// full
			fallthrough
		default:
			//fmt.Printf("cidx=%d ", re.cidx)
			ret := values[re.cidx&re.mask]
			re.cidx++
			if re.cidx > uint16(1<<re.size)-1 {
				re.cidx = 0
			}
			b := atomic.CompareAndSwapUint64((*uint64)(unsafe.Pointer(r)), *(*uint64)((unsafe.Pointer)(&ore)), *(*uint64)((unsafe.Pointer)(&re))) //store(r, &ore, &re)
			if b {
				atomic.AddUint64(&ss, 1)
				return ret, false
			}
			atomic.AddUint64(&sf, 1)
		}
		gs++
		gcnt++
		//fmt.Printf("GC ")
	}
}
