package ut

import (
	"testing"
	"path"
	"runtime"
)

const (
	s1 = "hello test\r\neq with\ndefault golden\nmy name is wenzhe liu"
	s2 = "Hello TEst\r\nNEW LINE\nNOT eq with\r\nmy name is wenzhe"
)

func TestEqWithDefaultGolden(t *testing.T) {
	// test default golden file
	gf := DefaultGoldenFile()  // gocrms/golden/golden_test
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal(ok)
	}
	dir := path.Dir(file)
	Verify(t).Eq(gf, path.Join(dir, "golden", "golden_test", "ut.TestEqWithDefaultGolden"))

	// test eq
	if err := EqWithGolden(gf, s1); err != nil {
		t.Fatal(err)
	}
	if err := EqWithGolden(gf, s2); err != nil {
		// t.Error(err)  ---- to see the error, just uncomment it
	} else {
		t.Error(err)
	}
}

func TestDiff(t *testing.T) {
	// t.Error(Diff(s1, s2)) ---- to see the pretty Diff, just uncomment it
}
