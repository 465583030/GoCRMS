package gocrms

import (
	"github.com/coreos/etcd/clientv3"
	"time"
	"encoding/json"
	"github.com/WenzheLiu/GoCRMS/common"
)

const (
	CrmsNodePrefix   = "crms/"
	ServerNodePrefix = CrmsNodePrefix + "server/"
	JobNodePrefix    = CrmsNodePrefix + "job/"
	AssignNodePrefix = CrmsNodePrefix + "assign/"
	JobStateNodePrefix    = CrmsNodePrefix + "jobstate/"
	JobOutNodePrefix    = CrmsNodePrefix + "jobout/"
)

func serverNode(serverName string) string {
	return ServerNodePrefix + serverName
}

func jobNode(jobId string) string {
	return JobNodePrefix + jobId
}

func assignServerNode(server string) string {
	return AssignNodePrefix + server
}

func assignNode(server, jobId string) string {
	return assignServerNode(server) + "/" + jobId
}

func jobStateNode(jobId string) string {
	return JobStateNodePrefix + jobId
}

func jobOutNode(jobId string) string {
	return JobOutNodePrefix + jobId
}

type Crms struct {
	etcd               Etcd
	cancelables        common.OnceFuncs
}

func NewCrms(cfg clientv3.Config, requestTimeout time.Duration) (*Crms, error) {
	cli, err := clientv3.New(cfg)
	if err != nil {
		return nil, err
	}
	crms := &Crms{
		etcd:        Etcd{cli, requestTimeout},
		cancelables: *common.NewOnceFuncs(4),
	}
	return crms, nil
}

func (crms *Crms) Close() {
	crms.cancelables.CallAll()
	crms.etcd.Close()
}

func (crms *Crms) GetServer(name string) (server *Server, exist bool, err error) {
	resp, err := crms.etcd.Get(serverNode(name))
	if err != nil {
		return
	}
	r := GetResponse{resp}
	if r.Len() == 0 {
		return nil, false, nil
	}
	exist = true
	v, err := GetResponse{resp}.Value()
	if err != nil {
		return
	}
	server, err = NewServer(name, v)
	return
}

func (crms *Crms) GetServers() (servers []*Server, err error) {
	resp, err := crms.etcd.GetWithPrefix(ServerNodePrefix)
	if err != nil {
		return
	}
	kvs := GetResponse{resp}.KeyValues()
	servers = make([]*Server, len(kvs))
	for i, kv := range kvs {
		name := kv.K[len(ServerNodePrefix):]
		server, err := NewServer(name, kv.V)
		if err != nil {
			return nil, err
		}
		servers[i] = server
	}
	return servers, nil
}

func (crms *Crms) UpdateServer(server *Server) error {
	_, err := crms.etcd.Put(serverNode(server.Name), server.StringValue())
	return err
}

func (crms *Crms) WatchServer(name string, handler ServerWatchHandler) (cancelFunc *common.OnceFunc) {
	rch, cancel := crms.etcd.Watch(serverNode(name))
	go HandleWatchEvt(rch, ServerHandlerFactory(handler))
	return crms.cancelables.Add(cancel)
}

func (crms *Crms) WatchServers(handler ServerWatchHandler) (cancelFunc *common.OnceFunc) {
	rch, cancel := crms.etcd.WatchWithPrefix(ServerNodePrefix)
	go HandleWatchEvt(rch, ServerHandlerFactory(handler))
	return crms.cancelables.Add(cancel)
}

func (crms *Crms) StopServer(name string) error {
	_, err := crms.etcd.Put(serverNode(name), CloseServer)
	return err
}

func (crms *Crms) StopAllServers() (count int, err error) {
	servers, err := crms.GetServers()
	if err != nil {
		return
	}
	var errs []error
	for _, server := range servers {
		if !server.Closed {
			if e := crms.StopServer(server.Name); e != nil {
				errs = append(errs, e)
			}
			count++
		}
	}
	return count, common.ComposeErrors(errs...)
}

// jobCommand is an array of each part of the command
func (crms *Crms) CreateJob(job *Job) error {
	cmd, err := json.Marshal(job.Command)
	if err != nil {
		return err
	}
	_, err = crms.etcd.Put(jobNode(job.ID), string(cmd))
	return err
}

func (crms *Crms) RunJob(jobId string, server string) error {
	_, err := crms.etcd.Put(assignNode(server, jobId), "")
	return err
}

