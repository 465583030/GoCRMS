package base

type JobOut struct {
	ID string
	Out string
}

func NewJobOut(id, out string) (*JobOut, error) {
	return &JobOut{
		ID: id,
		Out: out,
	}, nil
}

type JobOutWatchHandler interface {
	OnPut(jobOut *JobOut)
	OnDelete(jobOut *JobOut)
}

type JobOutWatchFunc struct {
	HandlePut func(jobOut *JobOut)
	HandleDelete func(jobOut *JobOut)
}

func (w JobOutWatchFunc) OnPut(jobOut *JobOut) {
	if w.HandlePut != nil {
		w.HandlePut(jobOut)
	}
}

func (w JobOutWatchFunc) OnDelete(jobOut *JobOut) {
	if w.HandleDelete != nil {
		w.HandleDelete(jobOut)
	}
}

type jobOutWatchHandlerAdapter struct {
	jobOut *JobOut
	handler JobOutWatchHandler
}

func (h *jobOutWatchHandlerAdapter) OnPut() {
	h.handler.OnPut(h.jobOut)
}

func (h *jobOutWatchHandlerAdapter) OnDelete() {
	h.handler.OnDelete(h.jobOut)
}

func JobOutHandlerFactory(handler JobOutWatchHandler) WatchHandlerFactory {
	return func(k, v string) (WatchHandler, error) {
		jobOut, err := NewJobOut(k, v)
		return &jobOutWatchHandlerAdapter{
			jobOut: jobOut,
			handler: handler,
		}, err
	}
}