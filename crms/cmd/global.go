package cmd

import "time"

type GlobalFlags struct {
	Endpoints          []string
	DialTimeout        time.Duration
	RequestTimeout     time.Duration
	OutputFormat string
	Debug bool
}

var global = GlobalFlags{}
