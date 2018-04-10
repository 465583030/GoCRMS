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
	"errors"
)

const (
	crmsPrefix   = "crms/"
	serverPrefix = crmsPrefix + "server/"
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
	ID      string
	Command []string             // Job Command is an array of each part of the command
	State   JobState // key: assigned server name, value: JobData (state + stdout/err)
}

func NewJob(id string) *Job {
	return &Job{
		ID:      id,
		Command: make([]string, 0),
		State:   JobState{STATUS_NEW, ""},
	}
}

func (job *Job) GetStatus() string {
	return job.State.Status
}

func (job *Job) GetStdOutErr() string {
	return job.State.Stdouterr
}

type Server struct {
	Name            string
	ParellelAbility int
	// the job current assign to the server, server is now handling it or waiting for handling.
	// it is used as a set, the value is useless (as Go doesn't have set)
	Jobs map[string]bool
}

func NewServer(name string, parellelAbility int) *Server {
	return &Server{name, parellelAbility, make(map[string]bool)}
}

type CrmsCli struct {
	cli                *clientv3.Client
	requestTimeout     time.Duration
	servers            map[string]*Server // key: server name, value: server parellel job count
	jobs               map[string]*Job    // key: jobId, value: Job
	cancelWatchServers context.CancelFunc
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
		servers:        make(map[string]*Server),
		jobs:           make(map[string]*Job),
	}
	return
}

func (crms *CrmsCli) Close() {
	if crms.cancelWatchServers != nil {
		crms.cancelWatchServers()
	}
	if crms.cancelWatchJobs != nil {
		crms.cancelWatchJobs()
	}
	if crms.cancelWatchAssign != nil {
		crms.cancelWatchAssign()
	}
	crms.cli.Close()
}

func (crms *CrmsCli) GetServer(name string) (server *Server, exist bool, err error) {
	servers, err := crms.GetServers()
	if err != nil {
		return
	}
	server, exist = servers[name]
	return
}

func (crms *CrmsCli) GetServers() (servers map[string]*Server, err error) {
	if crms.cancelWatchServers == nil {
		crms.servers, err = crms.getServers()
		if err != nil {
			return
		}
		crms.watchServers() //TODO: if the server change before watch event start, how to do?
	}
	if crms.cancelWatchAssign == nil {
		crms.watchAssign()
	}
	servers = crms.servers
	return
}

func (crms *CrmsCli) getServers() (servers map[string]*Server, err error) {
	resp, err := crms.get(serverPrefix, clientv3.WithPrefix())
	if err != nil {
		log.Println(err)
		return
	}
	servers = make(map[string]*Server)
	for _, kv := range resp.Kvs {
		serverName := string(kv.Key)[len(serverPrefix):]
		v := string(kv.Value)
		if v == "close" {
			continue
		}
		parellelAbility, err := strconv.Atoi(v)
		if err != nil {
			log.Println(err)
			continue
		}
		server := NewServer(serverName, parellelAbility)
		jobIds, err := crms.GetAssignJobs(serverName)
		if err != nil {
			log.Println(err)
			continue
		}
		server.Jobs = make(map[string]bool)
		for _, jobId := range jobIds {
			server.Jobs[jobId] = true
		}
		servers[serverName] = server
	}
	return
}

func (crms *CrmsCli) watchServers() {
	rch, cancel := crms.watch(serverPrefix, clientv3.WithPrefix())
	crms.cancelWatchServers = cancel
	go func(servers map[string]*Server) {
		for wresp := range rch {
			for _, ev := range wresp.Events {
				name := string(ev.Kv.Key)[len(serverPrefix):]
				switch ev.Type {
				case WATCH_EVT_PUT:
					v := string(ev.Kv.Value)
					if v == "close" {
						log.Println("delete server", name)
						delete(servers, name)
						continue
					}
					parellelAbility, err := strconv.Atoi(v)
					if err != nil {
						log.Println(err)
						continue
					}
					server, exist := servers[name]
					if exist {
						server.ParellelAbility = parellelAbility
					} else {
						server := NewServer(name, parellelAbility)
						servers[name] = server
					}
				case WATCH_EVT_DELETE:
					log.Println("delete server", name)
					delete(servers, name)
				default:
					log.Printf("Unknown event type %s (%q : %q)\n", ev.Type, ev.Kv.Key, ev.Kv.Value)
				}
			}
		}
	}(crms.servers)
}

func (crms *CrmsCli) StopServer(name string) error {
	return crms.put(serverPrefix+name, "close")
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

func (crms *CrmsCli) RunJob(jobId string, server string) error {
	return crms.put(assignPrefix+server+"/"+jobId, "")
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
	case 2:
		if ks[1] == "state" {
			job.State.Status = string(v)
		}
	case 3:
		if ks[1] == "state" && ks[2] == "stdouterr" {
			job.State.Stdouterr = string(v)
		}
	}
	return nil
}

// example of key-value format in etcd server:
//    job/3
//    ["ls", "-l", ".."]
//    job/3/state
//    done
//    job/3/state/stdouterr
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

// return the assign jobs' id that currently assign to the server (not including finish)
func (crms *CrmsCli) GetAssignJobs(server string) (jobs []string, err error) {
	prefix := assignPrefix + server + "/"
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

func (crms *CrmsCli) GetJobsByServer(server string) (jobs []*Job, err error) {
	assignJobIds, err := crms.GetAssignJobs(server)
	if err != nil {
		return
	}
	if len(assignJobIds) == 0 {
		return
	}
	js, err := crms.GetJobs()
	if err != nil {
		return
	}
	jobs = make([]*Job, 0, len(assignJobIds))
	for i, jobId := range assignJobIds {
		job, exist := js[jobId]
		if !exist {
			err = errors.New("job id " + jobId + " doesn't exist")
			return
		}
		jobs[i] = job
	}
	return
}

func (crms *CrmsCli) watchAssign() {
	rch, cancel := crms.watch(assignPrefix, clientv3.WithPrefix())
	crms.cancelWatchAssign = cancel
	go func(servers map[string]*Server) {
		for wresp := range rch {
			for _, ev := range wresp.Events {
				k := string(ev.Kv.Key) // k starts with crms/assign/<server_name>
				ks := strings.Split(k, "/")
				if len(ks) < 3 { // invalid
					log.Println("Invalid key", k)
					continue
				}
				ks = ks[2:]
				serverName := ks[0]
				server, exist := crms.servers[serverName] //TODO: may not thread safe
				if !exist {
					// nothing need to update
					continue
				}
				jobId := ks[1]
				if ev.Type == WATCH_EVT_DELETE {
					delete(server.Jobs, jobId)
				} else if ev.Type == WATCH_EVT_PUT {
					server.Jobs[jobId] = true
				}
			}
		}
	}(crms.servers)
}

// TODO: the following common method is duplicate with server.go.
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
