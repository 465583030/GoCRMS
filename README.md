# GoCRMS
Cluster Manager written in Go lang

# start worker
```
GoCRMS <Worker Name> <ParellelAbility>
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
