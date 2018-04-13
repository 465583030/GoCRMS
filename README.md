# GoCRMS
Cluster Manager written in Go lang

# TODO
kill job assign new job
test etcd server max count
leaf job/host job

# start worker
```
GoCRMS <Worker Name> <ParellelAbility> [<etcd_host:port>=localhost:2379]
```

# Go CLI
demo by gore (go get -u github.com/motemen/gore)
```
C:\Users\weliu>gore
gore version 0.2.6  :help for help
gore> :import github.com/WenzheLiu/GoCRMS/gocrmscli
gore> :import time
gore> crms, err := gocrmscli.New([]string{"localhost:2379"}, 5 * time.Second, 10 * time.Second)
&gocrmscli.CrmsCli{cli:(*clientv3.Client)(0xc04201ea80), requestTimeout:10000000000, workers:map[string]*gocrmscli.Worker{}, jobs:map[string]*gocrmscli
.Job{}, cancelWatchWorkers:(context.CancelFunc)(nil), cancelWatchJobs:(context.CancelFunc)(nil), cancelWatchAssign:(context.CancelFunc)(nil)}
<nil>
gore> workers, err := crms.GetWorkers()
map[string]*gocrmscli.Worker{"qiqi":(*gocrmscli.Worker)(0xc0421e2020), "wenzhe":(*gocrmscli.Worker)(0xc042002120)}
<nil>
gore> wenzhe, exist, err := crms.GetWorker("wenzhe")
&gocrmscli.Worker{Name:"wenzhe", ParellelAbility:2, Jobs:map[string]bool{}}
true
<nil>
gore> crms.CreateJob("9876", []string{"go", "version"})
<nil>
gore> jobs, err := crms.GetJobs()
map[string]*gocrmscli.Job{"7887":(*gocrmscli.Job)(0xc04219a870), "8081":(*gocrmscli.Job)(0xc04219a8d0), "9876":(*gocrmscli.Job)(0xc04219a930), "1256":(
*gocrmscli.Job)(0xc04219a690), "1847":(*gocrmscli.Job)(0xc04219a6f0), "2":(*gocrmscli.Job)(0xc04219a750), "3":(*gocrmscli.Job)(0xc04219a7b0), "4059":(*
gocrmscli.Job)(0xc04219a810), "1":(*gocrmscli.Job)(0xc04219a630)}
<nil>
gore> job, exist, err := crms.GetJob("9876")
&gocrmscli.Job{ID:"9876", Command:[]string{"go", "version"}, StateOfWorkers:map[string]*gocrmscli.JobState{}}
true
<nil>
gore> crms.RunJob("9876", "wenzhe")
<nil>
gore> job
&gocrmscli.Job{ID:"9876", Command:[]string{"go", "version"}, StateOfWorkers:map[string]*gocrmscli.JobState{"wenzhe":(*gocrmscli.JobState)(0xc0421316c0)
}}
gore> jobState, exist := job.StateOfWorkers["wenzhe"]
&gocrmscli.JobState{Status:"done", Stdouterr:"go version go1.9.3 windows/amd64\n"}
true
```

# Python CLI
demo
```
>>> import sys
>>> sys.path.insert(0, r"C:\Users\weliu\code\go\src\github.com\WenzheLiu\GoCRMS\pycli")
>>> import crmscli
>>> crms = crmscli.CrmsCli()
>>> crms.createJob("1256", ["java", "-version"])
>>> crms.getJobs()
{'1256': <crmscli.Job object at 0x03140B70>, '8081': <crmscli.Job object at 0x03140E90>, '1847': <crmscli.Job object at 0x03140BB0>, '4059': <crmscli.Job object at 0x03140D30>, '1': <crmscli.Job object at 0x031404B0>, '3': <crmscli.Job object at 0x03140CB0>, '2': <crmscli.Job object at 0x03140B30>, '7887': <crmscli.Job object at 0x03140DD0>}
>>> crms.getWorkers()
{'worker/wenzhe': '2', 'worker/qiqi': '1'}
>>> crms.runJob("1256", ["wenzhe"])
>>> job = crms.getJob("1256")
>>> job.stateOfWorkers
{'wenzhe': <crmscli.JobState object at 0x0313FAF0>}
>>> job.stateOfWorkers['wenzhe'].status
'done'
>>> job.stateOfWorkers['wenzhe'].stdouterr
'java version "1.8.0_60"\r\nJava(TM) SE Runtime Environment (build 1.8.0_60-b27)\r\nJava HotSpot(TM) 64-Bit Server VM (build 25.60-b23, mixed mode)\r\nPicked up _JAVA_OPTIONS: -Djava.net.preferIPv4Stack=true\n'
```

