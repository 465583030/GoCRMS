package base

import "github.com/coreos/etcd/clientv3"

type KV struct {
	K string
	V string
}

type GetResponse clientv3.GetResponse

func (resp *GetResponse) KeyValues() []KV {
	kvs := make([]KV, len(resp.Kvs))
	for i, kv := range resp.Kvs {
		k := string(kv.Key)
		v := string(kv.Value)
		kvs[i] = KV{k, v}
	}
	return kvs
}

func (resp *GetResponse) Value() (string, error) {

}
