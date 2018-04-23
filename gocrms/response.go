package gocrms

import (
	"github.com/coreos/etcd/clientv3"
	"errors"
)

type KV struct {
	K string
	V string
}

type GetResponse struct {
	*clientv3.GetResponse
}

func (resp GetResponse) KeyValues() []KV {
	kvs := make([]KV, len(resp.Kvs))
	for i, kv := range resp.Kvs {
		k := string(kv.Key)
		v := string(kv.Value)
		kvs[i] = KV{k, v}
	}
	return kvs
}

func (resp GetResponse) Value() (string, error) {
	if len(resp.Kvs) != 1 {
		return "", errors.New("clientv3.GetResponse's KVs' len is not 1")
	}
	return string(resp.Kvs[0].Value), nil
}

func (resp *GetResponse) Len() int {
	return len(resp.Kvs)
}