# Web UI
```
./gocrmsweb [<webport>=8080] [<etcd_host:port>=localhost:2379]
(make sure "dist/" folder built by angular is in the same folder with gocrmsweb)
```

# Work Log:
## etcd takes resources:
```
[weliu@fnode438 ~]$ top

top - 01:51:59 up 608 days,  8:08,  2 users,  load average: 0.65, 0.82, 0.89
Tasks: 650 total,   1 running, 649 sleeping,   0 stopped,   0 zombie
Cpu(s):  0.0%us,  0.0%sy,  0.0%ni, 99.9%id,  0.1%wa,  0.0%hi,  0.0%si,  0.0%st
Mem:  132131272k total, 101924600k used, 30206672k free,   504580k buffers
Swap: 100662268k total, 18446744073709457064k used, 100756820k free, 97277272k cached

   PID USER      PR  NI  VIRT  RES  SHR S %CPU %MEM    TIME+  COMMAND
292800 weliu     20   0 7779m 2.2g 1.2g S  0.7  1.7   7:19.27 etcd
296066 weliu     20   0 13540 1628  868 R  0.3  0.0   0:00.37 top
     1 root      20   0 19356  620  388 S  0.0  0.0   0:11.20 init
     2 root      20   0     0    0    0 S  0.0  0.0   0:03.54 kthreadd
```
## Issue: database space exceeded
It is successful to support 33000 servers:
```
[weliu@fnode332 pycli]$ etcdctl --endpoints 172.20.0.18:12379 get --prefix crms/server/ | grep crms | wc -l
33000
```
When jobs count is 330000 and server count is 33000, etcd server shows database space exceeded.
```
[weliu@fnode332 pycli]$ python performance_test.py 330000 2 33000 172.20.0.18:12379 100
start performance test for GoCRMS
connect etcd 172.20.0.18:12379
closejob count: 139096
Traceback (most recent call last):
  File "performance_test.py", line 503, in <module>
    main()
  File "performance_test.py", line 494, in main
    test_run_job(jobcount)
  File "performance_test.py", line 383, in test_run_job
    crms.run_job(job_id, server)
  File "/gpfs/DEV/PWO/weliu/code/go/src/github.com/WenzheLiu/GoCRMS/pycli/crmscli.py", line 149, in run_job
    self.cli.put(ASSIGN_PREFIX + server + '/' + job_id, '')
  File "/gpfs/DEV/PWO/weliu/code/python2.7/site-packages/etcd3/client.py", line 48, in handler
    _translate_exception(exc)
  File "/gpfs/DEV/PWO/weliu/code/python2.7/site-packages/etcd3/client.py", line 46, in handler
    return f(*args, **kwargs)
  File "/gpfs/DEV/PWO/weliu/code/python2.7/site-packages/etcd3/client.py", line 341, in put
    credentials=self.call_credentials,
  File "/gpfs/DEV/PWO/weliu/code/python2.7/site-packages/grpc/_channel.py", line 487, in __call__
    return _end_unary_response_blocking(state, call, False, deadline)
  File "/gpfs/DEV/PWO/weliu/code/python2.7/site-packages/grpc/_channel.py", line 437, in _end_unary_response_blocking
    raise _Rendezvous(state, None, None, deadline)
grpc._channel._Rendezvous: <_Rendezvous of RPC that terminated with (StatusCode.RESOURCE_EXHAUSTED, etcdserver: mvcc: database space exceeded)>
```
In this case, the total keys count in etcd is 607136.
```
[weliu@fnode332 pycli]$ etcdctl --endpoints 172.20.0.18:12379 del --prefix crms/job

[weliu@fnode332 pycli]$ etcdctl --endpoints 172.20.0.18:12379 get --prefix crms/ | grep crms | wc -l
607136
```
Now put any node to etcd will fail (even remove the jobs node)
```
[weliu@fnode332 pycli]$ etcdctl --endpoints 172.20.0.18:12379 put newnode newvalue
Error: etcdserver: mvcc: database space exceeded
```
kill the new added servers and back to 29000, but etcd server is still not work.
```
[weliu@fnode332 pycli]$ bkill 571253
Job <571253> is being terminated
[weliu@fnode332 pycli]$ etcdctl --endpoints 172.20.0.18:12379 get --prefix crms/server/ | grep crms | wc -l
29000
[weliu@fnode332 pycli]$ etcdctl --endpoints 172.20.0.18:12379 put newnode newvalue
Error: etcdserver: mvcc: database space exceeded
```
Use `alarm list` or `endpoint status` sub command to show the reason:
```
[weliu@fnode332 pycli]$ etcdctl --endpoints 172.20.0.18:12379 alarm list
memberID:16386505787103497523 alarm:NOSPACE
[weliu@fnode332 pycli]$ etcdctl --endpoints 172.20.0.18:12379 endpoint status
172.20.0.18:12379, e3689031a62fcd33, 3.3.0+git, 2.2 GB, true, 2, 20185482, 20185482, memberID:16386505787103497523 alarm:NOSPACE
[weliu@fnode332 pycli]$ etcdctl --endpoints 172.20.0.18:12379 endpoint status --write-out="json"
[{"Endpoint":"172.20.0.18:12379","Status":{"header":{"cluster_id":16034816922318444818,"member_id":16386505787103497523,"revision":17261206,"raft_term":2},"version":"3.3.0+git","dbSize":2185646080,"leader":16386505787103497523,"raftIndex":20185482,"raftTerm":2,"raftAppliedIndex":20185482,"errors":["memberID:16386505787103497523 alarm:NOSPACE "],"dbSizeInUse":2185240576}}]
```
### Reason to the error "database space exceeded":
在[Etcd 架构与实现解析](http://jolestar.com/etcd-architecture/)中，“Etcd 的 compact 机制”一节说明了原因：Etcd 默认不会自动 compact，需要设置启动参数，或者通过命令进行compact，如果变更频繁建议设置，否则会导致空间和内存的浪费以及错误。Etcd v3 的默认的 backend quota 2GB，如果不 compact，boltdb 文件大小超过这个限制后，就会报错：”Error: etcdserver: mvcc: database space exceeded”，导致数据无法写入。

The snap/db file takes 1.2G:
```
weliu@fnode438 ~]$ ls -lh fnode438.etcd/member/snap/db
-rw------- 1 weliu weliu 1.2G Apr 13 01:53 fnode438.etcd/member/snap/db
```

### Solution
Refer to: [ETCD数据空间压缩清理](http://www.cnblogs.com/davygeek/p/8524477.html)
```
[weliu@fnode332 pycli]$ rev=$(etcdctl --endpoints 172.20.0.18:12379 endpoint status --write-out="json" | egrep -o '"revision":[0-9]*' | egrep -o '[0-9].*')
[weliu@fnode332 pycli]$ etcdctl --endpoints 172.20.0.18:12379 compact $rev
Error: context deadline exceeded
[weliu@fnode332 pycli]$ etcdctl --endpoints 172.20.0.18:12379 compact $rev
Error: etcdserver: mvcc: required revision has been compacted
[weliu@fnode332 pycli]$ etcdctl --endpoints 172.20.0.18:12379 defrag
Failed to defragment etcd member[172.20.0.18:12379] (context deadline exceeded)
```
Restart etcd:
```
[weliu@fnode438 ~]$ etcd --name fnode438 --initial-advertise-peer-urls http://172.20.0.18:12380 --listen-peer-urls http://172.20.0.18:12380 --listen-client-urls http://172.20.0.18:12379 --advertise-client-urls http://172.20.0.18:12379 --initial-cluster-token etcd-cluster-1 --initial-cluster fnode438=http://172.20.0.18:12380 --initial-cluster-state new
2018-04-13 01:37:08.780086 W | pkg/flags: recognized environment variable ETCD_INITIAL_CLUSTER_STATE, but unused: shadowed by corresponding flag
2018-04-13 01:37:08.780167 I | etcdmain: etcd Version: 3.3.0+git
2018-04-13 01:37:08.780175 I | etcdmain: Git SHA: Not provided (use ./build instead of go build)
2018-04-13 01:37:08.780181 I | etcdmain: Go Version: go1.10
2018-04-13 01:37:08.780186 I | etcdmain: Go OS/Arch: linux/amd64
2018-04-13 01:37:08.780192 I | etcdmain: setting maximum number of CPUs to 32, total number of available CPUs is 32
2018-04-13 01:37:08.780207 W | etcdmain: no data-dir provided, using default data-dir ./fnode438.etcd
2018-04-13 01:37:08.781507 N | etcdmain: the server is already initialized as member before, starting as etcd member...
2018-04-13 01:37:08.781629 I | embed: listening for peers on http://172.20.0.18:12380
2018-04-13 01:37:08.781671 I | embed: listening for client requests on 172.20.0.18:12379

2018-04-13 01:37:27.388720 W | etcdserver: another etcd process is using "fnode438.etcd/member/snap/db" and holds the file lock, or loading backend file is taking >10 seconds
2018-04-13 01:37:27.388755 W | etcdserver: waiting for it to exit before starting...
2018-04-13 01:37:51.331815 I | etcdserver: recovered store from snapshot at index 20200276
2018-04-13 01:38:45.910578 I | mvcc: store.index: compact 17261206
2018-04-13 01:38:51.274892 I | mvcc: resume scheduled compaction at 17261206
2018-04-13 01:38:51.292522 I | etcdserver: name = fnode438
2018-04-13 01:38:51.292545 I | etcdserver: data dir = fnode438.etcd
2018-04-13 01:38:51.292556 I | etcdserver: member dir = fnode438.etcd/member
2018-04-13 01:38:51.292562 I | etcdserver: heartbeat = 100ms
2018-04-13 01:38:51.292568 I | etcdserver: election = 1000ms
2018-04-13 01:38:51.292573 I | etcdserver: snapshot count = 100000
2018-04-13 01:38:51.292598 I | etcdserver: advertise client URLs = http://172.20.0.18:12379
2018-04-13 01:38:51.783043 I | etcdserver: restarting member e3689031a62fcd33 in cluster de871d08e5369912 at commit index 20272986
2018-04-13 01:38:51.787501 I | raft: e3689031a62fcd33 became follower at term 2
2018-04-13 01:38:51.787537 I | raft: newRaft e3689031a62fcd33 [peers: [e3689031a62fcd33], term: 2, commit: 20272986, applied: 20200276, lastindex: 20272986, lastterm: 2]
2018-04-13 01:38:51.787752 I | etcdserver/api: enabled capabilities for version 3.3
2018-04-13 01:38:51.787771 I | etcdserver/membership: added member e3689031a62fcd33 [http://172.20.0.18:12380] to cluster de871d08e5369912 from store
2018-04-13 01:38:51.787780 I | etcdserver/membership: set the cluster version to 3.3 from store
2018-04-13 01:39:46.041163 I | mvcc: store.index: compact 17261206
2018-04-13 01:39:51.341677 I | mvcc: resume scheduled compaction at 17261206
2018-04-13 01:39:51.356265 W | auth: simple token is not cryptographically signed
2018-04-13 01:39:51.358165 I | etcdserver: starting server... [version: 3.3.0+git, cluster version: 3.3]
2018-04-13 01:39:51.389180 I | raft: e3689031a62fcd33 is starting a new election at term 2
2018-04-13 01:39:51.389216 I | raft: e3689031a62fcd33 became candidate at term 3
2018-04-13 01:39:51.389240 I | raft: e3689031a62fcd33 received MsgVoteResp from e3689031a62fcd33 at term 3
2018-04-13 01:39:51.389256 I | raft: e3689031a62fcd33 became leader at term 3
2018-04-13 01:39:51.389266 I | raft: raft.node: e3689031a62fcd33 elected leader e3689031a62fcd33 at term 3
2018-04-13 01:39:51.391371 I | embed: ready to serve client requests
2018-04-13 01:39:51.391489 I | etcdserver: published {Name:fnode438 ClientURLs:[http://172.20.0.18:12379]} to cluster de871d08e5369912
2018-04-13 01:39:51.392002 N | embed: serving insecure client requests on 172.20.0.18:12379, this is strongly discouraged!
WARNING: 2018/04/13 01:39:51 grpc: Server.processUnaryRPC failed to write status: connection error: desc = "transport is closing"
WARNING: 2018/04/13 01:39:51 grpc: Server.processUnaryRPC failed to write status: connection error: desc = "transport is closing"
WARNING: 2018/04/13 01:39:51 grpc: Server.processUnaryRPC failed to write status: connection error: desc = "transport is closing"
WARNING: 2018/04/13 01:39:51 grpc: Server.processUnaryRPC failed to write status connection error: desc = "transport is closing"
```
In client, call `defrag` command to 整理多余的空间:
```
[weliu@fnode332 pycli]$ etcdctl --endpoints 172.20.0.18:12379 defrag
Failed to defragment etcd member[172.20.0.18:12379] (context deadline exceeded)
```
And etcd server side will log:
```
2018-04-13 01:41:48.812722 N | etcdserver/api/v3rpc: starting to defragment the storage backend...
2018-04-13 01:43:01.563374 N | etcdserver/api/v3rpc: finished defragmenting the storage backend
WARNING: 2018/04/13 01:43:01 grpc: Server.processUnaryRPC failed to write status connection error: desc = "transport is closing"
2018-04-13 01:46:04.741091 I | mvcc: finished scheduled compaction at 17261206 (took 6m13.399335972s)
```
The log shows that finished defragmenting and finished scheduled compaction, but still cannot put new node:
```
[weliu@fnode332 pycli]$ etcdctl --endpoints 172.20.0.18:12379 put gdga gada
Error: etcdserver: mvcc: database space exceeded
```
But after defragment, all etcd nodes lost. However, it still not works. Need to study more!
```
[weliu@fnode332 pycli]$ etcdctl --endpoints 172.20.0.18:12379 get --prefix crms/ | grep crms | wc -l
0
```
A fast solution is restarting etcd with a new data dir by `--data-dir <new_data_dir>`, then the etcd server will be a new one.
```
etcd --name fnode438 --initial-advertise-peer-urls http://172.20.0.18:12380 --listen-peer-urls http://172.20.0.18:12380 --listen-client-urls http://172.20.0.18:12379 --advertise-client-urls http://172.20.0.18:12379 --initial-cluster-token etcd-cluster-1 --initial-cluster fnode438=http://172.20.0.18:12380 --initial-cluster-state new --data-dir fnode438.etcd_1
```
Now we can put in client:
```
[weliu@fnode332 pycli]$ etcdctl --endpoints 172.20.0.18:12379 put gdga gada
OK
[weliu@fnode332 pycli]$ etcdctl --endpoints 172.20.0.18:12379 get gdga
gdga
gada
```

## Issue 2: context deadline exceeded
```
[weliu@fnode332 pycli]$ etcdctl --endpoints 172.20.0.18:12379 defrag
Failed to defragment etcd member[172.20.0.18:12379] (context deadline exceeded)
```
