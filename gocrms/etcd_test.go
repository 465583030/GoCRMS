package gocrms

import (
	"testing"
	"github.com/coreos/etcd/clientv3"
	"time"
	"fmt"
	"errors"
)

func TestEtcd(t *testing.T) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints: []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	etcd := Etcd{cli, 10 * time.Second}
	defer etcd.Close()

	// make sure test/** is not existed before and after testing
	etcd.DeleteWithPrefix("test/")
	defer etcd.DeleteWithPrefix("test/")

	// test put temp node (call go routine early to shorter time test wait time)
	const maxTimeout = 9 * time.Second
	const timeoutSecond = 5
	const timeout = time.Duration(timeoutSecond) * time.Second
	leaseID1, cancelTimeout1, err := etcd.Timeout(timeoutSecond)
	if err != nil {
		t.Fatal(err)
	}
	defer cancelTimeout1()
	err = etcd.PutTempNode("test/Tmp1", "1", leaseID1)
	if err != nil {
		t.Fatal(err)
	}

	// test get temp node (5s), without any keep alive action, the actual life should be 5s
	tmpNode1Life, err1Chan := goGetNodeDuration(&etcd, timeout, maxTimeout, "test/Tmp1", "1", -1,nil)

	// test get temp node (5s),  keep alive once on 3s, the actual life should extend to 8s
	leaseID2, cancelTimeout2, err := etcd.Timeout(timeoutSecond)
	if err != nil {
		t.Fatal(err)
	}
	defer cancelTimeout2()
	err = etcd.PutTempNode("test/Tmp2", "2", leaseID2)
	if err != nil {
		t.Fatal(err)
	}
	tmpNode2Life, err2Chan := goGetNodeDuration(&etcd, timeout, maxTimeout, "test/Tmp2", "2",
		3 * time.Second, func() error {
			return etcd.KeepAliveOnce(leaseID2)
		})

	// test get temp node (1s), keep alive forever, only cancel can make it close
	leaseID3, cancelTimeout3, err := etcd.Timeout(1)
	if err != nil {
		t.Fatal(err)
	}
	defer cancelTimeout3()
	err = etcd.PutTempNode("test/Tmp3", "3", leaseID3)
	if err != nil {
		t.Fatal(err)
	}
	cancel3, err := etcd.KeepAliveForever(leaseID3)
	defer cancel3()
	if err != nil {
		t.Fatal(err)
	}
	tmpNode3Life, err3Chan := goGetNodeDuration(&etcd, 1 * time.Second, maxTimeout, "test/Tmp3", "3",
		6 * time.Second, func() error {
			cancel3()
			return nil
		})

	// test get temp node (1s), keep alive forever, only cancel can make it close
	//etcd.KeepAlive
	//tmpNode4Life, err4Chan := goPutTempNode(&etcd, 5, maxTimeout, "test/Tmp4", "4",
	//	nil, nil)

	// test get non-exist node
	getResp, err := etcd.Get("test/not exist")
	if err != nil {
		t.Fatal(err)
	}
	if len(getResp.Kvs) != 0 {
		t.Fatal("get a node by non-exist key should return a empty kvs slice")
	}

	// test delete non-exist node
	prevValue, prevExist, err := etcd.DeleteAndReturnPrevValue("not exist")
	if err != nil {
		t.Fatal(err)
	}
	if prevExist || prevValue != "" {
		t.Fatal("Delete count should be zero for non-exist node, but actual count is not")
	}

	// watch non-exist node
	rchA, cancelA := etcd.Watch("test/a")
	defer cancelA()
	resultA := make(chan []WatchEvtData)
	go handle(rchA, resultA)

	// test put a new node
	putResp, err := etcd.Put("test/a", "1", clientv3.WithPrevKV())
	if err != nil {
		t.Fatal(err)
	}
	if putResp.PrevKv != nil {
		t.Fatal("not as expected, actual is", putResp.PrevKv)
	}

	// test get an existed node
	getResp, err = etcd.Get("test/a")
	if err != nil {
		t.Fatal(err)
	}
	if getResp.Count != 1 || len(getResp.Kvs) != 1 {
		t.Fatal("get count should be 1, but actual is", getResp.Count)
	}
	kv := getResp.Kvs[0]
	if string(kv.Key) != "test/a" || string(kv.Value) != "1" {
		t.Fatalf("key value is not expected, actual (%s, %s)", string(kv.Key), string(kv.Value))
	}

	// test put an exist node without specify prev option
	putResp, err = etcd.Put("test/a", "1.5")
	if err != nil {
		t.Fatal(err)
	}
	if putResp.PrevKv != nil {
		t.Fatal("PrevKvs should be nil without specify prev option, no matter previous exist or not. But actual is",
			putResp.PrevKv)
	}

	// test put an exist node with specify prev option
	prevValue, prevExist, err = etcd.PutAndReturnPrevValue("test/a", "2")
	if err != nil {
		t.Fatal(err)
	}
	if !prevExist {
		t.Fatal("prevExist should exist, but actual is not")
	}
	if prevValue != "1.5" {
		t.Fatal("PrevKvs should be the previous for put an existed node, but actual is:", prevValue)
	}

	// put more node
	_, err = etcd.Put("test/a/b", "23")
	if err != nil {
		t.Fatal(err)
	}
	_, err = etcd.Put("test/a/b/1", "234")
	if err != nil {
		t.Fatal(err)
	}

	// test delete one exist node
	prevValue, prevExist, err = etcd.DeleteAndReturnPrevValue("test/a/b")
	if err != nil {
		t.Fatal(err)
	}
	if !prevExist {
		t.Fatal("previous should exist, but actual not")
	}
	if prevValue != "23" {
		t.Fatal("Previous value is not expected, actual is", prevValue)
	}

	// test get node with prefix, prefix itself will be got as well
	getResp, err = etcd.GetWithPrefix("test/a")
	if err != nil {
		t.Fatal(err)
	}
	if getResp.Count != 2 || len(getResp.Kvs) != 2 {
		t.Fatal("get count should be 2, but actual is", getResp.Count)
	}
	kv0 := getResp.Kvs[0]
	if string(kv0.Key) != "test/a" || string(kv0.Value) != "2" {
		t.Fatalf("key value is not expected, actual (%s, %s)", string(kv0.Key), string(kv0.Value))
	}
	kv1 := getResp.Kvs[1]
	if string(kv1.Key) != "test/a/b/1" || string(kv1.Value) != "234" {
		t.Fatalf("key value is not expected, actual (%s, %s)", string(kv1.Key), string(kv1.Value))
	}

	_, err = etcd.Put("test/a/b/2", "22")
	if err != nil {
		t.Fatal(err)
	}
	_, err = etcd.Put("test/a/b/09", "0909")
	if err != nil {
		t.Fatal(err)
	}
	_, err = etcd.Put("test/a/b/10", "1010")
	if err != nil {
		t.Fatal(err)
	}
	_, err = etcd.Put("test/a/b/11", "1111")
	if err != nil {
		t.Fatal(err)
	}

	// watch test/a/b/** after put something, the previous put event will not receive
	rchAB, cancelAB := etcd.WatchWithPrefix("test/a/b/")
	defer cancelAB()
	resultAB := make(chan []WatchEvtData)
	go handle(rchAB, resultAB)

	_, err = etcd.Put("test/a/b/21", "2121")
	if err != nil {
		t.Fatal(err)
	}

	// test get range
	getResp, err = etcd.GetWithRange("test/a/b/09", "test/a/b/11")
	if err != nil {
		t.Fatal(err)
	}
	if getResp.Count != 3 || len(getResp.Kvs) != 3 {
		t.Fatal("get count should be 2, but actual is", getResp.Count)
	}
	kv = getResp.Kvs[0]
	if string(kv.Key) != "test/a/b/09" || string(kv.Value) != "0909" {
		t.Fatalf("key value is not expected, actual (%s, %s)", string(kv.Key), string(kv.Value))
	}
	kv = getResp.Kvs[1]
	if string(kv.Key) != "test/a/b/1" || string(kv.Value) != "234" {
		t.Fatalf("key value is not expected, actual (%s, %s)", string(kv.Key), string(kv.Value))
	}
	kv = getResp.Kvs[2]
	if string(kv.Key) != "test/a/b/10" || string(kv.Value) != "1010" {
		t.Fatalf("key value is not expected, actual (%s, %s)", string(kv.Key), string(kv.Value))
	}

	// test delete range
	delResp, err := etcd.DeleteWithRange("test/a/b/09", "test/a/b/11")
	if err != nil {
		t.Fatal(err)
	}
	if delResp.Deleted != 3 {
		t.Fatal("Delete count should be 3, but actual is", delResp.Deleted)
	}
	if delResp.PrevKvs != nil {
		t.Fatal("as not specify prevKV, PrevKvs should nil, but actual:", delResp.PrevKvs)
	}

	// delete from key and return previous
	delResp, err = etcd.Delete("test/a/b/11", clientv3.WithPrevKV(), clientv3.WithFromKey())
	if err != nil {
		t.Fatal(err)
	}
	if delResp.Deleted != 3 {
		t.Error(delResp.Deleted)
	}
	if len(delResp.PrevKvs) != 3 {
		t.Error(delResp.PrevKvs)
	}
	kv = delResp.PrevKvs[0]
	if string(kv.Key) != "test/a/b/11" || string(kv.Value) != "1111" {
		t.Fatal(kv)
	}
	kv = delResp.PrevKvs[1]
	if string(kv.Key) != "test/a/b/2" || string(kv.Value) != "22" {
		t.Fatal(kv)
	}
	kv = delResp.PrevKvs[2]
	if string(kv.Key) != "test/a/b/21" || string(kv.Value) != "2121" {
		t.Fatal(kv)
	}

	// check watch A's result, call cancelA to stop watch so that result will be return from channel
	cancelA()
	resA, ok := <-resultA
	if !ok {
		t.Fatal(ok)
	}
	const lenA = 3
	if len(resA) != lenA {
		t.Fatalf("len is not as expected, actual len: %d\nActual: %v", len(resA), resA)
	}
	var arrA [lenA]WatchEvtData
	copy(arrA[:], resA)
	if arrA != [...]WatchEvtData{
		{
			Type: WatchEvtPut,
			KV: KV{
				K: "test/a",
				V: "1",
			},
		}, {
			Type: WatchEvtPut,
			KV: KV{
				K: "test/a",
				V: "1.5",
			},
		}, {
			Type: WatchEvtPut,
			KV: KV{
				K: "test/a",
				V: "2",
			},
		},
	} {
		t.Fatal(resA)
	}

	// check watch AB's result, call cancelAB to stop watch so that result will be return from channel
	cancelAB()
	resAB, ok := <-resultAB
	if !ok {
		t.Fatal(ok)
	}
	const lenAB = 7
	if len(resAB) != lenAB {
		t.Fatalf("len is not as expected, actual len: %d\nActual: %v", len(resAB), resAB)
	}
	var arrAB [lenAB]WatchEvtData
	copy(arrAB[:], resAB)
	if arrAB != [...]WatchEvtData{
		{
			Type: WatchEvtPut,
			KV: KV{
				K: "test/a/b/21",
				V: "2121",
			},
		}, {
			Type: WatchEvtDelete,
			KV: KV{
				K: "test/a/b/09",
				V: "",
			},
		}, {
			Type: WatchEvtDelete,
			KV: KV{
				K: "test/a/b/1",
				V: "",
			},
		}, {
			Type: WatchEvtDelete,
			KV: KV{
				K: "test/a/b/10",
				V: "",
			},
		}, {
			Type: WatchEvtDelete,
			KV: KV{
				K: "test/a/b/11",
				V: "",
			},
		}, {
			Type: WatchEvtDelete,
			KV: KV{
				K: "test/a/b/2",
				V: "",
			},
		}, {
			Type: WatchEvtDelete,
			KV: KV{
				K: "test/a/b/21",
				V: "",
			},
		},
	} {
		t.Fatal(resAB)
	}

	// verify temp node life
	select {
	case life, ok := <-tmpNode1Life:
		if !ok {
			t.Error(ok)
		} else if life < 4500 * time.Millisecond || life > 5500 * time.Millisecond {
			t.Error(life)
		}
	case err := <- err1Chan:
		t.Error(err)
	case <- time.After(maxTimeout):
		t.Error("time out")
	}
	select {
	case life, ok := <-tmpNode2Life:
		if !ok {
			t.Error(ok)
		} else if life < 7500 * time.Millisecond || life > 8500 * time.Millisecond {
			t.Error(life)
		}
	case err := <- err2Chan:
		t.Error(err)
	case <- time.After(maxTimeout):
		t.Error("time out")
	}
	// test keep alive once/forever after the node life, should get an error
	const errNoLease = "etcdserver: requested lease not found"
	if err := etcd.KeepAliveOnce(leaseID1); err == nil || err.Error() != errNoLease {
		t.Error(err)
	}
	//if _, err := etcd.KeepAliveForever(leaseID1); err == nil || err.Error() != errNoLease {
	//	t.Error(err)
	//}

	select {
	case life := <-tmpNode3Life:
		if life < 6 * time.Second || life >= 8500 * time.Millisecond {
			t.Error(life)
		}
	case err := <- err3Chan:
		// should not go into this case
		t.Error(err)
	case <- time.After(maxTimeout):
		t.Error("time out")
	}


	// test get temp node (5s),  keep alive once after the node life, should get an error


	//etcd.KeepAliveOnce(leaseID)

	// test get nodes that has value
}

