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

import etcd3
import json
import thread
from etcd3.events import PutEvent
from etcd3.events import DeleteEvent

JOB_PREFIX = 'crms/job/'
WORKER_PREFIX = 'crms/worker/'
ASSIGN_PREFIX = 'crms/assign/'


class JobState(object):
    def __init__(self):
        self.status = 'new'
        self.stdouterr = ''


class Job(object):
    def __init__(self):
        self.id = ''
        self.command = []  # jobCommand is an array of each part of the command
        self.stateOfWorkers = {}  # key: assigned worker name, value: JobState (state + stdout/err)

    def get_state_or_create_if_absent(self, worker_name):
        if not self.stateOfWorkers.has_key(worker_name):
            self.stateOfWorkers[worker_name] = JobState()
        return self.stateOfWorkers[worker_name]

    def get_state(self):
        vs = self.stateOfWorkers.values()
        if len(vs) == 0:
            return None
        else:
            return vs[0]


class CrmsCli(object):
    def __init__(self):
        self.cli = etcd3.client()
        self.__workers = {}  # key: worker name, value: worker parellel job count
        self.__cancelWatchWorkers = None
        self.__jobs = {}  # key: jobId, value: Job
        self.__cancelWatchJobs = None
        self.__onJobStatusChanged = None

        self.get_workers()
        self.get_jobs()

    def add_watcher(self, on_job_status_changed):
        self.get_workers()
        self.get_jobs()

        self.__onJobStatusChanged = on_job_status_changed

    def close(self):
        if self.__cancelWatchWorkers is not None:
            self.__cancelWatchWorkers()
            self.__cancelWatchWorkers = None
        if self.__cancelWatchJobs is not None:
            self.__cancelWatchJobs()
            self.__cancelWatchJobs = None

    def get_workers(self):
        if self.__cancelWatchWorkers is None:  # not watch yet
            self.__workers = self.__get_workers()
            self.__watch_workers()
        return self.__workers

    def __get_workers(self):
        workers = self.cli.get_prefix(WORKER_PREFIX)
        return {wk[1].key[len(WORKER_PREFIX):]: wk[0] for wk in workers}

    def __watch_workers(self):
        evts, self.__cancelWatchWorkers = self.cli.watch_prefix(WORKER_PREFIX)

        def update_workers(evts):
            for evt in evts:
                if isinstance(evt, PutEvent):
                    self.__workers[evt.key[len(WORKER_PREFIX):]] = evt.value
                elif isinstance(evt, DeleteEvent):
                    self.__workers.pop(evt.key[len(WORKER_PREFIX):])

        thread.start_new_thread(update_workers, (evts,))

    def stop_worker(self, name):
        self.cli.put(WORKER_PREFIX + name, 'close')

    def create_job(self, job_id, job_command):  # jobCommand is an array of each part of the command
        cmd = json.dumps(job_command)  # e.g: ['ls', '-l', '..']
        self.cli.put(JOB_PREFIX + job_id, cmd)

    def run_job(self, job_id, worker):
        self.cli.put(ASSIGN_PREFIX + worker + '/' + job_id, '')

    def __get_job_or_create_if_absent(self, job_id):
        if not self.__jobs.has_key(job_id):
            job = Job()
            job.id = job_id
            self.__jobs[job_id] = job
        return self.__jobs[job_id]

    def __update_job(self, k, v):
        ks = k.split('/')[2:]
        n = len(ks)
        job_id = ks[0]
        job = self.__get_job_or_create_if_absent(job_id)
        if n == 1:
            job.id = job_id
            job.command = json.loads(v)
        elif n == 3:
            worker = ks[2]
            state = job.get_state_or_create_if_absent(worker)
            state.status = v
            if self.__onJobStatusChanged is not None:
                self.__onJobStatusChanged(job)
        elif n == 4:
            worker = ks[2]
            state = job.get_state_or_create_if_absent(worker)
            prop = ks[3]
            if prop == "stdouterr":
                state.stdouterr = v

    def __get_jobs(self):
        ''' example of key-value format in etcd server:
        job/3
        ["ls", "-l", ".."]
        job/3/state/wenzhe
        done
        job/3/state/wenzhe/stdouterr
        total 1760
        drwxr-xr-x 1 weliu 1049089       0 Dec 13 17:18 angular
        drwxr-xr-x 1 weliu 1049089       0 Jan 17 16:53 bctools
        drwxr-xr-x 1 weliu 1049089       0 Jan  2 09:47 cluster
        '''
        jobs = self.cli.get_prefix(JOB_PREFIX)
        for job in jobs:
            k = job[1].key
            v = job[0]
            self.__update_job(k, v)
        return self.__jobs

    def __watch_jobs(self):
        events, self.__cancelWatchJobs = self.cli.watch_prefix(JOB_PREFIX)

        def update_jobs(evts):
            for evt in evts:
                if isinstance(evt, PutEvent):
                    self.__update_job(evt.key, evt.value)
                elif isinstance(evt, DeleteEvent):
                    pass  # currently no job remove yet

        thread.start_new_thread(update_jobs, (events,))

    def get_jobs(self):
        if self.__cancelWatchJobs is None:
            self.__get_jobs()
            self.__watch_jobs()
        return self.__jobs

    def get_job(self, job_id):
        jobs = self.get_jobs()
        return jobs[job_id]

    def get_job_state(self, job_id, worker_name):
        job = self.get_job(job_id)
        return job.stateOfWorkers[worker_name]

    def nodes(self):
        result = self.cli.get_prefix('crms/')
        nodes = {}
        for r in result:
            k = r[1].key
            v = r[0]
            nodes[k] = v
        return nodes

    def clean(self):
        self.cli.delete_prefix('crms/assign/')
        self.cli.delete_prefix('crms/job/')


# example of usage
if __name__ == "__main__":
    cli = CrmsCli()
    # import pdb
    # pdb.set_trace()
    print cli.get_workers()
    # import time

    # time.sleep(20)
    # print cli.getWorkers()
    cli.create_job("1234", ['pwd'])
    cli.run_job("1234", "wenzhe")


    def print_jobs(jobs):
        for jobId, job in jobs.items():
            print "job id:", job.id
            print "job command", ' '.join(job.command)
            for worker, state in job.stateOfWorkers.items():
                print worker, state.status
                print state.stdouterr

    print_jobs(cli.get_jobs())
    cli.run_job("1234", "qiqi")
    print_jobs(cli.get_jobs())
    cli.close()
