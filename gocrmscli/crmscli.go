// Copyright 2018 Wenzhe Liu (liuwenzhe2008@qq.com)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gocrmscli

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"
)

const (
	crmsPrefix   = "crms/"
	workerPrefix = crmsPrefix + "worker/"
	jobPrefix    = crmsPrefix + "job/"
	assignPrefix = crmsPrefix + "assign/"

	WATCH_EVT_PUT    = 0
	WATCH_EVT_DELETE = 1

	STATUS_NEW     = "new"
	STATUS_RUNNING = "running"
	STATUS_DONE    = "done"
	STATUS_FAIL    = "fail"
)

type JobState struct {
	Status    string
	Stdouterr string
}

type Job struct {
	ID             string
	Command        []string             // Job Command is an array of each part of the command
	StateOfWorkers map[string]*JobState // key: assigned worker name, value: JobData (state + stdout/err)
}

func NewJob(id string) *Job {
	return &Job{
		ID:             id,
		Command:        make([]string, 0),
		StateOfWorkers: make(map[string]*JobState),
	}
}

func (job *Job) GetStateOrCreateIfAbsent(worker string) *JobState {
	state, exist := job.StateOfWorkers[worker]
	if !exist {
		state = &JobState{"new", ""}
		job.StateOfWorkers[worker] = state
	}
	return state
}

func (job *Job) GetState(worker string) (state *JobState, exist bool) {
	state, exist = job.StateOfWorkers[worker]
	return
}

func (job *Job) GetStatus() string {
	status := ""
	for _, state := range job.StateOfWorkers {
		switch status {
		case STATUS_NEW:
			if state.Status != STATUS_DONE {
				status = state.Status
			}
		case STATUS_FAIL:
			// do nothing, keep fail
		case STATUS_RUNNING:
			if state.Status == STATUS_FAIL {
				status = STATUS_FAIL
			}
		case STATUS_DONE:
			status = state.Status
		default:
			status = state.Status
		}
	}
	if status == "" {
		return STATUS_NEW
	} else {
		return status
	}
}

//TODO: here only get the any one of the worker's stdouterr, should merge or take other action
func (job *Job) GetStdOutErr() string {
	for _, state := range job.StateOfWorkers {
		if state.Stdouterr != "" {
			return state.Stdouterr
		}
	}
	return ""
}

type Worker struct {
	Name            string
	ParellelAbility int
	// the job current assign to the worker, worker is now handling it or waiting for handling.
	// it is used as a set, the value is useless (as Go doesn't have set)
	Jobs map[string]bool
}

func NewWorker(name string, parellelAbility int) *Worker {
	return &Worker{name, parellelAbility, make(map[string]bool)}
}

type CrmsCli struct {
	cli                *clientv3.Client
	requestTimeout     time.Duration
	workers            map[string]*Worker // key: worker name, value: worker parellel job count
	jobs               map[string]*Job    // key: jobId, value: Job
	cancelWatchWorkers context.CancelFunc
	cancelWatchJobs    context.CancelFunc
	cancelWatchAssign  context.CancelFunc
}

func New(endpoints []string, dialTimeout time.Duration, requestTimeout time.Duration) (crms *CrmsCli, err error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: dialTimeout,
	})
	if err != nil {
		return
	}
	crms = &CrmsCli{
		cli:            cli,
		requestTimeout: requestTimeout,
		workers:        make(map[string]*Worker),
		jobs:           make(map[string]*Job),
	}
	return
}

func (crms *CrmsCli) Close() {
	if crms.cancelWatchWorkers != nil {
		crms.cancelWatchWorkers()
	}
	if crms.cancelWatchJobs != nil {
		crms.cancelWatchJobs()
	}
	if crms.cancelWatchAssign != nil {
		crms.cancelWatchAssign()
	}
	crms.cli.Close()
}

func (crms *CrmsCli) GetWorker(name string) (worker *Worker, exist bool, err error) {
	workers, err := crms.GetWorkers()
	if err != nil {
		return
	}
	worker, exist = workers[name]
	return
}

func (crms *CrmsCli) GetWorkers() (workers map[string]*Worker, err error) {
	if crms.cancelWatchWorkers == nil {
		crms.workers, err = crms.getWorkers()
		if err != nil {
			return
		}
		crms.watchWorkers() //TODO: if the worker change before watch event start, how to do?
	}
	if crms.cancelWatchAssign == nil {
		crms.watchAssign()
	}
	workers = crms.workers
	return
}

