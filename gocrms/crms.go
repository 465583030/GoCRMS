package gocrms

import (
	"github.com/coreos/etcd/clientv3"
	"time"
	"encoding/json"
	"github.com/WenzheLiu/GoCRMS/common"
	"fmt"
	"path"
	"os"
	"errors"
	"os/exec"
	"log"
	"strconv"
)

const (
	CrmsNodePrefix   = "crms/"
	ServerNodePrefix = CrmsNodePrefix + "server/"
	JobNode = CrmsNodePrefix + "job"
	JobNodePrefix    = JobNode + "/"
	AssignNodePrefix = CrmsNodePrefix + "assign/"
)

func serverNode(serverName string) string {
	return ServerNodePrefix + serverName
}

func jobNode(jobId int) string {
	return JobNodePrefix + strconv.Itoa(jobId)
}

func assignServerNode(server string) string {
	return AssignNodePrefix + server
}

func assignNode(server string, jobId int) string {
	return assignServerNode(server) + "/" + strconv.Itoa(jobId)
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

///// server related methods, implement ServerOp interface

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

///// job related methods, implement ServerOp interface

// jobCommand is an array of each part of the command
func (crms *Crms) SubmitJob(cmd []string, jobName, server string, onHold bool) (jobID int, err error) {
	resp, err := crms.etcd.Get(JobNode)
	if err != nil {  // TODO: test the err is the return value or the local
		return
	}
	v, err := GetResponse{resp}.Value()
	if err != nil {
		return
	}
	lastJobId, err := strconv.Atoi(v)
	if err != nil {
		return
	}
	jobID = lastJobId + 1
	var status int
	if onHold {
		status = StatusOnHold
	} else {
		status = StatusPending
	}
	jobState := JobState{
		Command:cmd,
		Status:status,
		SubmitTime:time.Now(),
		Server:server,
	}

	state, err := json.Marshal(jobState)
	if err != nil {
		return
	}
	// TODO: can we call only one etcd req instead of 2 atomic
	// create job
	_, err = crms.etcd.Put(jobNode(jobID), string(state)) // TODO: can etcd check job non-exist
	if err != nil {
		return
	}
	// update last job ID
	// TODO: this op may be conflict with other, can etcd support CAS op
	_, err = crms.etcd.Put(JobNode, strconv.Itoa(jobID))

	// if not on hold, assign to server to run
	if !onHold {
		assignNode(server, jobID)
		_, err = crms.etcd.Put(assignNode(server, jobID), "")
		if err != nil {
			return
		}
	}
	return
}

func (crms *Crms) updateJob(job *Job) error {
	state, err := json.Marshal(job.State)
	if err != nil {
		return err
	}
	_, err = crms.etcd.Put(jobNode(job.ID), string(state))
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
		job, err := ParseJobFromJson(kv.K, kv.V)
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
	return ParseJobFromJson(id, v)
}

func (crms *Crms) WatchJobs(handler JobWatchHandler) *common.OnceFunc {
	rch, cancel := crms.etcd.WatchWithPrefix(JobNodePrefix)
	go HandleWatchEvt(rch, JobHandlerFactory(handler))
	return crms.cancelables.Add(cancel)
}

func (crms *Crms) FailJob(id string, jobErr error) error {
	job, err := crms.GetJob(id)
	if err != nil {
		return err
	}
	job.State.Status = StatusFail
	job.State.Error = jobErr.Error()
	return crms.updateJob(job)
}

func (crms *Crms) DoneJob(id string) error {
	job, err := crms.GetJob(id)
	if err != nil {
		return err
	}
	job.State.Status = StatusDone
	return crms.updateJob(job)
}

func (crms *Crms) RunJob(id) {

}

func (cs *CrmsServer) runJob(jobID string) error {
	job, err := cs.crms.GetJob(jobID)
	if err != nil {
		return err
	}
	if job.State.Status == StatusRunning {
		return errors.New(fmt.Sprintf("Job %s is already running", jobID))
	}
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	job.State.Host = hostname

	err = execCmd(job)
	job.State.EndTime = time.Now()
	if err != nil {
		job.State.Error = err.Error()
	} else {
		job.State.Error = ""
	}
	return err
}

func execCmd(job *Job) error {
	command := job.State.Command
	if len(command) == 0 {
		return errors.New(fmt.Sprint("No command to the job", job.ID))
	}
	cmd := exec.Command(command[0], command[1:]...)
	outPath := path.Join(joboutDir(), job.ID)
	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	cmd.Stdout = outFile
	cmd.Stderr = outFile
	log.Println("Run Job", job.ID, "with command", command)
	if err = cmd.Start(); err != nil {
		return err
	}

	job.State.Status = StatusRunning
	job.State.OutFile = outPath
	job.State.PID = cmd.Process.Pid
	job.State.StartTime = time.Now()

	return cmd.Wait()
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

func (crms *Crms) AssignJob(jobId string, server string) error {
	_, err := crms.etcd.Put(assignNode(server, jobId), "")
	return err
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
