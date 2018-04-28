package common

import (
	"sync"
	"unsafe"
	"sync/atomic"
)

// Add Done method to sync.Once
type Once struct {
	sync.Once
}

func (once *Once) Done() bool {
	// get the private field value of sync.Once
	o := (*struct {
		_    sync.Mutex
		done uint32
	})(unsafe.Pointer(&once.Once))
	return atomic.LoadUint32(&o.done) == 1
}

type OnceFunc struct {
	once Once
	fn func()
}

func NewOnceFunc(fn func()) *OnceFunc {
	return &OnceFunc{
		fn: fn,
	}
}

func (of *OnceFunc) Call() {
	of.once.Do(of.fn)
}

func (of *OnceFunc) Called() bool {
	return of.once.Done()
}

type OnceFuncs struct {
	fns        []*OnceFunc
	locker sync.Mutex
}

func NewOnceFuncs(cap int) *OnceFuncs {
	return &OnceFuncs{
		fns: make([]*OnceFunc, 0, cap),
	}
}

func (ofs *OnceFuncs) Add(fn func()) *OnceFunc {
	ofs.locker.Lock()
	defer ofs.locker.Unlock()
	if len(ofs.fns) == cap(ofs.fns) {
		ofs.removeCanceled()
	}
	of := NewOnceFunc(fn)
	ofs.fns = append(ofs.fns, of)
	return of
}

func (ofs *OnceFuncs) CallAll() {
	ofs.locker.Lock()
	defer ofs.locker.Unlock()
	for _, fn := range ofs.fns {
		fn.Call()
	}
	ofs.fns = ofs.fns[:0]
}

// Remove whose cancel func has been called
// 0 1x 2 3x 4 5x 6 7
// 0 2 4 6 7
func (ofs *OnceFuncs) removeCanceled() bool {
	j := 0 // new slice's len
	fns := ofs.fns
	for i, fn := range fns {
		if !fn.Called() {
			if i > j {
				fns[j] = fn
			}
			j++
		}
	}
	if j < len(fns) {
		ofs.fns = fns[:j]
		return true
	} else {
		return false
	}
}