func (crms *CrmsCli) getWorkers() (workers map[string]*Worker, err error) {
	resp, err := crms.get(workerPrefix, clientv3.WithPrefix())
	if err != nil {
		log.Println(err)
		return
	}
	workers = make(map[string]*Worker)
	for _, kv := range resp.Kvs {
		workerName := string(kv.Key)[len(workerPrefix):]
		v := string(kv.Value)
		if v == "close" {
			continue
		}
		parellelAbility, err := strconv.Atoi(v)
		if err != nil {
			log.Println(err)
			continue
		}
		worker := NewWorker(workerName, parellelAbility)
		jobIds, err := crms.GetAssignJobs(workerName)
		if err != nil {
			log.Println(err)
			continue
		}
		worker.Jobs = make(map[string]bool)
		for _, jobId := range jobIds {
			worker.Jobs[jobId] = true
		}
		workers[workerName] = worker
	}
	return
}

func (crms *CrmsCli) watchWorkers() {
	rch, cancel := crms.watch(workerPrefix, clientv3.WithPrefix())
	crms.cancelWatchWorkers = cancel
	go func(workers map[string]*Worker) {
		for wresp := range rch {
			for _, ev := range wresp.Events {
				name := string(ev.Kv.Key)[len(workerPrefix):]
				switch ev.Type {
				case WATCH_EVT_PUT:
					v := string(ev.Kv.Value)
					if v == "close" {
						log.Println("delete worker", name)
						delete(workers, name)
						continue
					}
					parellelAbility, err := strconv.Atoi(v)
					if err != nil {
						log.Println(err)
						continue
					}
					worker, exist := workers[name]
					if exist {
						worker.ParellelAbility = parellelAbility
					} else {
						worker := NewWorker(name, parellelAbility)
						workers[name] = worker
					}
				case WATCH_EVT_DELETE:
					log.Println("delete worker", name)
					delete(workers, name)
				default:
					log.Printf("Unknown event type %s (%q : %q)\n", ev.Type, ev.Kv.Key, ev.Kv.Value)
				}
			}
		}
	}(crms.workers)
}

func (crms *CrmsCli) StopWorker(name string) error {
	return crms.put(workerPrefix+name, "close")
}

// jobCommand is an array of each part of the command
func (crms *CrmsCli) CreateJob(jobId string, jobCommand []string) error {
	cmd, err := json.Marshal(jobCommand)
	if err != nil {
		return err
	}
	crms.put(jobPrefix+jobId, string(cmd))
	return nil
}

func (crms *CrmsCli) RunJob(jobId string, worker string) error {
	return crms.put(assignPrefix+worker+"/"+jobId, "")
}

func (crms *CrmsCli) getJobOrCreateIfAbsent(jobId string) *Job {
	job, exist := crms.jobs[jobId]
	if !exist {
		job = NewJob(jobId)
		crms.jobs[jobId] = job
	}
	return job
}

// k starts with "crms/job/<jobid>"
func (crms *CrmsCli) updateJob(k string, v []byte) error {
	ks := strings.Split(k, "/")[2:]
	n := len(ks)
	jobId := ks[0]
	job := crms.getJobOrCreateIfAbsent(jobId)
	switch n {
	case 1:
		job.ID = jobId
		if err := json.Unmarshal(v, &job.Command); err != nil {
			return err
		}
	case 3:
		worker := ks[2]
		state := job.GetStateOrCreateIfAbsent(worker)
		state.Status = string(v)
	case 4:
		worker := ks[2]
		state := job.GetStateOrCreateIfAbsent(worker)
		prop := ks[3]
		if prop == "stdouterr" {
			state.Stdouterr = string(v)
		}
	}
	return nil
}

// example of key-value format in etcd server:
//    job/3
//    ["ls", "-l", ".."]
//    job/3/state/wenzhe
//    done
//    job/3/state/wenzhe/stdouterr
//    total 1760
//    drwxr-xr-x 1 weliu 1049089       0 Dec 13 17:18 angular
//    drwxr-xr-x 1 weliu 1049089       0 Jan 17 16:53 bctools
//    drwxr-xr-x 1 weliu 1049089       0 Jan  2 09:47 cluster
func (crms *CrmsCli) getJobs() (map[string]*Job, error) {
	resp, err := crms.get(jobPrefix, clientv3.WithPrefix())
	if err != nil {
		return crms.jobs, err
	}
	for _, kv := range resp.Kvs {
		k := string(kv.Key)
		v := kv.Value
		crms.updateJob(k, v)
	}
	return crms.jobs, nil
}

func (crms *CrmsCli) watchJobs() {
	rch, cancel := crms.watch(jobPrefix, clientv3.WithPrefix())
	crms.cancelWatchJobs = cancel
	go func() {
		for wresp := range rch {
			for _, ev := range wresp.Events {
				switch ev.Type {
				case WATCH_EVT_PUT:
					err := crms.updateJob(string(ev.Kv.Key), ev.Kv.Value)
					if err != nil {
						log.Println(err)
					}
				case WATCH_EVT_DELETE:
					// currently no job remove yet
				default:
					log.Printf("Unknown event type %s (%q : %q)\n", ev.Type, ev.Kv.Key, ev.Kv.Value)
				}
			}
		}
	}()
}

