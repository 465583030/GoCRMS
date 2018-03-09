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

package main

import (
	"flag"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/WenzheLiu/GoCRMS/worker"
)

const (
	dialTimeout    = 5 * time.Second
	requestTimeout = 10 * time.Second
)

var (
	endpoints = []string{"localhost:2379"}
)

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
	endpoint := flag.Arg(2)
	if endpoint != "" {
		endpoints = []string{endpoint}
	}

	fmt.Println("Hello GoCRMS worker", name)

	// connect and create worker
	worker, err := worker.NewWorker(name, parellelCount, endpoints, dialTimeout, requestTimeout)
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

	// wait until worker close
	worker.WaitUntilClose()
}
