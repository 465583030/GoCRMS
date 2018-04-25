package gocrms

import (
	"io/ioutil"
	"strings"
	"errors"
	"github.com/sergi/go-diff/diffmatchpatch"
	"os"
	"fmt"
	"runtime"
	"path"
)

// to force update case, use env: update_golden=force, for example:
// update_golden=force go test -run TestEqWithDefaultGolden github.com/WenzheLiu/GoCRMS/gocrms
func EqWithGolden(goldenFile, actual string) error {
	return EqByFuncWithGolden(goldenFile, actual, func(actual, expected string) (bool, error) {
		return actual == expected, nil
	})
}

// default golden file is the <caller's dir>/golden/<caller's file name>/<caller's method name>
func DefaultGoldenFile() (string, error) {
	pc, callerFile, _, ok := runtime.Caller(1)
	if !ok {
		return "", errors.New("fail to get caller")
	}
	dir, callerFileName := path.Split(callerFile)
	var caseName string
	if dot := strings.LastIndex(callerFileName, "."); dot == -1 {
		caseName = callerFileName
	} else {
		caseName = callerFileName[:dot]
	}
	goldenDir := path.Join(dir, "golden", caseName)
	if err := os.MkdirAll(goldenDir, 0775); err != nil {
		return "", err
	}
	method := runtime.FuncForPC(pc).Name()
	method = path.Base(method)
	return path.Join(goldenDir, method), nil
}

func diff(old string, new string) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(old, new, false)
	return dmp.DiffPrettyText(diffs)
}

func EqByFuncWithGolden(goldenFile, actual string, isEqual func(actual, expected string) (bool, error)) error {
	actual = strings.Replace(actual, "\r\n", "\n", -1)

	gf, err := ioutil.ReadFile(goldenFile)
	if err != nil {
		if os.IsNotExist(err) {
			err = ioutil.WriteFile(goldenFile, []byte(actual), 0664)
			if err == nil {
				return errors.New(fmt.Sprintf(
					"NOT ERROR: create golden file %s with content:\n%s", goldenFile, actual))
			}
		}
		return err
	}
	expected := strings.Replace(string(gf), "\r\n", "\n", -1)

	ok, eqErr := isEqual(actual, expected)
	if ok {
		return nil
	}

	dif := diff(expected, actual)
	force := os.Getenv("update_golden")
	switch force {
	case "force":
		if err = ioutil.WriteFile(goldenFile, []byte(actual), 0666); err != nil {
			return ComposeErrors([]error{eqErr, err})
		} else {
			return ComposeErrors([]error{eqErr, errors.New(fmt.Sprintf(
				`Diff:
%s
--------------------------
NOT ERROR: update golden file %s with content:
%s`, dif, goldenFile, actual))})
		}
	default:
		return ComposeErrors([]error{eqErr, errors.New(fmt.Sprintf(
			`Diff with golden file %s
%s
---------- Actual: ---------
%s`, goldenFile, dif, actual))})
	}
}
