package base

import (
	"github.com/coreos/etcd/clientv3"
	"time"
	"context"
)

const (
	CrmsNodePrefix   = "crms/"
	ServerNodePrefix = CrmsNodePrefix + "server/"
	JobNodePrefix    = CrmsNodePrefix + "job/"
	AssignNodePrefix = CrmsNodePrefix + "assign/"

	WatchEvtPut    = 0
	WatchEvtDelete = 1

	StatusNew     = "new"
	StatusRunning = "running"
	StatusDone    = "done"
	StatusFail    = "fail"
)

type JobState struct {
	Status    string
	StdOutErr string
}

type Job struct {
	ID      string
	Command []string // Job Command is an array of each part of the command
	State   JobState
}

func NewJob(id string) *Job {
	return &Job{
		ID:      id,
		Command: make([]string, 0),
		State:   JobState{StatusNew, ""},
	}
}

type Server struct {
	Name            string
	SlotCount       int
	// the job current assign to the server, server is now handling it or waiting for handling.
	// it is used as a set, the value is useless (as Go doesn't have set)
	Jobs map[string]bool
}

func NewServer(name string, slotCount int) *Server {
	return &Server{name, slotCount, make(map[string]bool)}
}



type Crms struct {
	etcd               Etcd
	cancelables        []context.CancelFunc
}

func New(endpoints []string, dialTimeout time.Duration, requestTimeout time.Duration) (*Crms, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: dialTimeout,
	})
	if err != nil {
		return nil, err
	}
	crms := &Crms{
		etcd:        Etcd{cli, requestTimeout},
		cancelables: []context.CancelFunc{},
	}
	return crms, nil
}

func (crms *Crms) Close() {
	for _, cancel := range crms.cancelables {
		cancel()
	}
	crms.etcd.Close()
}

func (crms *Crms) serverNode(serverName string) string {
	return ServerNodePrefix + serverName
}

func (crms *Crms) GetServer(name string) (server *Server, exist bool, err error) {
	resp, err := crms.etcd.Get(crms.serverNode(name))
	if err != nil {
		return
	}
	kvs := ParseGetResponse(resp)
}
