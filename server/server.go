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

package server

import (
	"context"
	"encoding/json"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"
)

const (
	jobStatusRunning = "running"
	jobStatusDone    = "done"
	jobStatusFail    = "fail"

	watchEvtPut    = 0
	watchEvtDelete = 1

	TimeOutServer    = 5 // second
	timeOutBeatHeart = 2 * time.Second

	ServerClose = "close"
)

type Server struct {
	name           string
	parellelCount  int
	cli            *clientv3.Client
	requestTimeout time.Duration
	jobChan        chan string
	closed         chan bool
	onClose        []func()
}

func NewServer(name string, parellelCount int, endpoints []string,
	dialTimeout time.Duration, requestTimeout time.Duration) (server *Server, err error) {

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: dialTimeout,
	})
	if err != nil {
		return
	}

	server = &Server{name, parellelCount, cli, requestTimeout,
		make(chan string, parellelCount), make(chan bool), make([]func(), 0)}

	// ready to run job parellel
	server.prepareForRun()
	return server, err
}

func (server *Server) put(key, val string, opts ...clientv3.OpOption) error {
	ctx, cancel := context.WithTimeout(context.Background(), server.requestTimeout)
	defer cancel()
	_, err := server.cli.Put(ctx, key, val, opts...)
	if err != nil {
		log.Println(err)
	}
	return err
}

func (server *Server) get(key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), server.requestTimeout)
	defer cancel()
	return server.cli.Get(ctx, key, opts...)
}

func (server *Server) watch(key string, opts ...clientv3.OpOption) (clientv3.WatchChan, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	return server.cli.Watch(ctx, key, opts...), cancel
}

func (server *Server) delete(key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), server.requestTimeout)
	defer cancel()
	return server.cli.Delete(ctx, key, opts...)
}

func (server *Server) Close() {
	for _, fn := range server.onClose {
		fn()
	}
	server.delete(server.node())
	server.cli.Close()
	close(server.jobChan)
	server.closed <- true
}

func (server *Server) getAssignedDir() string {
	return "crms/assign/" + server.name + "/"
}

func (server *Server) getAssignedJobKey(jobId string) string {
	return server.getAssignedDir() + jobId
}

func (server *Server) runJob(jobId string) {
	// get the job command by job Id
	jobIdNodeKey := "crms/job/" + jobId
	resp, err := server.get(jobIdNodeKey)
	if err != nil {
		log.Println(err)
		return
	}
	if len(resp.Kvs) == 0 {
		log.Println("No job with id", jobId)
		return
	}
	var cmdWithArgs []string
	err = json.Unmarshal(resp.Kvs[0].Value, &cmdWithArgs)
	if err != nil {
		log.Println(err)
		return
	}
	if len(cmdWithArgs) == 0 {
		log.Println("No command in the job", jobId)
		return
	}
	command := cmdWithArgs[0]
	args := cmdWithArgs[1:]

	// set job state: running
	jobState := jobIdNodeKey + "/state"
	stdouterr := jobState + "/stdouterr"
	server.delete(stdouterr)
	server.put(jobState, jobStatusRunning)

	// run job: execute command
	cmd := exec.Command(command, args...)

	log.Println("Run Job", jobId, "with command:", command, strings.Join(args, " "))

	//TODO: use cmd.Start() to async execute, and periodly update the stdout and stderr to node
	// job/<jobid>/state/stdout and stderr, so that user can known its running status.
	byteBuf, err := cmd.CombinedOutput()
	jobOutput := string(byteBuf)
	log.Println("Finish Job", jobId, "with result: ", jobOutput)

	// set job output
	server.put(stdouterr, jobOutput)

	if err != nil {
		// if error (exit value is not 0), set job state fail
		log.Println("Job", jobId, "fail:", err)
		server.put(jobState, jobStatusFail)
	} else {
		// set job state done
		server.put(jobState, jobStatusDone)
	}
	// remove the assign job assign to the server, as it is not running or pending by the server
	server.delete(server.getAssignedJobKey(jobId))
}

func (server *Server) node() string {
	return "crms/server/" + server.name
}

func (server *Server) Exists() (bool, error) {
	resp, err := server.get(server.node())
	if err != nil {
		return false, err
	}
	if len(resp.Kvs) == 0 {
		return false, nil
	}
	for _, kv := range resp.Kvs {
		if string(kv.Value) == ServerClose {
			return false, nil
		}
	}
	return true, nil
}

