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
	"github.com/WenzheLiu/GoCRMS/ut"
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

	// watch crms/ evt
	wch, cancel := crmsd.crms.etcd.WatchWithPrefix("crms/")
	defer cancel()
	go HandleWatchEvt(wch, KVHandlerFactory(WatchFunc{
		HandlePut: func(k, v string) {
			log.Printf("etcd on put: (%s, %s)\n", k, v)
		},
		HandleDelete: func(k string) {
			log.Printf("etcd on del: %s\n", k)
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
			crms.AssignJob(id, server)
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
		err := ut.EqLogWithGolden(ut.DefaultGoldenFile(), string(content), 300 * time.Millisecond)
		if err != nil {
			t.Error(err)
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
