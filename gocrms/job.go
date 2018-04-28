package gocrms

import (
	"encoding/json"
	"time"
)

const (
	// status enum
	StatusPending = iota // wait for schedule
	StatusRunning
	StatusDone
	StatusExit   // fail
	StatusOnHold // not schedule
)

type JobOp interface {
	// create the job, assign to a server,
	// and run async if not set as on hold and the server has resource.
	// return the job ID (increase by 1, start from 1)
	SubmitJob(cmd []string, jobName, server string, onHold bool) (id int, err error)

	// release on hold job, then the job will be scheduled
	ReleaseOnHoldJob(id int) error

	// the the job detail information by id
	GetJob(id int) (*Job, error)

	// kill the job by id
	KillJob(id int) error
}

type Job struct {
	ID string
	JobState
}

type JobState struct {
	Command []string // Job Command is an array of each part of the command
	Status  int // job status, value is StatusXXX
	Host    string // machine host to run the job
	PID     int    // process ID that run the job
	OutFile string // redirect stdout/stderr to the file
	Error   string // if fail, record the error
	SubmitTime time.Time // the time to submit the job
	StartTime time.Time // the time when start to run the job
	EndTime time.Time // the time when done/fail to run the job
	Server string // the server to run the job
}

func ParseJobFromJson(id, jsonJobState string) (*Job, error) {
	job := &Job{
		ID: id,
	}
	err := json.Unmarshal([]byte(jsonJobState), &job.JobState)
	return job, err
}

type JobWatchHandler interface {
	OnPut(job *Job)
	OnDelete(job *Job)
}

type JobWatchFunc struct {
	HandlePut func(job *Job)
	HandleDelete func(job *Job)
}

func (w JobWatchFunc) OnPut(job *Job) {
	if w.HandlePut != nil {
		w.HandlePut(job)
	}
}

func (w JobWatchFunc) OnDelete(job *Job) {
	if w.HandleDelete != nil {
		w.HandleDelete(job)
	}
}

type jobWatchHandlerAdapter struct {
	job *Job
	handler JobWatchHandler
}

func (h *jobWatchHandlerAdapter) OnPut() {
	h.handler.OnPut(h.job)
}

func (h *jobWatchHandlerAdapter) OnDelete() {
	h.handler.OnDelete(h.job)
}

func JobHandlerFactory(handler JobWatchHandler) WatchHandlerFactory {
	return func(k, v string) (WatchHandler, error) {
		var jobID string
		if n := len(JobNodePrefix); len(k) >= n && k[:n] == ServerNodePrefix {
			jobID = k[n:]
		} else {
			jobID = k
		}
		job, err := ParseJobFromJson(jobID, v)
		return &jobWatchHandlerAdapter{
			job: job,
			handler: handler,
		}, err
	}
}
