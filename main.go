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
	name          string
	parellelCount int
	jobs          map[int]*Job
	cli           *clientv3.Client
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

	// connect
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: dialTimeout,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close()
	worker := Worker{name, parellelCount, make(map[int]*Job), cli}

	// check existance
	workerNodeKey := "worker/" + name
	resp, err := worker.get(workerNodeKey)
	if err != nil {
		log.Fatal(err)
	}
	if len(resp.Kvs) > 0 {
		kv := resp.Kvs[0]
		log.Fatal("Worker", name, "has already existed. (", string(kv.Key), ":", string(kv.Value), ")")
	}

	// register
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	grantResp, err := cli.Grant(ctx, 5)
	cancel()
	if err != nil {
		log.Fatal(err)
	}

	worker.put(workerNodeKey, "Work", clientv3.WithLease(grantResp.ID))
	fmt.Println("Worker", name, "has registered to", workerNodeKey)

	// beat heart
	go func(id clientv3.LeaseID) {
		for {
			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			_, kaerr := cli.KeepAliveOnce(ctx, id)
			cancel()
			if kaerr != nil {
				log.Fatal(kaerr)
			}
			time.Sleep(2 * time.Second)
		}
	}(grantResp.ID)

	// listen to work assigned
	assignNodeKey := "assign/" + name + "/"
	lenAssignNodeKey := len(assignNodeKey)
	fmt.Println("Worker", name, "is ready for work on node", assignNodeKey)
	rch := worker.watch(assignNodeKey, clientv3.WithPrefix())
	for wresp := range rch {
		for _, ev := range wresp.Events {
			switch ev.Type.String() {
			case "PUT":
				// get the job id
				assignJobKey := string(ev.Kv.Key)
				jobId := assignJobKey[lenAssignNodeKey:]
				fmt.Println("Run job", jobId)
				// get the job command by job Id
				jobIdNodeKey := "job/" + jobId
				resp, err := worker.get(jobIdNodeKey)
				if err != nil {
					fmt.Println(err)
					continue
				}
				if len(resp.Kvs) == 0 {
					fmt.Println("No job with id", jobId)
					continue
				}
				var cmdWithArgs []string
				err = json.Unmarshal(resp.Kvs[0].Value, &cmdWithArgs)
				if err != nil {
					fmt.Println(err)
					continue
				}
				if len(cmdWithArgs) == 0 {
					fmt.Println("No command in the job", jobId)
					continue
				}
				command := cmdWithArgs[0]
				args := cmdWithArgs[1:]
				// add to job
				//					job := Job{jobId, command, args, JOB_STATUS_PENDING}
				//					worker.jobs[jobId] = &job

				// check resource to run job (parellel count)

				// set job state: running
				jobStateFromWorker := jobIdNodeKey + "/state/" + worker.name
				worker.put(jobStateFromWorker, JOB_STATUS_RUNNING)

				// run job: execute command
				cmd := exec.Command(command, args...)
				fmt.Print("Run command: ", command)
				for _, arg := range args {
					fmt.Print(" ", arg)
				}
				fmt.Println()

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
				worker.delete(assignJobKey)
			case "DELETE":
				// do nothing
			default:
				fmt.Printf("Unknown event type %s (%q : %q)\n", ev.Type, ev.Kv.Key, ev.Kv.Value)
			}
		}
	}
}
