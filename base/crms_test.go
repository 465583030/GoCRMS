package base

import (
	"testing"
	"github.com/coreos/etcd/clientv3"
	"time"
)

func TestCrms(t *testing.T) {


	t.SkipNow()



	crms, err := NewCrms(clientv3.Config{
		Endpoints: []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	}, 10 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer crms.Close()

	crms.Reset()

	//crms.UpdateServer(nil)
}
