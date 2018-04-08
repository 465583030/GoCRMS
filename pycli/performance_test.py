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
# Usage: python performance_test.py <num_of_jobs> [work_second=0] [server_count=1] [etcd_host:port=localhost:2379] [gocrms_count_per_lsf_server=1]
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
USE_LSF = True

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

    def get_count(self):
        self.lock.acquire()
        cnt = self.count
        self.lock.release()


class JobTime(object):
    def __init__(self):
        self.start_on_client = None
        self.start_on_server = None
        self.end_on_server = None
        self.end_on_client = None
        self.create_job_on_client = None
        self.job_become_running = None
        self.server = ""

    def start_cost(self):
        self.__check_none()
        return self.start_on_server - self.start_on_client

    def end_cost(self):
        self.__check_none()
        return self.end_on_client - self.end_on_server

    def create_to_running(self):
        if None in [self.create_job_on_client, self.job_become_running]:
            print 'JobTime contains None field', self.__dict__
        return self.job_become_running - self.create_job_on_client

    def __check_none(self):
        if None in [self.start_on_client, self.start_on_server, self.end_on_server, self.end_on_client]:
            print 'JobTime contains None field:', self.__dict__


class Summary(object):
    def __init__(self):
        self.servers = []
        self.start_time = datetime.now()
        self.jobs_time = {}

    def get_job(self, job_id):
        if self.jobs_time.has_key(job_id):
            return self.jobs_time[job_id]
        else:
            jt = JobTime()
            self.jobs_time[job_id] = jt
            return jt

    def parse_server_log(self, server):
        '''
        the server's log format is like this:
        fnode400 2018/03/23 11:42:44.374266 server.go:150: Run Job 1 with command: python -c print 1
        fnode401 2018/03/23 11:42:45.005266 server.go:156: Finish Job 0 with result:  0
        '''
        RUN_JOB = 'Run Job '
        LEN_RUN = len(RUN_JOB)
        FINISH_JOB = 'Finish Job '
        LEN_FINISH = len(FINISH_JOB)

        logfile = os.path.expanduser('~/.gocrms/%s.log' % server)
        with open(logfile) as f:
            for line in f:
                line = line.rstrip()
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
                    self.get_job(job_id).server = server
                    self.get_job(job_id).start_on_server = dt
                elif content.startswith(FINISH_JOB):
                    job_id = find_job_id(content[LEN_FINISH:])
                    self.get_job(job_id).server = server
                    self.get_job(job_id).end_on_server = dt

    def parse_client_log(self):
        '''
        log format:
        2018-03-23 10:13:37,927 run job 1 on server w1
        2018-03-23 10:13:38,269 finish job 0 with result 0
        '''
        CREATE_JOB = 'create job '
        LEN_CREATE = len(CREATE_JOB)
        RUN_JOB = 'run job '
        LEN_RUN = len(RUN_JOB)
        RUNNING_JOB = 'become running for job '
        LEN_RUNNING = len(RUNNING_JOB)
        FINISH_JOB = 'finish job '
        LEN_FINISH = len(FINISH_JOB)

        logfile = os.path.expanduser(LOG_PATH)
        with open(logfile) as f:
            for line in f:
                line = line.rstrip()
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
                    server = content.split(' ')[5]
                    self.get_job(job_id).server = server
                elif content.startswith(FINISH_JOB):
                    job_id = find_job_id(content[LEN_FINISH:])
                    self.get_job(job_id).end_on_client = dt
                elif content.startswith(CREATE_JOB):
                    job_id = find_job_id(content[LEN_CREATE:])
                    self.get_job(job_id).create_job_on_client = dt
                elif content.startswith(RUNNING_JOB):
                    job_id = find_job_id(content[LEN_RUNNING:])
                    self.get_job(job_id).job_become_running = dt

    def parse_log(self):
        self.parse_client_log()
        for server in summary.servers:
            self.parse_server_log(server)

    def average_create_to_running(self):
        costs = [jt.create_to_running() for jt in self.jobs_time.values()]
        return average(costs)

    def average_start_cost(self):
        costs = [jt.start_cost() for jt in self.jobs_time.values()]
        return average(costs)

    def average_end_cost(self):
        costs = [jt.end_cost() for jt in self.jobs_time.values()]
        return average(costs)

    def __average_servers_cost(self, fn):
        server_cost_map = {}
        for jt in self.jobs_time.values():
            if not server_cost_map.has_key(jt.server):
                server_cost_map[jt.server] = []
            server_cost_map[jt.server].append(fn(jt))
        for server in server_cost_map.keys():
            server_cost_map[server] = average(server_cost_map[server])
        return server_cost_map

    def average_servers_create_to_running(self):
        return self.__average_servers_cost(lambda jt: jt.create_to_running())

    def average_servers_start_cost(self):
        return self.__average_servers_cost(lambda jt: jt.start_cost())

    def average_servers_end_cost(self):
        return self.__average_servers_cost(lambda jt: jt.end_cost())

    def first_start_on_client(self):
        return min([jt.start_on_client for jt in self.jobs_time.values()])

    def first_start_on_server(self):
        return min([jt.start_on_server for jt in self.jobs_time.values()])

    def last_start_on_client(self):
        return max([jt.start_on_client for jt in self.jobs_time.values()])

    def last_start_on_server(self):
        return max([jt.start_on_server for jt in self.jobs_time.values()])

    def first_end_on_server(self):
        return min([jt.end_on_server for jt in self.jobs_time.values()])

    def first_end_on_client(self):
        return min([jt.end_on_client for jt in self.jobs_time.values()])

    def last_end_on_server(self):
        return max([jt.end_on_server for jt in self.jobs_time.values()])

    def last_end_on_client(self):
        return max([jt.end_on_client for jt in self.jobs_time.values()])

    def first_create_on_client(self):
        return min([jt.create_job_on_client for jt in self.jobs_time.values()])

    def max_create_to_running(self):
        return max([jt.create_to_running() for jt in self.jobs_time.values()])

    def last_running(self):
        return max([jt.job_become_running for jt in self.jobs_time.values()])

    def report(self):
        # fmt = '%-6s | %-8s | %-17s | %-16s | %-15s | %-15s | %-15s'
        # print fmt % (
        #     'job id', 'server', 'create to running',
        #     'create on client', 'start on client',
        #     'become running', 'end on client'
        # )
        # for job_id, jt in sorted(self.jobs_time.items()):
        #     print fmt % (
        #         job_id, jt.server, jt.create_to_running(),
        #         jt.create_job_on_client.time(), jt.start_on_client.time(),
        #         jt.job_become_running.time(), jt.end_on_client.time()
        #     )
        #
        # print 'average create to running for each server:'
        # fmt = '%-8s | %-17s'
        # print fmt % (
        #     'server', 'create to running'
        # )
        average_servers_create_to_running = self.average_servers_create_to_running()
        # for server, create_to_running in sorted(average_servers_create_to_running.items()):
        #     print fmt % (
        #         server, create_to_running
        #     )

        print 'total %d running on %d servers' % (jobcount, len(average_servers_create_to_running))
        print 'average create to running:', self.average_create_to_running()

        # first_start_on_client = self.first_start_on_client()
        # first_start_on_server = self.first_start_on_server()
        # last_start_on_client = self.last_start_on_client()
        # last_start_on_server = self.last_start_on_server()
        # first_end_on_server = self.first_end_on_server()
        # first_end_on_client = self.first_end_on_client()
        # last_end_on_server = self.last_end_on_server()
        # last_end_on_client = self.last_end_on_client()
        # print 'first start on client', first_start_on_client
        # print 'first start on server', first_start_on_server
        # print 'last start on client', last_start_on_client
        # print 'last start on server', last_start_on_server

        print 'total start cost (last running - first create on client)', \
            self.last_running() - self.first_create_on_client()


