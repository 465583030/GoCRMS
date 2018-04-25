package gocrms

import (
	"testing"
	"github.com/coreos/etcd/clientv3"
	"time"
	"strconv"
	"fmt"
	"os"
	"path"
	"io/ioutil"
	"log"
)

func TestCrmsd(t *testing.T) {
	// set data dir as test
	DataDir = path.Join(os.TempDir(), ".gocrms")

	// remove if data dir exists
	_, err := os.Stat(DataDir)
	if err == nil || !os.IsNotExist(err) {
		err = os.RemoveAll(DataDir)
		if err != nil {
			t.Fatal(err)
		}
	}
	// remove data dir after test
	defer os.RemoveAll(DataDir)

	serverName := "s1"
	MkDataDir()
	logFile := InitServerLog(serverName, log.Ldate | log.Lmicroseconds, false)
	defer logFile.Close()

	crmsd, err := NewCrmsServer(clientv3.Config{
		Endpoints: []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	}, 10 * time.Second, Server{
		Name: serverName,
		SlotCount: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer crmsd.Close()

	if err = crmsd.crms.Reset(); err != nil {
		t.Fatal(err)
	}

	// get nodes, should be empty
	nodes, err := crmsd.crms.Nodes()
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 0 {
		t.Fatal(nodes)
	}
	// now reset works, defer reset so after test data can clean
	defer crmsd.crms.Reset()

	// assign a job before server start, when server starts, it will be run immediately
	crmsd.crms.CreateJob(&Job{
		ID:      "pre",
		Command: []string{"python", "-c", "import sys,time; print 'pre'; time.sleep(2); print 'after'; sys.exit(0)"},
	})
	crmsd.crms.RunJob("pre", serverName)

	if err := crmsd.Start(); err != nil {
		t.Fatal(err)
	}

	//etcdEvtLogFile, err := os.Create(path.Join(logDir(), "etcdevt.log"))
	//if err != nil {
	//	t.Fatal(err)
	//}
	//defer etcdEvtLogFile.Close()
	//etcdEvtLog := log.New(etcdEvtLogFile, "", log.Ldate | log.Lmicroseconds)

	// watch crms/ evt
	wch, cancel := crmsd.crms.etcd.WatchWithPrefix("crms/")
	defer cancel()
	go HandleWatchEvt(wch, KVHandlerFactory(WatchFunc{
		HandlePut: func(k, v string) {
			log.Printf("etcd put (%s, %s)\n", k, v)
		},
		HandleDelete: func(k, v string) {
			log.Printf("etcd del (%s, %s)\n", k, v)
		},
	}))

	// assign some jobs
	go func(crms *Crms, server string) {
		time.Sleep(time.Second) // sleep to know the pre-assign job run first
		sleepSeconds := [...]int{3, 0, 1, 1, 2, 0}
		const nJob = len(sleepSeconds)
		for i := 0; i < nJob; i++ {
			id := strconv.Itoa(i)
			exitValue := i % 2
			crms.CreateJob(&Job{
				ID:      id,
				Command: []string{"python", "-c", fmt.Sprintf(
					"import sys,time; print %d; time.sleep(%d); print %d; sys.exit(%d)",
					i, sleepSeconds[i], -i, exitValue)},
			})
			crms.RunJob(id, server)
		}
		time.Sleep(7 * time.Second)
		crms.StopServer(server)
	}(crmsd.crms, crmsd.server.Name)

	crmsd.WaitUntilClose()
	// logFile.Sync()  // -- seems no need to call it to flush file, Write() doesn't use buffer

	// validate the out file
	if content, err := ioutil.ReadFile(path.Join(joboutDir(), "pre")); err != nil {
		t.Error(err)
	} else if equalsSkipLF(content, []byte("pre\nafter\n")) {
		t.Error(string(content))
	}

	if content, err := ioutil.ReadFile(path.Join(joboutDir(), "5")); err != nil {
		t.Error(err)
	} else if equalsSkipLF(content, []byte("5\n-5\n")) {
		t.Error(string(content))
	}

	// validate the log file
	if content, err := ioutil.ReadFile(path.Join(logDir(), "s1.log")); err != nil {
		t.Error(err)
	} else {
		err := EqLog(string(content), `
2018/04/24 16:24:00.601202 Slot 0 is assigned to Job pre
2018/04/24 16:24:00.602202 etcd put (crms/jobstate/pre, running)
2018/04/24 16:24:00.609202 Run Job pre with command [python -c import sys,time; print 'pre'; time.sleep(2); print 'after'; sys.exit(0)]
2018/04/24 16:24:01.602202 etcd put (crms/job/0, ["python","-c","import sys,time; print 0; time.sleep(3); print 0; sys.exit(0)"])
2018/04/24 16:24:01.610202 etcd put (crms/assign/s1/0, )
2018/04/24 16:24:01.610202 Slot 1 is assigned to Job 0
2018/04/24 16:24:01.614202 etcd put (crms/job/1, ["python","-c","import sys,time; print 1; time.sleep(0); print -1; sys.exit(1)"])
2018/04/24 16:24:01.617202 etcd put (crms/assign/s1/1, )
2018/04/24 16:24:01.620202 etcd put (crms/jobstate/0, running)
2018/04/24 16:24:01.620202 etcd put (crms/job/2, ["python","-c","import sys,time; print 2; time.sleep(1); print -2; sys.exit(0)"])
2018/04/24 16:24:01.624202 etcd put (crms/assign/s1/2, )
2018/04/24 16:24:01.629202 Run Job 0 with command [python -c import sys,time; print 0; time.sleep(3); print 0; sys.exit(0)]
2018/04/24 16:24:01.637202 etcd put (crms/job/3, ["python","-c","import sys,time; print 3; time.sleep(1); print -3; sys.exit(1)"])
2018/04/24 16:24:01.638202 etcd put (crms/assign/s1/3, )
2018/04/24 16:24:01.640202 etcd put (crms/job/4, ["python","-c","import sys,time; print 4; time.sleep(2); print -4; sys.exit(0)"])
2018/04/24 16:24:01.643202 etcd put (crms/assign/s1/4, )
2018/04/24 16:24:01.646202 etcd put (crms/job/5, ["python","-c","import sys,time; print 5; time.sleep(0); print -5; sys.exit(1)"])
2018/04/24 16:24:01.649202 etcd put (crms/assign/s1/5, )
2018/04/24 16:24:02.858202 Success to run job pre
2018/04/24 16:24:02.861202 etcd put (crms/jobstate/pre, done)
2018/04/24 16:24:02.875202 Slot 0 is assigned to Job 1
2018/04/24 16:24:02.876202 etcd del (crms/assign/s1/pre, )
2018/04/24 16:24:02.879202 etcd put (crms/jobstate/1, running)
2018/04/24 16:24:02.886202 Run Job 1 with command [python -c import sys,time; print 1; time.sleep(0); print -1; sys.exit(1)]
2018/04/24 16:24:03.117202 Fail to run job 1, reason: exit status 1
2018/04/24 16:24:03.118202 etcd put (crms/jobstate/1, fail)
2018/04/24 16:24:03.124202 Slot 0 is assigned to Job 2
2018/04/24 16:24:03.124202 etcd del (crms/assign/s1/1, )
2018/04/24 16:24:03.128202 etcd put (crms/jobstate/2, running)
2018/04/24 16:24:03.135202 Run Job 2 with command [python -c import sys,time; print 2; time.sleep(1); print -2; sys.exit(0)]
2018/04/24 16:24:04.336202 Success to run job 2
2018/04/24 16:24:04.339202 etcd put (crms/jobstate/2, done)
2018/04/24 16:24:04.343202 Slot 0 is assigned to Job 3
2018/04/24 16:24:04.343202 etcd del (crms/assign/s1/2, )
2018/04/24 16:24:04.347202 etcd put (crms/jobstate/3, running)
2018/04/24 16:24:04.359202 Run Job 3 with command [python -c import sys,time; print 3; time.sleep(1); print -3; sys.exit(1)]
2018/04/24 16:24:04.861202 Success to run job 0
2018/04/24 16:24:04.862202 etcd put (crms/jobstate/0, done)
2018/04/24 16:24:04.867202 Slot 1 is assigned to Job 4
2018/04/24 16:24:04.867202 etcd del (crms/assign/s1/0, )
2018/04/24 16:24:04.872202 etcd put (crms/jobstate/4, running)
2018/04/24 16:24:04.886202 Run Job 4 with command [python -c import sys,time; print 4; time.sleep(2); print -4; sys.exit(0)]
2018/04/24 16:24:05.576202 Fail to run job 3, reason: exit status 1
2018/04/24 16:24:05.578202 etcd put (crms/jobstate/3, fail)
2018/04/24 16:24:05.581202 Slot 0 is assigned to Job 5
2018/04/24 16:24:05.582202 etcd del (crms/assign/s1/3, )
2018/04/24 16:24:05.587202 etcd put (crms/jobstate/5, running)
2018/04/24 16:24:05.601202 Run Job 5 with command [python -c import sys,time; print 5; time.sleep(0); print -5; sys.exit(1)]
2018/04/24 16:24:05.825202 Fail to run job 5, reason: exit status 1
2018/04/24 16:24:05.829202 etcd put (crms/jobstate/5, fail)
2018/04/24 16:24:05.838202 etcd del (crms/assign/s1/5, )
2018/04/24 16:24:07.117202 Success to run job 4
2018/04/24 16:24:07.118202 etcd put (crms/jobstate/4, done)
2018/04/24 16:24:07.131202 etcd del (crms/assign/s1/4, )
2018/04/24 16:24:08.652202 etcd put (crms/server/s1, close)

`, 200 * time.Millisecond)
		if err != nil {
			t.Error(err, string(content))
		}
	}
}

func equalsSkipLF(actual []byte, expected []byte) bool {
	n := len(expected)
	j := 0
	for i, act := range actual {
		if i >= n {
			return false
		}
		var exp byte
		for exp = expected[j]; exp == '\r'; j++ {
			if j >= n - 1 {
				return false
			}
		}
		if act == exp {
			j++
		} else if act != '\r' {
			return false
		}
	}
	return true
}
