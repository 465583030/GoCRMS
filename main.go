package main

import (
	"context"
	"flag"
	"fmt"
	//	"io/ioutil"
	"log"
	//"net/http"
	//"strings"
	"encoding/json"
	//	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/coreos/etcd/clientv3"
	//"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	//grpcprom "github.com/grpc-ecosystem/go-grpc-prometheus"
	//"github.com/prometheus/client_golang/prometheus/promhttp"
	//"google.golang.org/grpc"
)

const (
	JOB_STATUS_NEW     = "new"
	JOB_STATUS_PENDING = "pending"
	JOB_STATUS_RUNNING = "running"
	JOB_STATUS_DONE    = "done"
	JOB_STATUS_FAIL    = "fail"
	JOB_STATUS_STOPPED = "stopped"
)

type Job struct {
	id      int
	command string
	args    []string
	status  string
}

type Worker struct {
	name           string
	parellelCount  int
	cli            *clientv3.Client
	requestTimeout time.Duration
	jobChan        chan string
	closed         chan bool
}

func NewWorker(name string, parellelCount int, endpoints []string,
	dialTimeout time.Duration, requestTimeout time.Duration) (worker *Worker, err error) {

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: dialTimeout,
	})
	if err != nil {
		return
	}

	worker = &Worker{name, parellelCount, cli, requestTimeout,
		make(chan string, parellelCount), make(chan bool)}

	// ready to run job parellel
	worker.prepareForRun()
	return worker, err
}

const (
	dialTimeout    = 5 * time.Second
	requestTimeout = 10 * time.Second
)

var (
	endpoints = []string{"localhost:2379"}
)

func (worker *Worker) put(key, val string, opts ...clientv3.OpOption) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	_, err := worker.cli.Put(ctx, key, val, opts...)
	cancel()
	if err != nil {
		log.Fatal(err)
	}
}

func (worker *Worker) get(key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	return worker.cli.Get(ctx, key, opts...)
}

func (worker *Worker) watch(key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	return worker.cli.Watch(context.Background(), key, opts...)
}

func (worker *Worker) delete(key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	return worker.cli.Delete(ctx, key, opts...)
}

func (worker *Worker) Close() {
	worker.delete(worker.node())
	worker.cli.Close()
	close(worker.jobChan)
	worker.closed <- true
}

func (worker *Worker) getAssignedDir() string {
	return "assign/" + worker.name + "/"
}

func (worker *Worker) getAssignedJobKey(jobId string) string {
	return worker.getAssignedDir() + jobId
}

func (worker *Worker) runJob(jobId string) {
	log.Println("Run job", jobId)
	// get the job command by job Id
	jobIdNodeKey := "job/" + jobId
	resp, err := worker.get(jobIdNodeKey)
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
	jobStateFromWorker := jobIdNodeKey + "/state/" + worker.name
	worker.put(jobStateFromWorker, JOB_STATUS_RUNNING)

	// run job: execute command
	cmd := exec.Command(command, args...)
	log.Print("Run command: ", command)
	for _, arg := range args {
		log.Print(" ", arg)
	}
	log.Println()

	//TODO: use cmd.Start() to async execute, and periodly update the stdout and stderr to node
	// job/<jobid>/state/<worker>/stdout and stderr, so that user can known its running status.
	byteBuf, err := cmd.CombinedOutput()
	jobOutput := string(byteBuf)
	log.Println(jobOutput)

	// set job output
	worker.put(jobStateFromWorker+"/stdouterr", jobOutput)

	if err != nil {
		// if error (exit value is not 0), set job state fail
		log.Println(err)
		worker.put(jobStateFromWorker, JOB_STATUS_FAIL)
	} else {
		// set job state done
		worker.put(jobStateFromWorker, JOB_STATUS_DONE)
	}
	// remove the assign job assign to the worker, as it is not running or pending by the worker
	worker.delete(worker.getAssignedJobKey(jobId))
}

func (worker *Worker) node() string {
	return "worker/" + worker.name
}

func (worker *Worker) Exists() (bool, error) {
	resp, err := worker.get(worker.node())
	if err != nil {
		return false, err
	}
	return len(resp.Kvs) > 0, nil
}

func (worker *Worker) Name() string {
	return worker.name
}

