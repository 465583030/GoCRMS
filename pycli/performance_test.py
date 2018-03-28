# Copyright 2018 Wenzhe Liu (liuwenzhe2008@qq.com)
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Usage: python performance_test.py <num_of_jobs> [work_second=0] [worker_count=1] [etcd_host:port=localhost:2379]
#

import crmscli
import random
import os.path
import logging
import sys
import threading
import time
import subprocess
from datetime import datetime

LOG_PATH = "~/.gocrms/performance.log"


def average(costs):
    '''
    average
    :param costs len(costs) should > 0
    '''
    cost = costs[0]
    for c in costs[1:]:
        cost += c
    return cost / len(costs)


def find_job_id(s):
    i = s.find(' ')
    if i == -1:
        return s
    else:
        return s[:i]


class CountDownLatch(object):
    def __init__(self, count=1):
        self.count = count
        self.lock = threading.Condition()

    def count_down(self):
        self.lock.acquire()
        self.count -= 1
        if self.count <= 0:
            self.lock.notifyAll()
        self.lock.release()

    def await(self):
        self.lock.acquire()
        while self.count > 0:
            self.lock.wait()
        self.lock.release()


class JobTime(object):
    def __init__(self):
        self.start_on_client = None
        self.start_on_worker = None
        self.end_on_worker = None
        self.end_on_client = None
        self.worker = ""

    def start_cost(self):
        self.__check_none()
        return self.start_on_worker - self.start_on_client

    def end_cost(self):
        self.__check_none()
        return self.end_on_client - self.end_on_worker

    def __check_none(self):
        if None in [self.start_on_client, self.start_on_worker, self.end_on_worker, self.end_on_client]:
            print 'JobTime contains None field:', self.__dict__


