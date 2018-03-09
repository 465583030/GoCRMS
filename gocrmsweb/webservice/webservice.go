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

package webservice

import (
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/WenzheLiu/GoCRMS/gocrmscli"
)

type Server struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Status      string `json:"status"`
	IsReachable bool   `json:"isReachable"`
}

type Worker struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type Job struct {
	Uuid    string   `json:"uuid"`
	Command []string `json:"command"`
}

type JobDetail struct {
	Job    Job    `json:"job"`
	Status string `json:"status"`
	//StartTime string `json:"startTime"`
	//EndTime string `json:"endTime"`
	StdOutErr string `json:"stdouterr"`
}

type RunCommand struct {
	Command   []string `json:"job"`
	HostPorts []string `json:"hostPorts"`
}

type WebService struct {
	crms *gocrmscli.CrmsCli
}

const (
	dialTimeout    = 5 * time.Second
	requestTimeout = 10 * time.Second
)

func NewWebService(endpoints []string) *WebService {
	var err error
	crms, err := gocrmscli.New(endpoints, dialTimeout, requestTimeout)
	if err != nil {
		log.Fatal(err)
	}
	return &WebService{crms}
}

func (ws *WebService) GetServers() (servers []Server, err error) {
	workers, err := ws.crms.GetWorkers()
	if err != nil {
		return
	}
	servers = make([]Server, len(workers))
	i := 0
	for name, _ := range workers {
		servers[i] = Server{name, 0, "new", true}
		i++
	}
	return
}

//TODO: rename: here worker is actually the parallel unit inside host, and host is actually worker
func (ws *WebService) GetWorkers(host string) ([]Worker, error) {
	worker, exist, err := ws.crms.GetWorker(host)
	if err != nil {
		return nil, err
	}
	if !exist {
		return nil, nil
	}
	wks := make([]Worker, worker.ParellelAbility)
	i := 0
	for jobId, _ := range worker.Jobs {
		// get status
		job, exist, err := ws.crms.GetJob(jobId)
		var status string
		if err != nil {
			log.Println(err)
			status = "unknown"
		} else if !exist {
			status = "new"
		} else {
			status = job.GetStatus()
		}
		if i < worker.ParellelAbility {
			wks[i] = Worker{host + " " + strconv.Itoa(i+1), status}
			i++
		} else {
			break
		}
	}
	for ; i < worker.ParellelAbility; i++ {
		wks[i] = Worker{host + " " + strconv.Itoa(i+1), "idle"}
	}
	return wks, nil
}

func (ws *WebService) GetJobs() (jobs []JobDetail, err error) {
	jobMap, err := ws.crms.GetJobs()
	if err != nil {
		return
	}
	jobs = make([]JobDetail, 0, len(jobMap))
	for _, job := range jobMap {
		jobs = append(jobs, toJobDetail(job))
	}
	return
}

func (ws *WebService) GetJobsByWorker(workerName string) (jobs []JobDetail, err error) {
	js, err := ws.crms.GetJobsByWorker(workerName)
	if err != nil {
		return
	}
	jobs = make([]JobDetail, 0, len(js))
	for _, job := range js {
		jobs = append(jobs, toJobDetail(job))
	}
	return
}

func toJobDetail(job *gocrmscli.Job) JobDetail {
	return JobDetail{
		Job: Job{
			Uuid:    job.ID,
			Command: job.Command,
		},
		Status:    job.GetStatus(),
		StdOutErr: job.GetStdOutErr(),
	}
}

func (ws *WebService) RunJob(job RunCommand) error {
	jobId := strconv.Itoa(rand.Intn(10000))
	err := ws.crms.CreateJob(jobId, job.Command)
	if err != nil {
		return err
	}
	for _, hp := range job.HostPorts {
		worker := strings.Split(hp, ":")[0]
		log.Println("run job", jobId, "on worker", worker)
		err = ws.crms.RunJob(jobId, worker) //TODO: collect all error instead of replace older
		if err != nil {
			log.Println(err)
		}
	}
	return err
}

func (ws *WebService) Shutdown(workers []string) {
	for _, worker := range workers {
		worker = strings.Split(worker, ":")[0]
		if err := ws.crms.StopWorker(worker); err != nil {
			log.Println("Cannot shutdown:", worker, ", reason is:", err.Error())
		}
		log.Println("shutdown:", worker)
	}
}

func (ws *WebService) GetNodes() (map[string]string, error) {
	return ws.crms.GetNodes()
}
