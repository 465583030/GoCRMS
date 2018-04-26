package gocrms

import (
	"testing"
	"time"
	"context"
)

func TestOnceDone(t *testing.T) {
	var once Once
	cnt := 0
	fn := func(){
		cnt++
	}

	if once.Done() {
		t.Fatal("Do hasn't called yet")
	}

	// do first time
	once.Do(fn)
	if !once.Done() {
		t.Fatal("Do has called yet")
	}
	if cnt != 1 {
		t.Fatal(cnt)
	}

	// do second time
	once.Do(fn)
	if !once.Done() {
		t.Fatal("Do has called yet")
	}
	// count is still 1
	if cnt != 1 {
		t.Fatal(cnt)
	}
}

func TestCancelFunc(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cf := NewOnceFunc(cancel)
	defer cf.Call()

	if cf.Called() {
		t.Fatal(cf.Called())
	}

	result := make(chan int)
	go func() {
		i := 0
		for {
			select {
			case <- ctx.Done():
				result <- i
				close(result)
				return
			case <- time.After(10 * time.Millisecond):
				i++
				if i > 10 {
					t.Fatal("time out of 10s")
				}
			}
		}
	}()

	cf.Call()

	if !cf.Called() {
		t.Fatal(cf.Called())
	}
	res, ok := <- result
	if !ok {
		t.Fatal(ok)
	}
	if res > 5 {
		t.Fatal(res)
	}
}

func TestOnceFuncs(t *testing.T) {
	const size = 8
	var cnts [size]int
	var fns [size]func()
	for i := 0; i < size; i++ {
		j := i
		fns[i] = func() {
			cnts[j]++
		}
	}

	ofs := NewOnceFuncs(4)
	of0 := ofs.Add(fns[0])
	of1 := ofs.Add(fns[1])
	of2 := ofs.Add(fns[2])
	of1.Call()
	of2.Call()
	if len(ofs.fns) != 3 || cap(ofs.fns) != 4 {
		t.Fatalf("len: %d , cap: %d", len(ofs.fns), cap(ofs.fns))
	}
	of3 := ofs.Add(fns[3])
	if len(ofs.fns) != 4 || cap(ofs.fns) != 4 {
		t.Fatalf("len: %d , cap: %d", len(ofs.fns), cap(ofs.fns))
	}

	// call again of1 and of2
	of1.Call()
	of2.Call()

	// the value still keep 1
	if cnts != [...]int{0,1,1,0,0,0,0,0} {
		t.Fatal(cnts)
	}

	// as now reach cap, add 1 will trigger remove called func
	of4 := ofs.Add(fns[4])

	if len(ofs.fns) != 3 || cap(ofs.fns) != 4 {
		t.Fatalf("len: %d , cap: %d", len(ofs.fns), cap(ofs.fns))
	}
	if ofs.fns[0] != of0 {
		t.Fatal(ofs.fns)
	}
	if ofs.fns[1] == of1 || ofs.fns[1] != of3 {
		t.Fatal(ofs.fns)
	}
	if ofs.fns[2] == of2 || ofs.fns[2] != of4 {
		t.Fatal(ofs.fns)
	}

	of4.Call()
	if cnts != [...]int{0,1,1,0,1,0,0,0} {
		t.Fatal(cnts)
	}

	// as now not reach cap, add 1 not trigger remove
	of5 := ofs.Add(fns[5])
	if len(ofs.fns) != 4 || cap(ofs.fns) != 4 {
		t.Fatalf("len: %d , cap: %d", len(ofs.fns), cap(ofs.fns))
	}
	if ofs.fns[3] == of3 || ofs.fns[3] != of5 {
		t.Fatal(ofs.fns)
	}

	// as now reach cap, add 1 will trigger remove
	of6 := ofs.Add(fns[6])
	if len(ofs.fns) != 4 || cap(ofs.fns) != 4 {
		t.Fatalf("len: %d , cap: %d", len(ofs.fns), cap(ofs.fns))
	}
	if ofs.fns[2] != of5 {
		t.Fatal(ofs.fns)
	}
	if ofs.fns[3] != of6 {
		t.Fatal(ofs.fns)
	}

	// as now reach cap, add 1 will trigger remove, nothing to remove will expand the cap
	of7 := ofs.Add(fns[7])
	if len(ofs.fns) != 5 || cap(ofs.fns) != 8 {
		t.Fatalf("len: %d , cap: %d", len(ofs.fns), cap(ofs.fns))
	}
	if ofs.fns[4] != of7 {
		t.Fatal(ofs.fns)
	}

	// call of6
	of6.Call()
	if cnts != [...]int{0,1,1,0,1,0,1,0} {
		t.Fatal(cnts)
	}
	// call all
	ofs.CallAll()
	if cnts != [...]int{1,1,1,1,1,1,1,1} {
		t.Fatal(cnts)
	}
	// as call all will remove all, so len should be 0, but cap not changed
	if len(ofs.fns) != 0 || cap(ofs.fns) != 8 {
		t.Fatal(ofs.fns)
	}
	// call all again, nothing change
	ofs.CallAll()
	if cnts != [...]int{1,1,1,1,1,1,1,1} {
		t.Fatal(cnts)
	}
	if len(ofs.fns) != 0 || cap(ofs.fns) != 8 {
		t.Fatalf("len: %d , cap: %d", len(ofs.fns), cap(ofs.fns))
	}
	// call any again, not change
	of7.Call()
	if cnts != [...]int{1,1,1,1,1,1,1,1} {
		t.Fatal(cnts)
	}
	if len(ofs.fns) != 0 || cap(ofs.fns) != 8 {
		t.Fatalf("len: %d , cap: %d", len(ofs.fns), cap(ofs.fns))
	}
}
