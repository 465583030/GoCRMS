package base

import (
	"strconv"
)

const (
	CloseServer = "close"
)

type Server struct {
	Name      string
	SlotCount int
	Closed bool
}

func (server *Server) StringValue() string {
	if server.Closed {
		return CloseServer
	} else {
		return strconv.Itoa(server.SlotCount)
	}
}

func NewServer(name, value string) (*Server, error) {
	if value == CloseServer {
		return &Server{
			Name: name,
			Closed: true,
		}, nil
	}
	slotCount, err := strconv.Atoi(value)
	if err != nil {
		return nil, err
	}
	return &Server{
		Name: name,
		SlotCount: slotCount,
	}, nil
}

type ServerWatchHandler interface {
	OnPut(server *Server)
	OnDelete(server *Server)
}

type ServerWatchFunc struct {
	HandlePut func(server *Server)
	HandleDelete func(server *Server)
}

func (w ServerWatchFunc) OnPut(server *Server) {
	if w.HandlePut != nil {
		w.HandlePut(server)
	}
}

func (w ServerWatchFunc) OnDelete(server *Server) {
	if w.HandleDelete != nil {
		w.HandleDelete(server)
	}
}

type serverWatchHandlerAdapter struct {
	server *Server
	handler ServerWatchHandler
}

func (h *serverWatchHandlerAdapter) OnPut() {
	h.handler.OnPut(h.server)
}

func (h *serverWatchHandlerAdapter) OnDelete() {
	h.handler.OnDelete(h.server)
}

func ServerHandlerFactory(handler ServerWatchHandler) WatchHandlerFactory {
	return func(k, v string) (WatchHandler, error) {
		var name string
		if n := len(ServerNodePrefix); k[:n] == ServerNodePrefix {
			name = k[n:]
		} else {
			name = k
		}
		server, err := NewServer(name, v)
		return &serverWatchHandlerAdapter{
			server: server,
			handler: handler,
		}, err
	}
}