class Summary(object):
    def __init__(self):
        self.workers = []
        self.start_time = datetime.now()
        self.jobs_time = {}

    def get_job(self, job_id):
        if self.jobs_time.has_key(job_id):
            return self.jobs_time[job_id]
        else:
            jt = JobTime()
            self.jobs_time[job_id] = jt
            return jt

    def parse_worker_log(self, worker):
        '''
        the worker's log format is like this:
        fnode400 2018/03/23 11:42:44.374266 worker.go:150: Run Job 1 with command: python -c print 1
        fnode401 2018/03/23 11:42:45.005266 worker.go:156: Finish Job 0 with result:  0
        '''
        RUN_JOB = 'Run Job '
        LEN_RUN = len(RUN_JOB)
        FINISH_JOB = 'Finish Job '
        LEN_FINISH = len(FINISH_JOB)

        logfile = os.path.expanduser('~/.gocrms/%s.log' % worker)
        with open(logfile) as f:
            for line in f:
                tags = line.split(' ', 4)
                if len(tags) < 5:
                    continue
                try:
                    dt = datetime.strptime(' '.join(tags[1:3]), '%Y/%m/%d %H:%M:%S.%f')
                except ValueError:
                    continue
                if dt <= self.start_time:
                    continue
                content = tags[4]
                if content.startswith(RUN_JOB):
                    job_id = find_job_id(content[LEN_RUN:])
                    self.get_job(job_id).worker = worker
                    self.get_job(job_id).start_on_worker = dt
                elif content.startswith(FINISH_JOB):
                    job_id = find_job_id(content[LEN_FINISH:])
                    self.get_job(job_id).worker = worker
                    self.get_job(job_id).end_on_worker = dt

    def parse_client_log(self):
        '''
        log format:
        2018-03-23 10:13:37,927 run job 1 on worker w1
        2018-03-23 10:13:38,269 finish job 0 with result 0
        '''
        RUN_JOB = 'run job '
        LEN_RUN = len(RUN_JOB)
        FINISH_JOB = 'finish job '
        LEN_FINISH = len(FINISH_JOB)

        logfile = os.path.expanduser(LOG_PATH)
        with open(logfile) as f:
            for line in f:
                tags = line.split(' ', 2)
                if len(tags) < 3:
                    continue
                try:
                    dt = datetime.strptime(' '.join(tags[:2]), '%Y-%m-%d %H:%M:%S,%f')
                except ValueError:
                    continue
                if dt <= self.start_time:
                    continue
                content = tags[2]
                if content.startswith(RUN_JOB):
                    job_id = find_job_id(content[LEN_RUN:])
                    self.get_job(job_id).start_on_client = dt
                elif content.startswith(FINISH_JOB):
                    job_id = find_job_id(content[LEN_FINISH:])
                    self.get_job(job_id).end_on_client = dt

    def parse_log(self):
        self.parse_client_log()
        for worker in summary.workers:
            self.parse_worker_log(worker)

    def average_start_cost(self):
        costs = [jt.start_cost() for jt in self.jobs_time.values()]
        return average(costs)

    def average_end_cost(self):
        costs = [jt.end_cost() for jt in self.jobs_time.values()]
        return average(costs)

    def __average_workers_cost(self, fn):
        worker_cost_map = {}
        for jt in self.jobs_time.values():
            if not worker_cost_map.has_key(jt.worker):
                worker_cost_map[jt.worker] = []
            worker_cost_map[jt.worker].append(fn(jt))
        for worker in worker_cost_map.keys():
            worker_cost_map[worker] = average(worker_cost_map[worker])
        return worker_cost_map

    def average_workers_start_cost(self):
        return self.__average_workers_cost(lambda jt: jt.start_cost())

    def average_workers_end_cost(self):
        return self.__average_workers_cost(lambda jt: jt.end_cost())

    def first_start_on_client(self):
        return min([jt.start_on_client for jt in self.jobs_time.values()])

    def first_start_on_worker(self):
        return min([jt.start_on_worker for jt in self.jobs_time.values()])

    def last_start_on_client(self):
        return max([jt.start_on_client for jt in self.jobs_time.values()])

    def last_start_on_worker(self):
        return max([jt.start_on_worker for jt in self.jobs_time.values()])

    def first_end_on_worker(self):
        return min([jt.end_on_worker for jt in self.jobs_time.values()])

    def first_end_on_client(self):
        return min([jt.end_on_client for jt in self.jobs_time.values()])

    def last_end_on_worker(self):
        return max([jt.end_on_worker for jt in self.jobs_time.values()])

    def last_end_on_client(self):
        return max([jt.end_on_client for jt in self.jobs_time.values()])

    def report(self):
        fmt = '%-6s | %-8s | %-14s | %-14s | %-15s | %-15s | %-15s | %-15s'
        print fmt % (
            'job id', 'worker', 'start cost', 'end cost',
            'start on client', 'start on worker', 'end on worker', 'end on client'
        )
        for job_id, jt in sorted(self.jobs_time.items()):
            print fmt % (
                job_id, jt.worker, jt.start_cost(), jt.end_cost(),
                jt.start_on_client.time(), jt.start_on_worker.time(),
                jt.end_on_worker.time(), jt.end_on_client.time()
            )

        print 'average start/end cost for each worker:'
        fmt = '%-8s | %-14s | %-14s'
        print fmt % (
            'worker', 'avr start cost', 'avr end cost'
        )
        average_workers_start_cost = self.average_workers_start_cost()
        average_workers_end_cost = self.average_workers_end_cost()
        for worker, start_cost in average_workers_start_cost.items():
            end_cost = average_workers_end_cost[worker]
            print fmt % (
                worker, start_cost, end_cost
            )

        print 'average start cost:', self.average_start_cost()
        print 'average  end  cost:', self.average_end_cost()

        first_start_on_client = self.first_start_on_client()
        # first_start_on_worker = self.first_start_on_worker()
        # last_start_on_client = self.last_start_on_client()
        last_start_on_worker = self.last_start_on_worker()
        # first_end_on_worker = self.first_end_on_worker()
        # first_end_on_client = self.first_end_on_client()
        # last_end_on_worker = self.last_end_on_worker()
        # last_end_on_client = self.last_end_on_client()
        # print 'first start on client', first_start_on_client
        # print 'first start on worker', first_start_on_worker
        # print 'last start on client', last_start_on_client
        # print 'last start on worker', last_start_on_worker

        print 'total start cost (last start on worker - first start on client)', \
            last_start_on_worker - first_start_on_client


