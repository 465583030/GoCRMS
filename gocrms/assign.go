package gocrms

type AssignWatchHandler interface {
	OnPut(jobId string)
	OnDelete(jobId string)
}

type AssignWatchFunc struct {
	HandlePut func(jobId string)
	HandleDelete func(jobId string)
}

func (w AssignWatchFunc) OnPut(jobId string) {
	if w.HandlePut != nil {
		w.HandlePut(jobId)
	}
}

func (w AssignWatchFunc) OnDelete(jobId string) {
	if w.HandleDelete != nil {
		w.HandleDelete(jobId)
	}
}

type assignWatchHandlerAdapter struct {
	jobId string
	handler AssignWatchHandler
}

func (h *assignWatchHandlerAdapter) OnPut() {
	h.handler.OnPut(h.jobId)
}

func (h *assignWatchHandlerAdapter) OnDelete() {
	h.handler.OnDelete(h.jobId)
}

func AssignHandlerFactory(server string, handler AssignWatchHandler) WatchHandlerFactory {
	return func(k, v string) (WatchHandler, error) {
		jobId := k[len(assignServerNode(server)) + 1:]  // "crms/asign/<server>/"
		return &assignWatchHandlerAdapter{
			jobId: jobId,
			handler: handler,
		}, nil
	}
}