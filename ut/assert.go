package ut

import (
	"testing"
)

type assert struct {
	t *testing.T
	fail func()
}

// when fail, the test can continue
func Verify(t *testing.T) assert {
	return assert{t, t.Fail}
}

// when fail, the test will stop
func Assert(t *testing.T) assert {
	return assert{t, t.FailNow}
}

func (this assert) Eq(actual, expected interface{}, msg ...string) {
	if actual != expected {
		// To prevent the caller file is always shown as assert.go instead of the caller.
		// It is caused by t.Log inside use runtime.Call(skip=3).
		// To fix, just mark this method is a helper to testing framework,
		this.t.Helper()

		if len(msg) > 0 {
			this.t.Log(msg)
		} else {
			this.t.Log(DiffAny(expected, actual))
		}
		this.fail()
	}
}
