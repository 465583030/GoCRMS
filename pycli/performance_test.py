import crmscli
import random
import time
import os.path

def onJobStatusChanged(job):
    jobState = job.getState()
    print "job", job.id, "status change to", jobState.status
    if jobState.status in ['done', 'fail']:
        print "finish job", job.id, "with result", jobState.stdouterr

def testRunJob(jobCount):
    crms = crmscli.CrmsCli()
    crms.onJobStatusChanged = onJobStatusChanged
    crms.clean()

    workers = crms.getWorkers().keys()
    print workers
    if len(workers) == 0:
        return
    for i in xrange(jobCount):
        f = os.path.join(os.path.dirname(__file__), 'mock_job.py')
        jobId = "%d"%i
        print 'create job', jobId
        crms.createJob(jobId, ['python', '-c', 'print ' + jobId])
        worker = random.choice(workers)
        print 'run job', jobId, 'on worker', worker
        crms.runJob(jobId, worker)
    # printNodes(crms.nodes())
    time.sleep(5)
    printNodes(crms.nodes())
    crms.close()

def printNodes(nodes):
    print("nodes:")
    for (k, v) in sorted(nodes.items()):
        print k, ":", v

if __name__ == "__main__":
    testRunJob(2)