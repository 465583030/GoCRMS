package base

import (
	"testing"
	"errors"
)

func TestComposableError(t *testing.T) {
	// define error variable
	var err error
	// test 0 error
	err = NewComposableError([]error{})
	if err != nil {
		t.Error(err)
	}

	// test 1 error
	err = NewComposableError([]error{
		errors.New("not wenzhe's error"),
	})
	if err.Error() != "not wenzhe's error" {
		t.Error(err)
	}

	// test 3 errors
	err = NewComposableError([]error{
		errors.New("not wenzhe's error"),
		errors.New("should be your error"),
		errors.New("never my error"),
	})
	if err.Error() != `Error 1: not wenzhe's error
Error 2: should be your error
Error 3: never my error` {
		t.Error(err)
	}
}