// example of key-value format in etcd server:
//    job/3
//    ["ls", "-l", ".."]
func (crms *Crms) GetJobs() ([]*Job, error) {
	resp, err := crms.etcd.GetWithPrefix(JobNodePrefix)
	if err != nil {
		return nil, err
	}
	kvs := GetResponse{resp}.KeyValues()
	jobs := make([]*Job, len(kvs))
	for i, kv := range kvs {
		job, err := NewJob(kv.K, kv.V)
		if err != nil {
			return jobs, err
		}
		jobs[i] = job
	}
	return jobs, nil
}

func (crms *Crms) GetJob(id string) (*Job, error) {
	resp, err := crms.etcd.Get(jobNode(id))
	if err != nil {
		return nil, err
	}
	v, err := GetResponse{resp}.Value()
	if err != nil {
		return nil, err
	}
	return NewJob(id, v)
}

func (crms *Crms) WatchJobs(handler JobWatchHandler) *common.OnceFunc {
	rch, cancel := crms.etcd.WatchWithPrefix(JobNodePrefix)
	go HandleWatchEvt(rch, JobHandlerFactory(handler))
	return crms.cancelables.Add(cancel)
}

//    jobstate/3
//    done
func (crms *Crms) GetJobState(id string) (*JobState, error) {
	resp, err := crms.etcd.Get(jobStateNode(id))
	if err != nil {
		return nil, err
	}
	res := GetResponse{resp}
	if res.Len() == 0 {
		return NewJobState(id, StatusNew)
	}
	v, err := res.Value()
	if err != nil {
		return nil, err
	}
	return NewJobState(id, v)
}

func (crms *Crms) WatchJobState(id string, handler JobStateWatchHandler) *common.OnceFunc {
	rch, cancel := crms.etcd.Watch(jobStateNode(id))
	go HandleWatchEvt(rch, JobStateHandlerFactory(handler))
	return crms.cancelables.Add(cancel)
}

func (crms *Crms) UpdateJobState(id string, state string) error {
	_, err := crms.etcd.Put(jobStateNode(id), state)
	return err
}

//    jobout/3
//    total 1760
//    drwxr-xr-x 1 weliu 1049089       0 Dec 13 17:18 angular
//    drwxr-xr-x 1 weliu 1049089       0 Jan 17 16:53 bctools
//    drwxr-xr-x 1 weliu 1049089       0 Jan  2 09:47 cluster
func (crms *Crms) GetJobOut(id string) (*JobOut, error) {
	resp, err := crms.etcd.Get(jobOutNode(id))
	if err != nil {
		return nil, err
	}
	res := GetResponse{resp}
	if res.Len() == 0 {
		return NewJobOut(id, "")
	}
	v, err := res.Value()
	if err != nil {
		return nil, err
	}
	return NewJobOut(id, v)
}

func (crms *Crms) WatchJobOut(id string, handler JobOutWatchHandler) *common.OnceFunc {
	rch, cancel := crms.etcd.Watch(jobOutNode(id))
	go HandleWatchEvt(rch, JobOutHandlerFactory(handler))
	return crms.cancelables.Add(cancel)
}

func (crms *Crms) Reset() error {
	_, err := crms.StopAllServers()
	if err != nil {
		return err
	}
	// remove all node under crms/
	_, err = crms.etcd.DeleteWithPrefix(CrmsNodePrefix)
	return err
}

func (crms *Crms) Nodes() ([]KV, error) {
	resp, err := crms.etcd.GetWithPrefix(CrmsNodePrefix)
	if err != nil {
		return nil, err
	}
	return GetResponse{resp}.KeyValues(), nil
}

func (crms *Crms) GetAssignJobs(server string) ([]string, error) {
	assignNode := assignServerNode(server) + "/"
	resp, err := crms.etcd.GetWithPrefix(assignNode)
	if err != nil {
		return nil, err
	}
	kvs := GetResponse{resp}.KeyValues()
	jobIds := make([]string, len(kvs))
	for i, kv := range kvs {
		jobIds[i] = kv.K[len(assignNode):]
	}
	return jobIds, nil
}

func (crms *Crms) WatchAssignJobs(server string, handler AssignWatchHandler) *common.OnceFunc {
	assignNode := assignServerNode(server) + "/"
	rch, cancel := crms.etcd.WatchWithPrefix(assignNode)
	go HandleWatchEvt(rch, AssignHandlerFactory(server, handler))
	return crms.cancelables.Add(cancel)
}

func (crms *Crms) DeleteAssignJob(server string, jobID string) error {
	_, err := crms.etcd.Delete(assignNode(server, jobID))
	return err
}