func (worker *Worker) Register() error {
	// register
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	grantResp, err := worker.cli.Grant(ctx, 5)
	cancel()
	if err != nil {
		return err
	}

	workerNodeKey := worker.node()
	worker.put(workerNodeKey, "", clientv3.WithLease(grantResp.ID))
	log.Println("Worker", worker.name, "has registered to", workerNodeKey)

	// beat heart
	go func(id clientv3.LeaseID) {
		for {
			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			_, kaerr := worker.cli.KeepAliveOnce(ctx, id)
			cancel()
			if kaerr != nil {
				log.Println(kaerr)
			}
			time.Sleep(2 * time.Second)
		}
	}(grantResp.ID)

	// listen to the worker node itself: write "close" means close the worker,
	// if receive DELETE event may be network timeout issue and may be reconnect? .
	go func() {
		rch := worker.watch(workerNodeKey, clientv3.WithPrefix())
		for wresp := range rch {
			for _, ev := range wresp.Events {
				switch ev.Type.String() {
				case "PUT":
					// "close" means close the worker
					if "close" == string(ev.Kv.Value) {
						log.Println("Get close signal, worker", worker.name, "now is closing")
						worker.Close()
					}
				case "DELETE":
					log.Println("Worker", worker.name, "is deleted.")
					//TODO: reconnect?
				default:
					log.Printf("Unknown event type %s (%q : %q)\n", ev.Type, ev.Kv.Key, ev.Kv.Value)
				}
			}
		}
	}()

	return nil
}

// when starting/restarting worker, get the works that already existed and run
func (worker *Worker) RunJobsAssigned() error {
	// get the node assign/<worker>/*
	assignNodeKey := worker.getAssignedDir()
	lenAssignNodeKey := len(assignNodeKey)

	resp, err := worker.get(assignNodeKey, clientv3.WithPrefix())
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
			worker.jobChan <- jobId
		}
	}(jobs)
	return err
}

// ready to run job parellel
func (worker *Worker) prepareForRun() {
	for i := 0; i < worker.parellelCount; i++ {
		go func() {
			for jobId := range worker.jobChan {
				worker.runJob(jobId)
			}
		}()
	}
}

// listen to the work assigned
func (worker *Worker) ListenNewJobAssigned() {
	go func() {
		assignNodeKey := worker.getAssignedDir()
		lenAssignNodeKey := len(assignNodeKey)
		log.Println("Worker", worker.Name(), "is ready for work on node", assignNodeKey)
		rch := worker.watch(assignNodeKey, clientv3.WithPrefix())
		for wresp := range rch {
			for _, ev := range wresp.Events {
				switch ev.Type.String() {
				case "PUT":
					// get the job id
					assignJobKey := string(ev.Kv.Key)
					jobId := assignJobKey[lenAssignNodeKey:]
					worker.jobChan <- jobId
				case "DELETE":
					// do nothing
				default:
					log.Printf("Unknown event type %s (%q : %q)\n", ev.Type, ev.Kv.Key, ev.Kv.Value)
				}
			}
		}
	}()
}

// test:
// etcdctl put worker/wenzhe
// etcdctl put job/py10 '["python", "-c", "import time; import sys; print 123; time.sleep(10); print 456; sys.exit(0)"]'
// etcdctl put job/1 '["gotest"]'
// etcdctl put assign/wenzhe/py10 ''
// etcdctl put assign/wenzhe/1 ''

func main() {
	// get argument
	flag.Parse()
	name := flag.Arg(0)
	parellelCount, err := strconv.Atoi(flag.Arg(1))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Hello GoCRMS worker", name)

	// connect and create worker
	worker, err := NewWorker(name, parellelCount, endpoints, dialTimeout, requestTimeout)
	if err != nil {
		log.Fatal(err)
	}

	// check existance
	existed, err := worker.Exists()
	if err != nil {
		log.Fatal(err)
	}
	if existed {
		log.Fatal("Worker ", worker.Name(), " has already existed.")
	}

	// register
	if err = worker.Register(); err != nil {
		log.Fatal(err)
	}

	// listen to the work assigned
	worker.ListenNewJobAssigned()

	// when starting/restarting worker, get the works that already existed and run
	if err = worker.RunJobsAssigned(); err != nil {
		log.Println(err)
	}

	// wait for worker close
	<-worker.closed
}