func handle(rch clientv3.WatchChan, result chan<- []WatchEvtData) {
	evts := make([]WatchEvtData, 0, 8)
	for wresp := range rch {
		for _, evt := range wresp.Events {
			evts = append(evts, WatchEvtData{
				Type: int(evt.Type),
				KV: KV{
					K: string(evt.Kv.Key),
					V: string(evt.Kv.Value),
				},
			})
		}
	}
	result <- evts
	close(result)
}

func goPutTempNode(etcd *Etcd, timeoutSecond int64, maxTimeout time.Duration, key, value string,
	onLast2Seconds, afterNodeDeleted func(id clientv3.LeaseID) error) (<-chan time.Duration, <-chan error) {
	lifeChan := make(chan time.Duration)
	errChan := make(chan error)
	go func() {
		life, err := putTempNode(etcd, timeoutSecond, maxTimeout, key, value, onLast2Seconds, afterNodeDeleted)
		if err != nil {
			errChan <- err
		} else {
			lifeChan <- life
		}
	}()
	return lifeChan, errChan
}

func putTempNode(etcd *Etcd, timeoutSecond int64, maxTimeout time.Duration, key, value string,
	onLast2Seconds, afterNodeDeleted func(id clientv3.LeaseID) error) (time.Duration, error) {
	timeout := time.Duration(timeoutSecond) * time.Second
	leaseID, cancelTimeout, err := etcd.Timeout(timeoutSecond)
	defer cancelTimeout()
	if err != nil {
		return 0, err
	}
	err = etcd.PutTempNode(key, value, leaseID)
	if err != nil {
		return 0, err
	}
	tmpNodeStep := timeout / 100
	tmpNodeLife := time.Duration(0)
	for ; tmpNodeLife < maxTimeout; tmpNodeLife += tmpNodeStep {
		getResp, err := etcd.Get(key)
		if err != nil {
			return tmpNodeLife, err
		}
		resp := GetResponse{getResp}
		if resp.Len() == 0 {
			if afterNodeDeleted == nil {
				break
			}
			if err := afterNodeDeleted(leaseID); err != nil {
				return tmpNodeLife, err
			}
			afterNodeDeleted = nil
		} else {
			v, err := resp.Value()
			if err != nil {
				return tmpNodeLife, err
			}
			if v != value {
				return tmpNodeLife, errors.New(
					fmt.Sprintf("expected %v but actual %v", value, v))
			}
			if tmpNodeLife == timeout - 2 * time.Second && onLast2Seconds != nil {
				if err := onLast2Seconds(leaseID); err != nil {
					return tmpNodeLife, err
				}
			}
		}
		time.Sleep(tmpNodeStep)
	}
	return tmpNodeLife, err
}

