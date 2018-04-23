package gocrms

import (
	"github.com/coreos/etcd/clientv3"
	"context"
	"time"
)

type Etcd struct {
	*clientv3.Client
	requestTimeout     time.Duration
}

// Get retrieves keys.
// By default, Get will return the value for "key", if any.
// When passed WithRange(end), Get will return the keys in the range [key, end).
// When passed WithFromKey(), Get returns keys greater than or equal to key.
// When passed WithRev(rev) with rev > 0, Get retrieves keys at the given revision;
// if the required revision is compacted, the request will fail with ErrCompacted .
// When passed WithLimit(limit), the number of returned keys is bounded by limit.
// When passed WithSort(), the keys will be sorted.
func (etcd *Etcd) Get(key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), etcd.requestTimeout)
	defer cancel()
	return etcd.Client.Get(ctx, key, opts...)
}

func (etcd *Etcd) GetWithPrefix(keyPrefix string) (*clientv3.GetResponse, error) {
	return etcd.Get(keyPrefix, clientv3.WithPrefix())
}

func (etcd *Etcd) GetWithRange(startKey, endKey string) (*clientv3.GetResponse, error) {
	return etcd.Get(startKey, clientv3.WithRange(endKey))
}

func (etcd *Etcd) Watch(key string, opts ...clientv3.OpOption) (clientv3.WatchChan, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	return etcd.Client.Watch(ctx, key, opts...), cancel
}

func (etcd *Etcd) WatchWithPrefix(keyPrefix string) (clientv3.WatchChan, context.CancelFunc) {
	return etcd.Watch(keyPrefix, clientv3.WithPrefix())
}

func (etcd *Etcd) WatchWithRange(startKey, endKey string) (clientv3.WatchChan, context.CancelFunc) {
	return etcd.Watch(startKey, clientv3.WithRange(endKey))
}

func (etcd *Etcd) Delete(key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), etcd.requestTimeout)
	defer cancel()
	return etcd.Client.Delete(ctx, key, opts...)
}

func (etcd *Etcd) DeleteAndReturnPrevValue(key string) (prevValue string, exist bool, err error) {
	resp, err := etcd.Delete(key, clientv3.WithPrevKV())
	if err != nil {
		return
	}
	if exist = resp.Deleted > 0 && len(resp.PrevKvs) > 0; exist {
		prevValue = string(resp.PrevKvs[0].Value)
	}
	return
}


func (etcd *Etcd) DeleteWithPrefix(keyPrefix string) (*clientv3.DeleteResponse, error) {
	return etcd.Delete(keyPrefix, clientv3.WithPrefix())
}

func (etcd *Etcd) DeleteWithRange(startKey, endKey string) (*clientv3.DeleteResponse, error) {
	return etcd.Delete(startKey, clientv3.WithRange(endKey))
}

// put doesn't support opts: clientv3.WithPrefix(), clientv3.WithRange(endKey)
// to get the previous value, use use option WithPrevKV
func (etcd *Etcd) Put(key, value string, opts ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), etcd.requestTimeout)
	defer cancel()
	return etcd.Client.Put(ctx, key, value, opts...)
}

func (etcd *Etcd) PutTempNode(key, value string, leaseID clientv3.LeaseID) error {
	_, err := etcd.Put(key, value, clientv3.WithLease(leaseID))
	return err
}

func (etcd *Etcd) PutAndReturnPrevValue(key, value string) (prevValue string, exist bool, err error) {
	resp, err := etcd.Put(key, value, clientv3.WithPrevKV())
	if err != nil {
		return
	}
	if exist = resp.PrevKv != nil; exist {
		prevValue = string(resp.PrevKv.Value)
	}
	return
}

func (etcd *Etcd) Timeout(seconds int64) (id clientv3.LeaseID, cancel context.CancelFunc, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), etcd.requestTimeout)
	if grantResp, err := etcd.Grant(ctx, seconds); err == nil {
		id = grantResp.ID
	}
	return
}

func (etcd *Etcd) KeepAliveOnce(id clientv3.LeaseID) error {
	ctx, cancel := context.WithTimeout(context.Background(), etcd.requestTimeout)
	defer cancel()
	_, err := etcd.Client.KeepAliveOnce(ctx, id)
	return err
}

func (etcd *Etcd) KeepAliveForever(id clientv3.LeaseID) (context.CancelFunc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), etcd.requestTimeout)
	_, err := etcd.Client.KeepAlive(ctx, id)
	return cancel, err
}
