package base

import (
	"strings"
	"strconv"
)

type ComposableError struct {
	errs []error
}

func NewComposableError(errs []error) *ComposableError {
	if len(errs) == 0 {
		return nil // not an error
	} else {
		return &ComposableError{errs}
	}
}

func (cerr *ComposableError) Error() string {
	switch len(cerr.errs) {
	case 0:
		panic("should not reach because not an error!")
	case 1:
		return cerr.errs[0].Error()
	default:
		var sb strings.Builder
		sb.WriteString("Error 1: ")
		sb.WriteString(cerr.errs[0].Error())
		for i, err := range cerr.errs[1:] {
			sb.WriteString("\nError ")
			sb.WriteString(strconv.Itoa(2 + i))
			sb.WriteString(": ")
			sb.WriteString(err.Error())
		}
		return sb.String()
	}
}
