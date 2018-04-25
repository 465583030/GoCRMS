package gocrms

import (
	"path"
	"os"
	"log"
	"io"
)

var (
	DataDir = path.Join(os.Getenv("HOME"), ".gocrms")
)

func joboutDir() string {
	return path.Join(DataDir, "jobout")
}

func logDir() string {
	return path.Join(DataDir, "log")
}

func MkDataDir() {
	for _, dir := range [...]string{joboutDir(), logDir()} {
		if err := os.MkdirAll(dir, 0775); err != nil {
			log.Fatalf("Fail to make directory for %s, reason: %v", dir, err)
		}
	}
}

// format is the parameter of log.SetFlags, for example: log.Ldate | log.Lmicroseconds | log.Lshortfile
func InitServerLog(serverName string, format int, stdout bool) *os.File {
	// set output
	logFile, err := os.OpenFile(path.Join(logDir(), serverName + ".log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,0664)
	if err != nil {
		log.Fatalln("Fail to open the log file", err)
	}
	if stdout {
		log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	} else {
		log.SetOutput(logFile)
	}

	// set format
	log.SetFlags(format)

	return logFile
}