logger = logging.getLogger("crms")
jobcount = int(sys.argv[1])
jobs_finished_count = CountDownLatch(jobcount)
summary = Summary()


def on_job_status_changed(job):
    job_state = job.get_state()
    # print "job", job.id, "status change to", job_state.status
    if job_state.status == 'running':
        logger.info('become running for job %s', job.id)
    elif job_state.status in ['done', 'fail']:
        logger.info('finish job %s with result %s', job.id, job_state.stdouterr)
        jobs_finished_count.count_down()
        sys.stdout.write("left job count: %d\r" % jobs_finished_count.count)
        sys.stdout.flush()


def test_run_job(job_count):
    if len(sys.argv) < 3:
        work_time = 0
    else:
        work_time = sys.argv[2]

    if len(sys.argv) < 4:
        server_count = 1
    else:
        server_count = int(sys.argv[3])

    if len(sys.argv) < 5:
        host_port = 'localhost:2379'
    else:
        host_port = sys.argv[4]

    print 'connect etcd', host_port
    with crmscli.CrmsCli(host_port) as crms:
        crms.add_watcher(on_job_status_changed)

        crms.clean()

        if USE_LSF:
            if len(sys.argv) < 6:
                start_servers_by_lsf(host_port, server_count)
            else:
                gocrms_count_per_lsf_server = int(sys.argv[5])
                start_multiple_servers_by_lsf(host_port, server_count, gocrms_count_per_lsf_server)
        else:
            start_servers(host_port, server_count)

        wait_for_servers_register(crms, server_count)

        servers = crms.get_servers().keys()
        logger.info("servers: %s", servers)
        if len(servers) == 0:
            sys.exit(0)
        summary.servers = servers
        for i in xrange(job_count):
            job_id = '%05d' %i
            logger.info('create job %s', job_id)
            crms.create_job(job_id, ['python', '-c', 'import time; time.sleep(%s); print "%s"' %(work_time, job_id)])
            server = random.choice(servers)
            logger.info('run job %s on server %s', job_id, server)
            crms.run_job(job_id, server)

        jobs_finished_count.await()
        # print_nodes(crms.nodes())

        stop_servers(crms)