func (server *Server) Name() string {
	return server.name
}

func (server *Server) register() error {
	// register
	ctx, cancel := context.WithTimeout(context.Background(), server.requestTimeout)
	grantResp, err := server.cli.Grant(ctx, TimeOutServer)
	cancel()
	if err != nil {
		return err
	}

	serverNodeKey := server.node()
	server.put(serverNodeKey, strconv.Itoa(server.parellelCount), clientv3.WithLease(grantResp.ID))
	log.Println("Server", server.name, "has registered to", serverNodeKey)

	// beat heart
	go func(id clientv3.LeaseID) {
		for {
			ctx, cancel := context.WithTimeout(context.Background(), server.requestTimeout)
			_, kaerr := server.cli.KeepAliveOnce(ctx, id)
			cancel()
			if kaerr != nil {
				log.Println(kaerr)
				return
			}
			time.Sleep(timeOutBeatHeart)
		}
	}(grantResp.ID)
	return nil
}

func (server *Server) Register() error {
	if err := server.register(); err != nil {
		return err
	}

	// listen to the server node itself: write "close" means close the server,
	// if receive DELETE event may be network timeout issue and may be reconnect? .
	go func() {
		rch, cancel := server.watch(server.node(), clientv3.WithPrefix())
		server.deferOnClose(cancel)
		for wresp := range rch {
			for _, ev := range wresp.Events {
				switch ev.Type {
				case watchEvtPut:
					// "close" means close the server
					if ServerClose == string(ev.Kv.Value) {
						log.Println("Get close signal, server", server.name, "now is closing")
						server.Close()
					}
				case watchEvtDelete:
					log.Println("Server", server.name, "is deleted. Try re-register.")
					// reconnect
					if err := server.register(); err != nil {
						// close server if re try fail
						log.Println("Fail to re-register, cause:", err)
						server.Close()
						return
					}
				default:
					log.Printf("Unknown event type %s (%q : %q)\n", ev.Type, ev.Kv.Key, ev.Kv.Value)
				}
			}
		}
	}()

	return nil
}

// when starting/restarting server, get the works that already existed and run
func (server *Server) RunJobsAssigned() error {
	// get the node assign/<server>/*
	assignNodeKey := server.getAssignedDir()
	lenAssignNodeKey := len(assignNodeKey)

	resp, err := server.get(assignNodeKey, clientv3.WithPrefix())
	if err != nil {
		return err
	}
	// get the jobs' id
	jobs := make([]string, len(resp.Kvs))
	for i, kv := range resp.Kvs {
		// get the job id
		assignJobKey := string(kv.Key)
		jobId := assignJobKey[lenAssignNodeKey:]
		jobs[i] = jobId
	}
	// run jobs
	go func(jobIds []string) {
		for _, jobId := range jobIds {
			// send to job chan to run
			server.jobChan <- jobId
		}
	}(jobs)
	return err
}

// ready to run job parellel
func (server *Server) prepareForRun() {
	for i := 0; i < server.parellelCount; i++ {
		go func() {
			for jobId := range server.jobChan {
				server.runJob(jobId)
			}
		}()
	}
}

func (server *Server) deferOnClose(fn func()) {
	server.onClose = append(server.onClose, fn) //TODO: may not thread safe
}

// listen to the work assigned
func (server *Server) ListenNewJobAssigned() {
	go func() {
		assignNodeKey := server.getAssignedDir()
		lenAssignNodeKey := len(assignNodeKey)
		log.Println("Server", server.Name(), "is ready for work on node", assignNodeKey)
		rch, cancel := server.watch(assignNodeKey, clientv3.WithPrefix())
		server.deferOnClose(cancel)

		for wresp := range rch {
			for _, ev := range wresp.Events {
				switch ev.Type {
				case watchEvtPut:
					// get the job id
					assignJobKey := string(ev.Kv.Key)
					jobId := assignJobKey[lenAssignNodeKey:]
					server.jobChan <- jobId
				case watchEvtDelete:
					// do nothing
				default:
					log.Printf("Unknown event type %s (%q : %q)\n", ev.Type, ev.Kv.Key, ev.Kv.Value)
				}
			}
		}
	}()
}

// wait until close
func (server *Server) WaitUntilClose() {
	<-server.closed
}
