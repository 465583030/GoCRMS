package cmd

import (
	"time"
)

type GlobalFlags struct {
	Endpoints          []string
	DialTimeout        time.Duration
	RequestTimeout     time.Duration
	Debug bool
	LogToStdOut bool

	Name string
	SlotCount int
}

var globalFlags = GlobalFlags{}
