package base

import (
	"testing"
	"github.com/coreos/etcd/clientv3"
	"time"
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
		t.Fatal(delResp.Deleted)
	}
	if len(delResp.PrevKvs) != 3 {
		t.Fatal(delResp.PrevKvs)
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