logger = logging.getLogger("crms")
jobcount = int(sys.argv[1])
jobs_finished_count = CountDownLatch(jobcount)
summary = Summary()


def on_job_status_changed(job):
    job_state = job.get_state()
    # print "job", job.id, "status change to", job_state.status
    if job_state.status in ['done', 'fail']:
        logger.info('finish job %s with result %s', job.id, job_state.stdouterr)
        jobs_finished_count.count_down()


def test_run_job(job_count):
    if len(sys.argv) < 3:
        work_time = 0
    else:
        work_time = sys.argv[2]

    if len(sys.argv) < 4:
        worker_count = 1
    else:
        worker_count = int(sys.argv[3])

    if len(sys.argv) < 5:
        host_port = 'localhost:2379'
    else:
        host_port = sys.argv[4]

    available_workers = get_available_workers(worker_count)
    print 'available workers:', available_workers

    start_workers(host_port, available_workers)

    print 'connect etcd', host_port
    with crmscli.CrmsCli(host_port) as crms:
        crms.add_watcher(on_job_status_changed)

        crms.clean()

        wait_for_workers_register(crms, available_workers)

        workers = crms.get_workers().keys()
        print "workers:", workers
        if len(workers) == 0:
            sys.exit(0)
        summary.workers = workers
        for i in xrange(job_count):
            # f = os.path.join(os.path.dirname(__file__), 'mock_job.py')
            job_id = str(i)
            logger.info('create job %s', job_id)
            crms.create_job(job_id, ['python', '-c', 'import time; time.sleep(%s); print %s' %(work_time, job_id)])
            worker = random.choice(workers)
            logger.info('run job %s on worker %s', job_id, worker)
            crms.run_job(job_id, worker)

        jobs_finished_count.await()
        print_nodes(crms.nodes())

        stop_workers(crms)


def get_available_workers(worker_count):
    '''
    fnode091      up 375+20:58,     0 users,  load  0.02,  0.14,  0.45
    fnode092      up 403+23:21,     0 users,  load  1.92,  2.14,  1.92
    '''
    p = subprocess.Popen("ruptime | grep 'fnode.*up'", shell=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    cnt = 0
    workers = []
    for line in p.stdout.readlines():
        workers.append(line.split(' ')[0])
        cnt += 1
        if cnt >= worker_count:
            break
    p.wait()
    return workers


def start_workers(etcd_host_port, workers):
    # workers = ['fnode400', 'fnode401']
    for worker in workers:
        print 'start worker', worker
        crmscli.start_worker(worker, worker, 100, etcd_host_port)


def wait_for_workers_register(crms, workers):
    try_count = 0
    while try_count < 10 and len(crms.get_workers()) < len(workers):
        try_count += 1
        time.sleep(2)


def stop_workers(crms):
    for worker in crms.get_workers().keys():
        crms.stop_worker(worker)
    time.sleep(1)  # sleep 1s so that workers have time to flush log (when close log file)


def print_nodes(nodes):
    print("nodes:")
    for (k, v) in sorted(nodes.items()):
        print k, ":", v


def init_log():
    formatter = logging.Formatter('%(asctime)s %(message)s', )
    logfile = os.path.expanduser(LOG_PATH)
    file_handler = logging.FileHandler(logfile)
    file_handler.setFormatter(formatter)
    stream_handler = logging.StreamHandler(sys.stderr)
    logger.addHandler(file_handler)
    logger.addHandler(stream_handler)
    logger.setLevel(logging.INFO)
    return file_handler


def main():
    print "start performance test for GoCRMS"
    file_handler = init_log()
    test_run_job(jobcount)
    file_handler.close() # close log file to flush

    print "Summary:"
    summary.parse_log()
    summary.report()


if __name__ == "__main__":
    main()
