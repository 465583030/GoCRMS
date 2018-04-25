package cmd

import (
	"time"
)

type GlobalFlags struct {
	Endpoints          []string
	DialTimeout        time.Duration
	RequestTimeout     time.Duration
	OutputFormat string
	Debug bool
	LogToStdOut bool

	Name string
	SlotCount int
}

const (
	// Output format value
	OutFormat_JSON = "json"
	OutFormat_JSON_Compact = "json_compact"
)

var globalFlags = GlobalFlags{}
