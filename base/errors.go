package base

import (
	"strings"
	"strconv"
)

type ComposableError []error

func ComposeErrors(errs []error) error {
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