func (crms *CrmsCli) GetJobs() (jobs map[string]*Job, err error) {
	if crms.cancelWatchJobs == nil {
		jobs, err = crms.getJobs()
		if err != nil {
			return
		}
		crms.watchJobs()
	} else {
		jobs = crms.jobs
	}
	return
}

func (crms *CrmsCli) GetJob(jobId string) (job *Job, exist bool, err error) {
	jobs, err := crms.getJobs()
	if err != nil {
		return
	}
	job, exist = jobs[jobId]
	return
}

// return the assign jobs' id that currently assign to the worker (not including finish)
func (crms *CrmsCli) GetAssignJobs(worker string) (jobs []string, err error) {
	prefix := assignPrefix + worker + "/"
	n := len(prefix)
	resp, err := crms.get(prefix, clientv3.WithPrefix())
	if err != nil {
		return
	}
	jobs = make([]string, len(resp.Kvs))
	for i, kv := range resp.Kvs {
		jobId := string(kv.Key)[n:]
		jobs[i] = jobId
	}
	return
}

func (crms *CrmsCli) GetJobsByWorker(worker string) (jobs []*Job, err error) {
	js, err := crms.GetJobs()
	if err != nil {
		return
	}
	jobs = make([]*Job, 0, len(jobs))
	for _, job := range js {
		_, exist := job.StateOfWorkers[worker]
		if exist {
			jobs = append(jobs, job)
		}
	}
	return
}

// pending job means the job has assigned to worker but has not yet started.
// the way to check is check "assign/<worker>/<jobId>" exist and "job/<jobId>/<worker>" not exist
func (crms *CrmsCli) GetPendingJobsByWorker(worker string) (jobs []*Job, err error) {
	assignJobIds, err := crms.GetAssignJobs(worker)
	if err != nil {
		return
	}
	jobs = make([]*Job, 0)
	for _, jobId := range assignJobIds {
		job, exist, err := crms.GetJob(jobId)
		if err != nil {
			log.Println(err)
			continue
		}
		if !exist {
			log.Println("no job with id", jobId)
			continue
		}
		_, exist = job.StateOfWorkers[worker]
		if !exist {
			jobs = append(jobs, job)
		}
	}
	return
}

func (crms *CrmsCli) watchAssign() {
	rch, cancel := crms.watch(assignPrefix, clientv3.WithPrefix())
	crms.cancelWatchAssign = cancel
	go func(workers map[string]*Worker) {
		for wresp := range rch {
			for _, ev := range wresp.Events {
				k := string(ev.Kv.Key) // k starts with crms/assign/<worker_name>
				ks := strings.Split(k, "/")
				if len(ks) < 3 { // invalid
					log.Println("Invalid key", k)
					continue
				}
				ks = ks[2:]
				workerName := ks[0]
				worker, exist := crms.workers[workerName] //TODO: may not thread safe
				if !exist {
					// nothing need to update
					continue
				}
				jobId := ks[1]
				if ev.Type == WATCH_EVT_DELETE {
					delete(worker.Jobs, jobId)
				} else if ev.Type == WATCH_EVT_PUT {
					worker.Jobs[jobId] = true
				}
			}
		}
	}(crms.workers)
}

// TODO: the following common method is duplicate with worker.go.
func (crms *CrmsCli) put(key, val string, opts ...clientv3.OpOption) error {
	ctx, cancel := context.WithTimeout(context.Background(), crms.requestTimeout)
	defer cancel()
	_, err := crms.cli.Put(ctx, key, val, opts...)
	if err != nil {
		log.Println(err)
	}
	return err
}

func (crms *CrmsCli) get(key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), crms.requestTimeout)
	defer cancel()
	return crms.cli.Get(ctx, key, opts...)
}

func (crms *CrmsCli) watch(key string, opts ...clientv3.OpOption) (clientv3.WatchChan, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	return crms.cli.Watch(ctx, key, opts...), cancel
}

func (crms *CrmsCli) delete(key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), crms.requestTimeout)
	defer cancel()
	return crms.cli.Delete(ctx, key, opts...)
}

func (crms *CrmsCli) GetNodes() (nodes map[string]string, err error) {
	resp, err := crms.get(crmsPrefix, clientv3.WithPrefix())
	if err != nil {
		return
	}
	nodes = make(map[string]string)
	for _, kv := range resp.Kvs {
		k := string(kv.Key)
		v := string(kv.Value)
		nodes[k] = v
	}
	return
}
