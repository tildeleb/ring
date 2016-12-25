// Copyright © 2015-2016 Lawrence E. Bakst. All rights reserved.

// Concurrent ring buffer, safe to call from goroutines

package main

import (
	"flag"
	"fmt"
	"sync"
	"sync/atomic"
	_ "time"
	"unsafe"
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

func store(r *cring, ov, nv *cring) bool {
	rp := (*uint64)(unsafe.Pointer(r))
	ovp := (*uint64)(unsafe.Pointer(ov))
	nvp := (*uint64)(unsafe.Pointer(nv))
	b := atomic.CompareAndSwapUint64(rp, *ovp, *nvp)
	if b {
		atomic.AddUint64(&ss, 1)
	} else {
		atomic.AddUint64(&sf, 1)
	}
	return b
}

func (r *cring) Put(v int) bool {
	for {
		re := load(r)
		ore := re
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
			if store(r, &ore, &re) {
				return false
			}
		}
		ps++
		//fmt.Printf("PC ")
	}
}

func (r *cring) Get() (int, bool) {
	for {
		re := load(r)
		ore := re
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
			if store(r, &ore, &re) {
				return ret, false
			}
		}
		gs++
		//fmt.Printf("GC ")
	}
}

var wg sync.WaitGroup

func producer(cr *cring, n int) {
	for i := 0; i < n; i++ {
	retry:
		if cr.Put(i) {
			//fmt.Printf("pR ")
			goto retry
		}
		//fmt.Printf("p=%d\n", i)
	}
	wg.Done()
}

func consumer(cr *cring, n int) {
	var cnt int
	for i := 0; i < n; i++ {
	retry:
		v, b := cr.Get()
		if b {
			//fmt.Printf("cR=%v\n", cr)
			goto retry
		}
		if v != cnt {
			fmt.Printf("v=%d, cnt=%d\n", v, cnt)
			panic("consumer")
		}
		//fmt.Printf("c=%d\n", v)
		cnt++
	}
	wg.Done()
}

var n = flag.Int("n", 1000*1000, "n")
var s = flag.Int("s", 10, "ring size")

func main() {
	flag.Parse()
	wg.Add(2)
	cr := New(*s)
	fmt.Printf("size=%d\n", uint16(1<<cr.size))
	go producer(cr, *n)
	go consumer(cr, *n)
	wg.Wait()
	fmt.Printf("ps=%d, gs=%d, ss=%d, sf=%d\n", ps, gs, ss, sf)
	//time.Sleep(10 * time.Second)
	return

	fmt.Printf("size=%d\n", unsafe.Sizeof(cr))
	cr.Put(1)
	cr.Put(2)
	cr.Put(3)
	cr.Put(4)
	for i := 0; i <= 4; i++ {
		v, b := cr.Get()
		fmt.Printf("b=%v, v=%v\n", b, v)
	}
}
