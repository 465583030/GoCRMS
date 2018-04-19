package base

import (
	"testing"
	"github.com/coreos/etcd/clientv3"
	"time"
	"encoding/json"
)

func TestCrms(t *testing.T) {
	crms, err := NewCrms(clientv3.Config{
		Endpoints: []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	}, 10 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer crms.Close()

	if err = crms.Reset(); err != nil {
		t.Fatal(err)
	}

	// get nodes, should be empty
	nodes, err := crms.Nodes()
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 0 {
		t.Fatal(nodes)
	}
	// now reset works, defer reset so after test data can clean
	defer crms.Reset()

	// watch servers
	serversPutChan := make(chan *Server, 10)
	serversDelChan := make(chan *Server, 10)
	cancelWatchServers := crms.WatchServers(ServerWatchFunc{
		HandlePut: func(server *Server) {
			serversPutChan <- server
		},
		HandleDelete: func(server *Server) {
			serversDelChan <- server
		},
	})

	// test add new servers
	servers := []*Server{
		{
			Name: "sv1",
			SlotCount: 16,
		}, {
			Name: "sv2",
			Closed: true,
		},
	}
	for _, server := range servers {
		if err = crms.UpdateServer(server); err != nil {
			t.Fatal(err)
		}
	}

	// test get servers (together test set servers)
	if servs, err := crms.GetServers(); err != nil {
		t.Fatal(err)
	} else {
		if !isServersEquals(servers, servs) {
			t.Fatal(toJsonString(servs))
		}
	}

	//sv1PutChan := make(chan *Server, 10)
	//sv1DelChan := make(chan *Server, 10)
	//cancelWatchSv1 := crms.WatchServer("sv1", ServerWatchFunc{
	//	HandlePut: func(server *Server) {
	//		sv1PutChan <- server
	//	},
	//	HandleDelete: func(server *Server) {
	//		sv1DelChan <- server
	//	},
	//})

	// test stop one server (as well update existing server)
	if err = crms.StopServer("sv1"); err != nil {
		t.Fatal(err)
	}

	// test get one server
	if sv1, err := crms.GetServer("sv1"); err != nil {
		t.Fatal(err)
	} else {
		if *sv1 != (Server{
			Name: "sv1",
			Closed: true,
		}) {
			t.Fatal(sv1)
		}
	}

	// validate watch servers
	cancelWatchServers.Call()
	close(serversPutChan)
	close(serversDelChan)

	if len(serversPutChan) != 3 {
		t.Error(len(serversDelChan))
	}

	if sv, ok := <-serversPutChan; !ok {
		t.Fatal("should have value in channel")
	} else {
		if *sv != (Server{
			Name: "sv1",
			SlotCount: 16,
		}) {
			t.Fatal(sv)
		}
	}
	if sv, ok := <-serversPutChan; !ok {
		t.Fatal("should have value in channel")
	} else {
		if *sv != (Server{
			Name: "sv2",
			Closed: true,
		}) {
			t.Fatal(sv)
		}
	}
	if sv, ok := <-serversPutChan; !ok {
		t.Fatal("should have value in channel")
	} else {
		if *sv != (Server{
			Name: "sv1",
			Closed: true,
		}) {
			t.Fatal(sv)
		}
	}
	if sv, ok := <-serversPutChan; ok || sv != nil {
		t.Fatalf("ok: %v; sv: %v", ok, sv)
	}

	if len(serversDelChan) != 0 {
		t.Error(len(serversDelChan))
	}
	if sv, ok := <-serversDelChan; ok || sv != nil {
		t.Fatalf("ok: %v; sv: %v", ok, sv)
	}


	// test create job
	//crms.CreateJob()
}

func isServersEquals(ss1, ss2 []*Server) bool {
	if len(ss1) != len(ss2) {
		return false
	}
	for i, s1 := range ss1 {
		if s2 := ss2[i]; *s1 != *s2 {
			return false
		}
	}
	return true
}

func toJsonString(v interface{}) string {
	bs, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(bs)
}