def get_available_servers(server_count):
    '''
    fnode091      up 375+20:58,     0 users,  load  0.02,  0.14,  0.45
    fnode092      up 403+23:21,     0 users,  load  1.92,  2.14,  1.92
    '''
    p = subprocess.Popen("ruptime | grep 'fnode.*up'", shell=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    cnt = 0
    servers = []
    for line in p.stdout.readlines():
        servers.append(line.split(' ')[0])
        cnt += 1
        if cnt >= server_count:
            break
    p.wait()
    return servers


def start_servers(etcd_host_port, server_count):
    available_servers = get_available_servers(server_count)
    print 'available servers:', available_servers  # ['fnode400', 'fnode401']
    available_count = len(available_servers)

    for i in xrange(server_count):
        j = i % available_count
        server_host = available_servers[j]
        name = 'sv' + str(i)
        logger.info('start server %s in host %s', name, server_host)
        crmscli.start_server(server_host, name, 100, etcd_host_port)


def start_servers_by_lsf(etcd_host_port, count):
    for i in xrange(count):
        name = 'sv' + str(i)
        logger.info('start server %s in LSF', name)
        crmscli.start_server_by_lsf(name, 100, etcd_host_port)


def start_multiple_servers_by_lsf(etcd_host_port, gocrms_server_count, gocrms_count_per_lsf_server):
        # each LSF server starts 100 GoCRMS server
    left_server_count = gocrms_server_count
    i = 0
    while left_server_count > 0:
        name = 'sv' + str(i)
        i += 1
        server_count_to_start = min(left_server_count, gocrms_count_per_lsf_server)
        crmscli.start_multiple_servers_by_lsf(server_count_to_start, name, 100, etcd_host_port)
        left_server_count -= gocrms_count_per_lsf_server


def wait_for_servers_register(crms, servers_count):
    try_count = 0
    while try_count < 20 and len(crms.get_servers()) < servers_count:
        sys.stdout.write("CRMS server count: %d\r" % len(crms.get_servers()))
        sys.stdout.flush()
        try_count += 1
        time.sleep(2)
    print ''
    print "CRMS server count: %d" % len(crms.get_servers())


def stop_servers(crms):
    for server in crms.get_servers().keys():
        crms.stop_server(server)
    time.sleep(1)  # sleep 1s so that servers have time to flush log (when close log file)


def print_nodes(nodes):
    print("nodes:")
    for (k, v) in sorted(nodes.items()):
        print k, ":", v


def init_log():
    formatter = logging.Formatter('%(asctime)s %(message)s', )
    logfile = os.path.expanduser(LOG_PATH)
    if not os.path.isfile(logfile):
        dirname = os.path.dirname(logfile)
        if not os.path.isdir(dirname):
            os.makedirs(dirname)
        with open(logfile, 'w') as f:
            f.write("")
    file_handler = logging.FileHandler(logfile)
    file_handler.setFormatter(formatter)
    stream_handler = logging.StreamHandler(sys.stderr)
    logger.addHandler(file_handler)
    # logger.addHandler(stream_handler)
    logger.setLevel(logging.INFO)
    return file_handler


def main():
    print "start performance test for GoCRMS"
    file_handler = init_log()
    test_run_job(jobcount)
    file_handler.close() # close log file to flush

    print "Summary:"
    summary.parse_client_log()
    summary.report()


if __name__ == "__main__":
    main()
