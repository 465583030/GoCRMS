package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
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
			case "PUTtt":
				// get the job id
				jobId, err := strconv.Atoi(string(ev.Kv.Key)[lenAssignNodeKey:])
				if err != nil {
					fmt.Println("Not a valid jobId in", ev.Kv.Key)
					continue
				}
				fmt.Println("Run job", jobId)
				//				var jobIds []int
				//				err = json.Unmarshal(ev.Kv.Value, &jobIds)
				//				if err != nil {
				//					fmt.Println(err)
				//					continue
				//				}
				//				// run jobs
				//				fmt.Printf("Run jobs %v\n", jobIds)
				//				for _, jobId := range jobIds {
				if _, ok := worker.jobs[jobId]; !ok {
					// get the job command by jobId
					jobIdNodeKey := "job/" + strconv.Itoa(jobId)
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
					job := Job{jobId, command, args, JOB_STATUS_PENDING}
					worker.jobs[jobId] = &job
					// check resource to run job (parellel count)

					// run job: execute command
					cmd := exec.Command(command, args...)
					stdout, err := cmd.StdoutPipe()
					if err != nil {
						fmt.Println(err)
						continue
					}
					defer stdout.Close()
					fmt.Print("Run command", command)
					for _, arg := range args {
						fmt.Print(" ", arg)
					}
					fmt.Println()
					if err := cmd.Start(); err != nil {
						fmt.Println(err)
						continue
					}
					opBytes, err := ioutil.ReadAll(stdout)
					if err != nil {
						fmt.Println(err)
						continue
					}
					fmt.Println(string(opBytes))
				}

				//			case "DELETE":
				//				fmt.Println("Shutdown worker", name)
				//				os.Exit(0)
			default:
				fmt.Printf("Unknown event type %s (%q : %q)\n", ev.Type, ev.Kv.Key, ev.Kv.Value)
			}
		}
	}
}
