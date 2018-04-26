package gocrms

import (
	"github.com/coreos/etcd/clientv3"
	"log"
)

const (
	WatchEvtPut    = 0
	WatchEvtDelete = 1
)

type WatchEvtData struct {
	Type int
	KV
}

type WatchHandlerFactory func(k, v string) (WatchHandler, error)

type WatchHandler interface {
	OnPut()
	OnDelete()
}

func HandleWatchEvt(rch clientv3.WatchChan, createHandler WatchHandlerFactory) {
	for wresp := range rch {
		for _, ev := range wresp.Events {
			handler, err := createHandler(string(ev.Kv.Key), string(ev.Kv.Value))
			if err != nil {
				log.Println(err)
				continue
			}
			switch ev.Type {
			case WatchEvtPut:
				handler.OnPut()
			case WatchEvtDelete:
				handler.OnDelete()
			default:
				log.Printf("Unknown event type %s (%q : %q)\n", ev.Type, ev.Kv.Key, ev.Kv.Value)
			}
		}
	}
}

type KVWatchHandler interface {
	OnPut(k, v string)
	OnDelete(k string)
}

type WatchFunc struct {
	HandlePut func(k, v string)
	HandleDelete func(k string)
}

func (w WatchFunc) OnPut(k, v string) {
	if w.HandlePut != nil {
		w.HandlePut(k, v)
	}
}

func (w WatchFunc) OnDelete(k string) {
	if w.HandleDelete != nil {
		w.HandleDelete(k)
	}
}

type kvWatchHandlerAdapter struct {
	k, v string
	handler KVWatchHandler
}

func (h *kvWatchHandlerAdapter) OnPut() {
	h.handler.OnPut(h.k, h.v)
}

func (h *kvWatchHandlerAdapter) OnDelete() {
	h.handler.OnDelete(h.k)
}

func KVHandlerFactory(handler KVWatchHandler) WatchHandlerFactory {
	return func(k, v string) (WatchHandler, error) {
		return &kvWatchHandlerAdapter{
			k: k,
			v: v,
			handler: handler,
		}, nil
	}
}
