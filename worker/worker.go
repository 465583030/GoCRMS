package worker

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
	JOB_STATUS_RUNNING = "running"
	JOB_STATUS_DONE    = "done"
	JOB_STATUS_FAIL    = "fail"

	WATCH_EVT_PUT    = 0
	WATCH_EVT_DELETE = 1

	TIME_OUT_WORKER     = 5 // second
	TIME_OUT_BEAT_HEART = 2 * time.Second
)

type Worker struct {
	name           string
	parellelCount  int
	cli            *clientv3.Client
	requestTimeout time.Duration
	jobChan        chan string
	closed         chan bool
	onClose        []func()
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
		make(chan string, parellelCount), make(chan bool), make([]func(), 0)}

	// ready to run job parellel
	worker.prepareForRun()
	return worker, err
}

func (worker *Worker) put(key, val string, opts ...clientv3.OpOption) error {
	ctx, cancel := context.WithTimeout(context.Background(), worker.requestTimeout)
	defer cancel()
	_, err := worker.cli.Put(ctx, key, val, opts...)
	if err != nil {
		log.Println(err)
	}
	return err
}

func (worker *Worker) get(key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), worker.requestTimeout)
	defer cancel()
	return worker.cli.Get(ctx, key, opts...)
}

func (worker *Worker) watch(key string, opts ...clientv3.OpOption) (clientv3.WatchChan, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	return worker.cli.Watch(ctx, key, opts...), cancel
}

func (worker *Worker) delete(key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), worker.requestTimeout)
	defer cancel()
	return worker.cli.Delete(ctx, key, opts...)
}

func (worker *Worker) Close() {
	for _, fn := range worker.onClose {
		fn()
	}
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
	stdouterr := jobStateFromWorker + "/stdouterr"
	worker.delete(stdouterr)
	worker.put(jobStateFromWorker, JOB_STATUS_RUNNING)

	// run job: execute command
	cmd := exec.Command(command, args...)

	log.Println("Run command:", command, strings.Join(args, " "))

	//TODO: use cmd.Start() to async execute, and periodly update the stdout and stderr to node
	// job/<jobid>/state/<worker>/stdout and stderr, so that user can known its running status.
	byteBuf, err := cmd.CombinedOutput()
	jobOutput := string(byteBuf)
	//log.Print("\n", jobOutput)

	// set job output
	worker.put(stdouterr, jobOutput)

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
	ctx, cancel := context.WithTimeout(context.Background(), worker.requestTimeout)
	grantResp, err := worker.cli.Grant(ctx, TIME_OUT_WORKER)
	cancel()
	if err != nil {
		return err
	}

	workerNodeKey := worker.node()
	worker.put(workerNodeKey, strconv.Itoa(worker.parellelCount), clientv3.WithLease(grantResp.ID))
	log.Println("Worker", worker.name, "has registered to", workerNodeKey)

	// beat heart
	go func(id clientv3.LeaseID) {
		for {
			ctx, cancel := context.WithTimeout(context.Background(), worker.requestTimeout)
			_, kaerr := worker.cli.KeepAliveOnce(ctx, id)
			cancel()
			if kaerr != nil {
				log.Println(kaerr)
			}
			time.Sleep(TIME_OUT_BEAT_HEART)
		}
	}(grantResp.ID)

	// listen to the worker node itself: write "close" means close the worker,
	// if receive DELETE event may be network timeout issue and may be reconnect? .
	go func() {
		rch, cancel := worker.watch(workerNodeKey, clientv3.WithPrefix())
		worker.deferOnClose(cancel)
		for wresp := range rch {
			for _, ev := range wresp.Events {
				switch ev.Type {
				case WATCH_EVT_PUT:
					// "close" means close the worker
					if "close" == string(ev.Kv.Value) {
						log.Println("Get close signal, worker", worker.name, "now is closing")
						worker.Close()
					}
				case WATCH_EVT_DELETE:
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

func (worker *Worker) deferOnClose(fn func()) {
	worker.onClose = append(worker.onClose, fn) //TODO: may not thread safe
}

// listen to the work assigned
func (worker *Worker) ListenNewJobAssigned() {
	go func() {
		assignNodeKey := worker.getAssignedDir()
		lenAssignNodeKey := len(assignNodeKey)
		log.Println("Worker", worker.Name(), "is ready for work on node", assignNodeKey)
		rch, cancel := worker.watch(assignNodeKey, clientv3.WithPrefix())
		worker.deferOnClose(cancel)

		for wresp := range rch {
			for _, ev := range wresp.Events {
				switch ev.Type {
				case WATCH_EVT_PUT:
					// get the job id
					assignJobKey := string(ev.Kv.Key)
					jobId := assignJobKey[lenAssignNodeKey:]
					worker.jobChan <- jobId
				case WATCH_EVT_DELETE:
					// do nothing
				default:
					log.Printf("Unknown event type %s (%q : %q)\n", ev.Type, ev.Kv.Key, ev.Kv.Value)
				}
			}
		}
	}()
}

// wait until close
func (worker *Worker) WaitUntilClose() {
	<-worker.closed
}
