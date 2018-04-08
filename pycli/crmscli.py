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
import os
import threading
from etcd3.events import PutEvent
from etcd3.events import DeleteEvent

JOB_PREFIX = 'crms/job/'
SERVER_PREFIX = 'crms/server/'
ASSIGN_PREFIX = 'crms/assign/'


def synchronized(func):
    func.__lock__ = threading.Lock()

    def synced_func(*args, **kws):
        with func.__lock__:
            return func(*args, **kws)

    return synced_func


class JobState(object):
    def __init__(self):
        self.status = 'new'
        self.stdouterr = ''


class Job(object):
    def __init__(self):
        self.id = ''
        self.command = []  # jobCommand is an array of each part of the command
        self.state = JobState()

    def get_state(self):
        return self.state


def start_server(server_host, name, parellel_count, etcd_host_port):
    ''' no wait for started '''
    os.system('ssh %s GoCRMS %s %d %s &' %(server_host, name, parellel_count, etcd_host_port))


def start_server_by_lsf(name, parellel_count, etcd_host_port):
    ''' no wait for started '''
    os.system(
        # 'bsub -q pwodebug "GoCRMS %s %d %s &"'
        'bsub -R "type==any" -q pwodebug "GoCRMS %s %d %s" &'
        % (name, parellel_count, etcd_host_port))


def start_multiple_servers_by_lsf(server_count, name_prefix, parellel_count, etcd_host_port):
    ''' no wait for started '''
    os.system(
        # 'bsub -q pwodebug "GoCRMS %s %d %s &"'
        'bsub -R "type==any" -q pwodebug "gocrmsstarter %d %s %d %s" &'
        % (server_count, name_prefix, parellel_count, etcd_host_port))


class CrmsCli(object):
    def __init__(self, host_port='localhost:2379'):
        hp = host_port.split(':')
        host = hp[0]
        port = int(hp[1])
        self.cli = etcd3.client(host, port)
        self.__servers = {}  # key: server name, value: server parellel job count
        self.__cancel_watch_servers = None
        self.__jobs = {}  # key: jobId, value: Job
        self.__cancelWatchJobs = None
        self.__onJobStatusChanged = None

        self.get_servers()
        self.get_jobs()

    def add_watcher(self, on_job_status_changed):
        self.get_servers()
        self.get_jobs()

        self.__onJobStatusChanged = on_job_status_changed

    def close(self):
        if self.__cancel_watch_servers is not None:
            self.__cancel_watch_servers()
            self.__cancel_watch_servers = None
        if self.__cancelWatchJobs is not None:
            self.__cancelWatchJobs()
            self.__cancelWatchJobs = None
        print "close"

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.close()

    @synchronized
    def get_servers(self):
        if self.__cancel_watch_servers is None:  # not watch yet
            self.__servers = self.__get_servers()
            self.__watch_servers()
        return self.__servers

    def __get_servers(self):
        servers = self.cli.get_prefix(SERVER_PREFIX)
        return {wk[1].key[len(SERVER_PREFIX):]: wk[0] for wk in servers}

    def __watch_servers(self):
        evts, self.__cancel_watch_servers = self.cli.watch_prefix(SERVER_PREFIX)

        def update_servers(evts):
            for evt in evts:
                if isinstance(evt, PutEvent):
                    self.__servers[evt.key[len(SERVER_PREFIX):]] = evt.value
                elif isinstance(evt, DeleteEvent):
                    self.__servers.pop(evt.key[len(SERVER_PREFIX):])

        thread.start_new_thread(update_servers, (evts,))

    def stop_server(self, name):
        self.cli.put(SERVER_PREFIX + name, 'close')

    def stop_servers(self):
        for server in self.get_servers().keys():
            self.stop_server(server)

    def create_job(self, job_id, job_command):  # jobCommand is an array of each part of the command
        cmd = json.dumps(job_command)  # e.g: ['ls', '-l', '..']
        self.cli.put(JOB_PREFIX + job_id, cmd)

    def run_job(self, job_id, server):
        self.cli.put(ASSIGN_PREFIX + server + '/' + job_id, '')

    def __get_job_or_create_if_absent(self, job_id):
        if not self.__jobs.has_key(job_id):
            job = Job()
            job.id = job_id
            self.__jobs[job_id] = job
        return self.__jobs[job_id]

    @synchronized
    def __update_job(self, k, v):
        ks = k.split('/')[2:]
        n = len(ks)
        job_id = ks[0]
        job = self.__get_job_or_create_if_absent(job_id)
        if n == 1:
            job.id = job_id
            job.command = json.loads(v)
        elif n == 2:
            prop = ks[1]
            if prop == 'state':
                job.state.status = v
                if self.__onJobStatusChanged is not None:
                    self.__onJobStatusChanged(job)
        elif n == 3:
            prop = ks[2]
            if prop == "stdouterr":
                job.state.stdouterr = v

    def __get_jobs(self):
        ''' example of key-value format in etcd server:
        job/3
        ["ls", "-l", ".."]
        job/3/state
        done
        job/3/state/stdouterr
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

    @synchronized
    def get_jobs(self):
        if self.__cancelWatchJobs is None:
            self.__get_jobs()
            self.__watch_jobs()
        return self.__jobs

    @synchronized
    def get_job(self, job_id):
        jobs = self.get_jobs()
        return jobs[job_id]

    def nodes(self):
        result = self.cli.get_prefix('crms/')
        nodes = {}
        for r in result:
            k = r[1].key
            v = r[0]
            nodes[k] = v
        return nodes

    def clean(self):
        ''' clean assign and job, not clean server '''
        self.cli.delete_prefix('crms/assign/')
        self.cli.delete_prefix('crms/job/')


# example of usage
if __name__ == "__main__":
    cli = CrmsCli()
    # import pdb
    # pdb.set_trace()
    print cli.get_servers()
    # import time

    # time.sleep(20)
    # print cli.getservers()
    cli.create_job("1234", ['pwd'])
    cli.run_job("1234", "wenzhe")


    def print_jobs(jobs):
        for jobId, job in jobs.items():
            print "job id:", job.id
            print "job command", ' '.join(job.command)
            print "job status", job.state.status
            print job.state.stdouterr

    print_jobs(cli.get_jobs())
    cli.run_job("1234", "qiqi")
    print_jobs(cli.get_jobs())
    cli.close()
