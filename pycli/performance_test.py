import crmscli
import random
import time
import os.path

def on_job_status_changed(job):
    job_state = job.get_state()
    print "job", job.id, "status change to", job_state.status
    if job_state.status in ['done', 'fail']:
        print "finish job", job.id, "with result", job_state.stdouterr


def test_run_job(job_count):
    crms = crmscli.CrmsCli()
    crms.add_watcher(on_job_status_changed)

    crms.clean()

    workers = crms.get_workers().keys()
    print workers
    if len(workers) == 0:
        return
    for i in xrange(job_count):
        f = os.path.join(os.path.dirname(__file__), 'mock_job.py')
        job_id = str(i)
        print 'create job', job_id
        crms.create_job(job_id, ['python', '-c', 'print ' + job_id])
        worker = random.choice(workers)
        print 'run job', job_id, 'on worker', worker
        crms.run_job(job_id, worker)
    # printNodes(crms.nodes())
    time.sleep(5)
    print_nodes(crms.nodes())
    crms.close()


def print_nodes(nodes):
    print("nodes:")
    for (k, v) in sorted(nodes.items()):
        print k, ":", v


def init_log():
    pass


if __name__ == "__main__":
    init_log()
    test_run_job(2)