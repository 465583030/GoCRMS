package base

import (
	"github.com/coreos/etcd/clientv3"
	"context"
	"time"
)



func ParseGetResponse(resp *clientv3.GetResponse) []KV {
	kvs := make([]KV, len(resp.Kvs))
	for i, kv := range resp.Kvs {
		k := string(kv.Key)
		v := string(kv.Value)
		kvs[i] = KV{k, v}
	}
	return kvs
}

type Etcd struct {
	*clientv3.Client
	requestTimeout     time.Duration
}

func (etcd *Etcd) Get(key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), etcd.requestTimeout)
	defer cancel()
	return etcd.Client.Get(ctx, key, opts...)
}

func (etcd *Etcd) GetWithPrefix(keyPrefix string) (*clientv3.GetResponse, error) {
	return etcd.Get(keyPrefix, clientv3.WithPrefix())
}

func (etcd *Etcd) GetWithRange(startKey string, endKey string) (*clientv3.GetResponse, error) {
	return etcd.Get(startKey, clientv3.WithRange(endKey))
}

func (etcd *Etcd) Watch(key string, opts ...clientv3.OpOption) (clientv3.WatchChan, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	return etcd.Client.Watch(ctx, key, opts...), cancel
}

func (etcd *Etcd) WatchWithPrefix(keyPrefix string) (clientv3.WatchChan, context.CancelFunc) {
	return etcd.Watch(keyPrefix, clientv3.WithPrefix())
}

func (etcd *Etcd) WatchWithRange(startKey string, endKey string) (clientv3.WatchChan, context.CancelFunc) {
	return etcd.Watch(startKey, clientv3.WithRange(endKey))
}

func (etcd *Etcd) Delete(key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), etcd.requestTimeout)
	defer cancel()
	return etcd.Client.Delete(ctx, key, opts...)
}

func (etcd *Etcd) DeleteWithPrefix(keyPrefix string) (*clientv3.DeleteResponse, error) {
	return etcd.Delete(keyPrefix, clientv3.WithPrefix())
}

func (etcd *Etcd) DeleteWithRange(startKey string, endKey string) (*clientv3.DeleteResponse, error) {
	return etcd.Delete(startKey, clientv3.WithRange(endKey))
}

func (etcd *Etcd) GetNodes(prefix string) ([]KV, error) {
	resp, err := etcd.Get(prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	return ParseGetResponse(resp), nil
}
