# usage: python close_crms.py <host:port>


import subprocess
import os
import crmscli
import sys


def kill_crms(server):
    '''
    weliu    634111  6.4  0.0 3223824 15796 ?       Ssl  Mar29  88:21 GoCRMS fnode1080 100 172.20.1.14:12379
    weliu    634112  6.6  0.0 3160368 16820 ?       Ssl  Mar29  90:56 GoCRMS fnode1081 100 172.20.1.14:12379
    '''
    p = subprocess.Popen("ssh %s ps aux | grep GoCRMS" %server, shell=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    pids = []
    for line in p.stdout.readlines():
        line = line.rstrip()
        if line.endswith('grep GoCRMS'):
            continue
        args = filter(lambda s: len(s) > 0, line.split(' '))
        if len(args) < 2:
            continue
        pid = args[1]
        if pid.isdigit():
            pids.append(pid)
    p.wait()
    for pid in pids:
        cmd = 'ssh %s kill -9 %s &' %(server, pid)
        print cmd
        os.system(cmd)


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


def kill_crms_workers_by_etcd():
    host_port = sys.argv[1]
    with crmscli.CrmsCli(host_port) as crms:
        crms.stop_workers()


if __name__ == "__main__":
    #for server in get_available_workers(300):
    #    kill_crms(server)
    kill_crms_workers_by_etcd()
