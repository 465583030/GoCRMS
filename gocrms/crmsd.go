package gocrms

import (
	"log"
	"errors"
	"fmt"
	"os/exec"
	"os"
	"github.com/coreos/etcd/clientv3"
	"time"
	"path"
)

type CrmsServer struct {
	crms *Crms
	server Server
	exit chan bool
	slots chan string	// string: job id
}

func NewCrmsServer(cfg clientv3.Config, requestTimeout time.Duration, server Server) (*CrmsServer, error) {
	crms, err := NewCrms(cfg, requestTimeout)
	if err != nil {
		return nil, err
	}
	return &CrmsServer{
		crms: crms,
		server: server,
		exit: make(chan bool),
		slots: make(chan string, server.SlotCount),
	}, nil
}

func (cs *CrmsServer) Start() error {
	if _, exist, err := cs.crms.GetServer(cs.server.Name); err != nil {
		return err
	} else if exist {
		return errors.New(fmt.Sprint("Server", cs.server.Name, "has already existed"))
	}

	leaseID, cancel, err := cs.crms.etcd.Timeout(3)
	if err != nil {
		return err
	}
	defer cancel()
	err = cs.crms.etcd.PutTempNode(cs.server.Name, cs.server.StringValue(), leaseID)
	if err != nil {
		return err
	}
	cs.crms.etcd.KeepAliveForever(leaseID)
	cs.crms.WatchServer(cs.server.Name, ServerWatchFunc{
		HandlePut: func(server *Server) {
			if server.Closed {
				close(cs.exit)
			}
		},
		HandleDelete: func(server *Server) {
			close(cs.exit)
		},
	})

	for i := 0; i < cs.server.SlotCount; i++ {
		go func(slotID int) {
			for jobID := range cs.slots {
				log.Println("Slot", slotID, "is assigned to Job", jobID)
				if err := cs.runJob(jobID); err != nil {
					log.Printf("Fail to run job %s, reason: %v", jobID, err)
					if err := cs.crms.FailJob(jobID, err); err != nil {
						log.Printf("Fail to update job %s state to fail, reason: %v", jobID, err)
					}
				} else {
					log.Println("Success to run job", jobID)
					if err := cs.crms.DoneJob(jobID); err != nil {
						log.Printf("Fail to update job %s state to done, reason: %v", jobID, err)
					}
				}
				// remove the assign job id no matter the
				if err := cs.crms.DeleteAssignJob(cs.server.Name, jobID); err != nil {
					log.Println(err)
				}
			}
		}(i)
	}

	cs.crms.WatchAssignJobs(cs.server.Name, AssignWatchFunc{
		HandlePut: func(jobID string) {
			cs.slots <- jobID
		},
	})
	if jobIDs, err := cs.crms.GetAssignJobs(cs.server.Name); err != nil {
		log.Println(err)
	} else {
		for _, jobID := range jobIDs {
			cs.slots <- jobID
		}
	}

	return nil
}

func (cs *CrmsServer) WaitUntilClose() {
	<-cs.exit
}

func (cs *CrmsServer) Close() {
	cs.crms.Close()
	close(cs.slots)
}
