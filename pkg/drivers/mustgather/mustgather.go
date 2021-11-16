package mustgather

import (
	"context"

	"github.com/k3s-io/kine/pkg/server"
	"github.com/sirupsen/logrus"
)

type MustGather struct {
}

func New() *MustGather {
	return &MustGather{}
}

func (mg *MustGather) Start(ctx context.Context) error {
	return nil
}

func (mg *MustGather) Get(ctx context.Context, key string, revision int64) (revRet int64, kvRet *server.KeyValue, errRet error) {
	logrus.Info("Get has been called")

	rev := int64(1)
	keyValue := &server.KeyValue{
		Key:            "key",
		CreateRevision: revision,
		ModRevision:    revision,
		Value:          []byte("value"),
		Lease:          100,
	}

	return rev, keyValue, nil
}

func (mg *MustGather) Create(ctx context.Context, key string, value []byte, lease int64) (revRet int64, errRet error) {
	logrus.Info("Create has been called")
	return
}

func (mg *MustGather) Delete(ctx context.Context, key string, revision int64) (revRet int64, kvRet *server.KeyValue, deletedRet bool, errRet error) {
	logrus.Info("Delete has been called")
	return
}

func (mg *MustGather) List(ctx context.Context, prefix, startKey string, limit, revision int64) (revRet int64, kvRet []*server.KeyValue, errRet error) {
	logrus.Info("List has been called")
	defer func() {
		logrus.Tracef("LIST %s, start=%s, limit=%d, rev=%d => rev=%d, kvs=%d, err=%v", prefix, startKey, limit, revision, revRet, len(kvRet), errRet)
	}()

	// rev, events, err := mg.log.List(ctx, prefix, startKey, limit, revision, false)
	// if err != nil {
	// 	return 0, nil, err
	// }
	// if revision == 0 && len(events) == 0 {
	// 	// if no revision is requested and no events are returned, then
	// 	// get the current revision and relist.  Relist is required because
	// 	// between now and getting the current revision something could have
	// 	// been created.
	// 	currentRev, err := mg.log.CurrentRevision(ctx)
	// 	if err != nil {
	// 		return 0, nil, err
	// 	}
	// 	return mg.List(ctx, prefix, startKey, limit, currentRev)
	// } else if revision != 0 {
	// 	rev = revision
	// }

	kvs := make([]*server.KeyValue, 0, 1)
	// for _, event := range events {
	// 	kvs = append(kvs, event.KV)
	// }
	rev := int64(1)
	return rev, kvs, nil
}

func (mg *MustGather) Count(ctx context.Context, prefix string) (revRet int64, count int64, err error) {
	logrus.Info("Count has been called")
	return 1, 3, nil
}

func (mg *MustGather) Update(ctx context.Context, key string, value []byte, revision, lease int64) (revRet int64, kvRet *server.KeyValue, updateRet bool, errRet error) {
	logrus.Info("Update has been called")

	updateEvent := &server.Event{
		KV: &server.KeyValue{
			Key:            key,
			CreateRevision: 1,
			Value:          value,
			Lease:          lease,
		},
	}

	return 1, updateEvent.KV, true, nil
}

func (mg *MustGather) Watch(ctx context.Context, prefix string, revision int64) <-chan []*server.Event {
	logrus.Info("Watch has been called")

	result := make(chan []*server.Event, 100)
	return result
}

func (mg *MustGather) DbSize(ctx context.Context) (int64, error) {
	logrus.Info("DbSize has been called")
	return 10, nil
}
