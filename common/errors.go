package gocrms

import (
	"strings"
	"strconv"
)

type ComposableError []error

func ComposeErrors(errors ...error) error {
	// remove nil from the slice
	errs := make([]error, 0, len(errors))
	for _, e := range errors {
		if e != nil {
			errs = append(errs, e)
		}
	}
	if len(errs) == 0 {
		return nil // not an error
	} else {
		return ComposableError(errs)
	}
}

func (cerr ComposableError) Error() string {
	switch len(cerr) {
	case 0:
		panic("should not reach because not an error!")
	case 1:
		return cerr[0].Error()
	default:
		var sb strings.Builder
		sb.WriteString("Error 1: ")
		sb.WriteString(cerr[0].Error())
		for i, err := range cerr[1:] {
			sb.WriteString("\nError ")
			sb.WriteString(strconv.Itoa(2 + i))
			sb.WriteString(": ")
			sb.WriteString(err.Error())
		}
		return sb.String()
	}
}
