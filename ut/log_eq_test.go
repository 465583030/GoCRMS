package common

import (
	"testing"
	"time"
	"path"
)

const logA = `
2018/04/24 16:27:45.631202 Slot 0 is assigned to Job pre
2018/04/24 16:27:45.636202 etcd put (crms/jobstate/pre, running)
2018/04/24 16:27:45.652202 Run Job pre with command [python -c import sys,time; print 'pre'; time.sleep(2); print 'after'; sys.exit(0)]
2018/04/24 16:27:46.633202 etcd put (crms/job/0, ["python","-c","import sys,time; print 0; time.sleep(3); print 0; sys.exit(0)"])
2018/04/24 16:27:46.639202 etcd put (crms/assign/s1/0, )
2018/04/24 16:27:46.639202 Slot 1 is assigned to Job 0
2018/04/24 16:27:46.643202 etcd put (crms/job/1, ["python","-c","import sys,time; print 1; time.sleep(0); print -1; sys.exit(1)"])
2018/04/24 16:27:46.647202 etcd put (crms/assign/s1/1, )
2018/04/24 16:27:46.659202 etcd put (crms/job/2, ["python","-c","import sys,time; print 2; time.sleep(1); print -2; sys.exit(0)"])
2018/04/24 16:27:46.659202 etcd put (crms/jobstate/0, running)
2018/04/24 16:27:46.664202 etcd put (crms/assign/s1/2, )
2018/04/24 16:27:46.670202 etcd put (crms/job/3, ["python","-c","import sys,time; print 3; time.sleep(1); print -3; sys.exit(1)"])
2018/04/24 16:27:46.681202 Run Job 0 with command [python -c import sys,time; print 0; time.sleep(3); print 0; sys.exit(0)]
2018/04/24 16:27:46.695202 etcd put (crms/assign/s1/3, )
2018/04/24 16:27:46.714202 etcd put (crms/job/4, ["python","-c","import sys,time; print 4; time.sleep(2); print -4; sys.exit(0)"])
2018/04/24 16:27:46.715202 etcd put (crms/assign/s1/4, )
2018/04/24 16:27:46.718202 etcd put (crms/job/5, ["python","-c","import sys,time; print 5; time.sleep(0); print -5; sys.exit(1)"])
2018/04/24 16:27:46.721202 etcd put (crms/assign/s1/5, )
2018/04/24 16:27:47.904202 Success to run job pre
2018/04/24 16:27:47.906202 etcd put (crms/jobstate/pre, done)
2018/04/24 16:27:47.911202 Slot 0 is assigned to Job 1
2018/04/24 16:27:47.912202 etcd del (crms/assign/s1/pre, )

`

const logB = `
2018/04/24 16:24:00.601202 Slot 0 is assigned to Job pre
2018/04/24 16:24:00.602202 etcd put (crms/jobstate/pre, running)
2018/04/24 16:24:00.609202 Run Job pre with command [python -c import sys,time; print 'pre'; time.sleep(2); print 'after'; sys.exit(0)]
2018/04/24 16:24:01.602202 etcd put (crms/job/0, ["python","-c","import sys,time; print 0; time.sleep(3); print 0; sys.exit(0)"])
2018/04/24 16:24:01.610202 etcd put (crms/assign/s1/0, )
2018/04/24 16:24:01.610202 Slot 1 is assigned to Job 0
2018/04/24 16:24:01.614202 etcd put (crms/job/1, ["python","-c","import sys,time; print 1; time.sleep(0); print -1; sys.exit(1)"])
2018/04/24 16:24:01.617202 etcd put (crms/assign/s1/1, )
2018/04/24 16:24:01.620202 etcd put (crms/jobstate/0, running)
2018/04/24 16:24:01.620202 etcd put (crms/job/2, ["python","-c","import sys,time; print 2; time.sleep(1); print -2; sys.exit(0)"])
2018/04/24 16:24:01.624202 etcd put (crms/assign/s1/2, )
2018/04/24 16:24:01.629202 Run Job 0 with command [python -c import sys,time; print 0; time.sleep(3); print 0; sys.exit(0)]
2018/04/24 16:24:01.637202 etcd put (crms/job/3, ["python","-c","import sys,time; print 3; time.sleep(1); print -3; sys.exit(1)"])
2018/04/24 16:24:01.638202 etcd put (crms/assign/s1/3, )
2018/04/24 16:24:01.640202 etcd put (crms/job/4, ["python","-c","import sys,time; print 4; time.sleep(2); print -4; sys.exit(0)"])
2018/04/24 16:24:01.643202 etcd put (crms/assign/s1/4, )
2018/04/24 16:24:01.646202 etcd put (crms/job/5, ["python","-c","import sys,time; print 5; time.sleep(0); print -5; sys.exit(1)"])
2018/04/24 16:24:01.649202 etcd put (crms/assign/s1/5, )
2018/04/24 16:24:02.858202 Success to run job pre
2018/04/24 16:24:02.861202 etcd put (crms/jobstate/pre, done)
2018/04/24 16:24:02.875202 Slot 0 is assigned to Job 1
2018/04/24 16:24:02.876202 etcd del (crms/assign/s1/pre, )
`

func TestEqLog(t *testing.T) {
	if err := EqLog(logA, logB, 100 * time.Millisecond); err != nil {
		t.Error(err)
	}
}

func TestParseLogItem(t *testing.T) {
	if it, err := parseLogItem("2018/04/24 11:49:46.979959 Slot 0 is assigned to Job 123"); err != nil {
		t.Error(err)
	} else if *it != (LogItem{
		Time:time.Date(2018, time.April, 24, 11, 49, 46, int(979959 * time.Microsecond), time.UTC),
		Msg: "Slot 0 is assigned to Job 123",
	}) {
		t.Error(it)
	}
}

func TestEqLogWithGolden(t *testing.T) {
	gf, err := DefaultGoldenFile()
	if err != nil {
		t.Fatal(err)
	}
	if path.Base(gf) != "gocrms.TestEqLogWithGolden" {
		t.Error(gf)
	}
	if err := EqLogWithGolden(gf, logA, 100 * time.Millisecond); err != nil {
		t.Error(err)
	}
	if err := EqLogWithGolden(gf, logB, 100 * time.Millisecond); err != nil {
		t.Error(err)
	}
}
