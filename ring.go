// Copyright © 2016,2017 Lawrence E. Bakst. All rights reserved.

// Concurrent ring buffer, safe to call from goroutines
// Currently slower than using a channel by about 3X

package ring

import (
	_ "fmt"
	"sync/atomic"
	"unsafe"
)

type Stats struct {
	Ps   uint64
	Gs   uint64
	Ss   uint64
	Sf   uint64
	Gcnt uint64
	Pcnt uint64
}

var Gstats Stats

type Pointer struct {
	pidx  uint16
	cidx  uint16
	mask  uint16
	shift uint8
	size  uint8
	//fill  uint8
}

type Ring struct {
	Pointer
	Stats
	Values []int
}

func New(size int) (r *Ring) {
	r = &Ring{Pointer: Pointer{mask: uint16(1<<uint(size)) - 1, shift: uint8(size), size: uint8(size + 1)}}
	r.Values = make([]int, 1<<uint(size))
	//fmt.Printf("r=%v\n", r)
	return
}

/*
func (p *Pointer) load() Pointer {
	uip := (*uint64)(unsafe.Pointer(p))
	ui := atomic.LoadUint64(uip)
	return *(*Pointer)(unsafe.Pointer(&ui))
}

func cp(dp *uint64, sp *uint64) {
	*dp = *sp
}

func store(r *Ring, ov, nv *Pointer) bool {
	rp := (*uint64)(unsafe.Pointer(r))
	ovp := (*uint64)(unsafe.Pointer(ov))
	nvp := (*uint64)(unsafe.Pointer(nv))
	b := atomic.CompareAndSwapUint64(rp, *ovp, *nvp)
	if b {
		atomic.AddUint64(&Gstats.Ss, 1)
	} else {
		atomic.AddUint64(&Gstats.Sf, 1)
	}
	return b
}
*/

func (r *Ring) Put(v int) bool {
	var ui uint64
	var re, ore Pointer

	r.Pcnt = 0
	for {
		//re := *(*cring)(unsafe.Pointer(uintptr(atomic.LoadUint64((*uint64)(unsafe.Pointer(r)))))) // load(r)
		//re := load(r)
		ui = atomic.LoadUint64((*uint64)(unsafe.Pointer(&r.Pointer)))
		re = *(*Pointer)(unsafe.Pointer(&ui))
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
			r.Values[re.pidx&re.mask] = v
			re.pidx++
			if re.pidx > uint16(1<<re.size)-1 {
				re.pidx = 0
			}
			swapped := atomic.CompareAndSwapUint64((*uint64)(unsafe.Pointer(r)), *(*uint64)((unsafe.Pointer)(&ore)), *(*uint64)((unsafe.Pointer)(&re))) //store(r, &ore, &re)
			if swapped {
				atomic.AddUint64(&r.Ss, 1)
				return false
			}
			atomic.AddUint64(&r.Sf, 1)
		}
		r.Ps++
		r.Pcnt++
		//fmt.Printf("PC ")
	}
}

func (r *Ring) Get() (int, bool) {
	var ui uint64
	var re, ore Pointer

	r.Gcnt = 0
	for {
		//re := *(*cring)(unsafe.Pointer(uintptr(atomic.LoadUint64((*uint64)(unsafe.Pointer(r)))))) // load(r)
		ui = atomic.LoadUint64((*uint64)(unsafe.Pointer(r)))
		re = *(*Pointer)(unsafe.Pointer(&ui))
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
			ret := r.Values[re.cidx&re.mask]
			re.cidx++
			if re.cidx > uint16(1<<re.size)-1 {
				re.cidx = 0
			}
			swapped := atomic.CompareAndSwapUint64((*uint64)(unsafe.Pointer(r)), *(*uint64)((unsafe.Pointer)(&ore)), *(*uint64)((unsafe.Pointer)(&re))) //store(r, &ore, &re)
			if swapped {
				atomic.AddUint64(&r.Ss, 1)
				return ret, false
			}
			atomic.AddUint64(&r.Sf, 1)
		}
		r.Gs++
		r.Gcnt++
		//fmt.Printf("GC ")
	}
}
