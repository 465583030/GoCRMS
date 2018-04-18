package base

import (
	"encoding/json"
)

type Job struct {
	ID      string
	Command []string // Job Command is an array of each part of the command
}

func jobCmdFromJson(data []byte) ([]string, error) {
	cmd := make([]string, 0, 4)
	if err := json.Unmarshal(data, &cmd); err != nil {
		return nil, err
	}
	return cmd, nil
}

func NewJob(id, jsonCmd string) (*Job, error) {
	cmd, err := jobCmdFromJson([]byte(jsonCmd))
	if err != nil {
		return nil, err
	}
	return &Job{
		ID: id,
		Command: cmd,
	}, nil
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
		job, err := NewJob(k, v)
		return &jobWatchHandlerAdapter{
			job: job,
			handler: handler,
		}, err
	}
}
