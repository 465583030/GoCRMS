package gocrms

import "encoding/json"

const (
	StatusNew     = "new"
	StatusRunning = "running"
	StatusDone    = "done"
	StatusFail    = "fail"
)

type JobState struct {
	ID    string // job ID
	State State  // job State
}

type State struct {
	Status  string // job status, value is StatusXXX
	Host    string // machine host to run the job
	PID     int    // process ID that run the job
	OutFile string // redirect stdout/stderr to the file
	Error   string // if fail, record the error
}

func NewJobState(id, jsonState string) (*JobState, error) {
	js := &JobState{
		ID:    id,
	}
	err := json.Unmarshal([]byte(jsonState), &js.State)
	return js, err
}

type JobStateWatchHandler interface {
	OnPut(jobState *JobState)
	OnDelete(jobState *JobState)
}

type JobStateWatchFunc struct {
	HandlePut    func(jobState *JobState)
	HandleDelete func(jobState *JobState)
}

func (w JobStateWatchFunc) OnPut(jobState *JobState) {
	if w.HandlePut != nil {
		w.HandlePut(jobState)
	}
}

func (w JobStateWatchFunc) OnDelete(jobState *JobState) {
	if w.HandleDelete != nil {
		w.HandleDelete(jobState)
	}
}

type jobStateWatchHandlerAdapter struct {
	jobState *JobState
	handler  JobStateWatchHandler
}

func (h *jobStateWatchHandlerAdapter) OnPut() {
	h.handler.OnPut(h.jobState)
}

func (h *jobStateWatchHandlerAdapter) OnDelete() {
	h.handler.OnDelete(h.jobState)
}

func JobStateHandlerFactory(handler JobStateWatchHandler) WatchHandlerFactory {
	return func(k, v string) (WatchHandler, error) {
		var jobID string
		if n := len(JobStateNodePrefix); k[:n] == JobStateNodePrefix {
			jobID = k[n:]
		} else {
			jobID = k
		}
		jobState, err := NewJobState(jobID, v)
		return &jobStateWatchHandlerAdapter{
			jobState: jobState,
			handler:  handler,
		}, err
	}
}
