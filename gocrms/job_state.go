package gocrms

const (
	StatusNew     = "new"
	StatusRunning = "running"
	StatusDone    = "done"
	StatusFail    = "fail"
)

type JobState struct {
	ID      string
	State	string
}

func NewJobState(id, state string) (*JobState, error) {
	return &JobState{
		ID: id,
		State: state,
	}, nil
}

type JobStateWatchHandler interface {
	OnPut(jobState *JobState)
	OnDelete(jobState *JobState)
}

type JobStateWatchFunc struct {
	HandlePut func(jobState *JobState)
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
	handler JobStateWatchHandler
}

func (h *jobStateWatchHandlerAdapter) OnPut() {
	h.handler.OnPut(h.jobState)
}

func (h *jobStateWatchHandlerAdapter) OnDelete() {
	h.handler.OnDelete(h.jobState)
}

func JobStateHandlerFactory(handler JobStateWatchHandler) WatchHandlerFactory {
	return func(k, v string) (WatchHandler, error) {
		jobState, err := NewJobState(k, v)
		return &jobStateWatchHandlerAdapter{
			jobState: jobState,
			handler: handler,
		}, err
	}
}
