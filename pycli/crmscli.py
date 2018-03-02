import etcd3
import json
from etcd3.events import PutEvent
from etcd3.events import DeleteEvent

JOB_PRIFIX = len('job/')

class JobState(object):
  def __init__(self):
    self.status = 'new'
    self.stdouterr = ''

class Job(object):
  def __init__(self):
    self.id = ''
    self.command = [] # jobCommand is an array of each part of the command
    self.stateOfWorkers = {} # key: assigned worker name, value: JobData (state + stdout/err)

  def getStateOrCreateIfAbsent(self, workerName):
    if not self.stateOfWorkers.has_key(workerName):
      self.stateOfWorkers[workerName] = JobState()
    return self.stateOfWorkers[workerName]

class CrmsCli(object):
  def __init__(self):
    self.cli = etcd3.client()
    self.__workers = {} # key: worker name, value: worker parellel job count
    self.__cancelWatchWorkers = None
    self.__jobs = {} # key: jobId, value: Job
    self.__cancelWatchJobs = None

  def close(self):
    if self.__cancelWatchWorkers != None:
      self.__cancelWatchWorkers()
      self.__cancelWatchWorkers = None

  def getWorkers(self):
    if self.__cancelWatchWorkers == None: # not watch yet
      self.__workers = self.__getWorkers()
      self.__watchWorkers()
    return self.__workers

  def __getWorkers(self):
    workers = self.cli.get_prefix('worker/')
    return {wk[1].key : wk[0] for wk in workers}

  def __watchWorkers(self):
    watchedWorkerEvents, self.__cancelWatchWorkers = self.cli.watch_prefix('worker/')
    def updateWorkers(events):
      for evt in events:
        if isinstance(evt, PutEvent):
          self.__workers[evt.key] = evt.value
        elif isinstance(evt, DeleteEvent):
          self.__workers.pop(evt.key)
    thread.start_new_thread(updateWorkers, (watchedWorkerEvents))

  def stopWorker(self, name):
    self.cli.put('worker/' + name, 'close')

  def createJob(self, jobId, jobCommand): # jobCommand is an array of each part of the command
    cmd = json.dumps(jobCommand) # e.g: ['ls', '-l', '..']
    cli.put('job/' + jobId, cmd)

  def runJob(self, jobId, workerNameList):
    for worker in workerNameList:
      self.cli.put('assign/' + worker + '/' + jobId, '')

  def __getJobOrCreateIfAbsent(self, jobId):
    if not self.__jobs.has_key(jobId):
      self.__jobs[jobId] = Job()
    return self.__jobs[jobId]

  def __getJobs(self):
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
    jobs = self.cli.get_prefix('job/')
    for job in jobs:
      k = job[1].key[JOB_PRIFIX:] # now k is 123/state/wenzhe/stdouterr for example (job/ has removed)
      v = job[0]
      ks = k.split('/')
      n = len(ks)
      jobId = ks[0]
      job = self.__getJobOrCreateIfAbsent(jobId)
      if n == 1:
        job.id = jobId
        job.command = json.loads(v)
      elif n == 3:
        worker = ks[2]
        state = job.getStateOrCreateIfAbsent(worker)
        state.status = v
      elif n == 4:
        worker = ks[2]
        state = job.getStateOrCreateIfAbsent(worker)
        state.stdouterr = v
    return self.__jobs

  def __watchJobs(self):
    pass

  def getJobs(self):
    if self.__cancelWatchJobs == None:
      self.__getJobs()
      self.__watchJobs()
    return self.__jobs