func goGetNodeDuration(etcd *Etcd, timeout, maxTimeout time.Duration, key, value string,
	cbTime time.Duration, callback func() error) (<-chan time.Duration, <-chan error) {
	lifeChan := make(chan time.Duration)
	errChan := make(chan error)
	go func() {
		life, err := getNodeDuration(etcd, timeout, maxTimeout, key, value, cbTime, callback)
		if err != nil {
			errChan <- err
		} else {
			lifeChan <- life
		}
	}()
	return lifeChan, errChan
}

func getNodeDuration(etcd *Etcd, timeout, maxTimeout time.Duration, key, value string,
	cbTime time.Duration, callback func() error) (time.Duration, error) {
	tmpNodeStep := timeout / 100
	tmpNodeLife := time.Duration(0)
	for ; tmpNodeLife < maxTimeout; tmpNodeLife += tmpNodeStep {
		getResp, err := etcd.Get(key)
		if err != nil {
			return tmpNodeLife, err
		}
		resp := GetResponse{getResp}
		if resp.Len() == 0 {
			break
		} else {
			v, err := resp.Value()
			if err != nil {
				return tmpNodeLife, err
			}
			if v != value {
				return tmpNodeLife, errors.New(
					fmt.Sprintf("expected %v but actual %v", value, v))
			}
			if callback != nil && tmpNodeLife >= cbTime {
				if err := callback(); err != nil {
					return tmpNodeLife, err
				}
				callback = nil
			}
		}
		time.Sleep(tmpNodeStep)
	}
	return tmpNodeLife, nil
}