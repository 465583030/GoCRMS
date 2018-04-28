package ut

import (
	"github.com/sergi/go-diff/diffmatchpatch"
	"fmt"
)

func Diff(old string, new string) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(old, new, false)
	return dmp.DiffPrettyText(diffs)
}

func DiffAny(old, new interface{}) string {
	o := fmt.Sprintf("%v", old)
	n := fmt.Sprintf("%v", new)
	return Diff(o, n)
}
