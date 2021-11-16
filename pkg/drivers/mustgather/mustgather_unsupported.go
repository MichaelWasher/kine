package mustgather

import (
	"context"
	"fmt"

	"github.com/k3s-io/kine/pkg/server"
	"github.com/sirupsen/logrus"
)

func (mg *MustGather) Create(ctx context.Context, key string, value []byte, lease int64) (revRet int64, errRet error) {
	logrus.Debugf("Create has been called for %s", key)
	// NOTE: Cannot return an error as this will crash the KubeAPI on boot. But this is not a supported function of Must-Gather driver
	return
}

func (mg *MustGather) Delete(ctx context.Context, key string, revision int64) (revRet int64, kvRet *server.KeyValue, deletedRet bool, errRet error) {
	logrus.Debugf("Delete has been called for %s", key)
	return 0, nil, false, fmt.Errorf("the delete function is not supported with the mustgather driver")
}

func (mg *MustGather) Count(ctx context.Context, prefix string) (revRet int64, count int64, err error) {
	logrus.Debugf("Count has been called with prefix %s", prefix)
	return 1, 0, fmt.Errorf("the count function is not supported with mustgather driver")
}

func (mg *MustGather) Update(ctx context.Context, key string, value []byte, revision, lease int64) (revRet int64, kvRet *server.KeyValue, updateRet bool, errRet error) {
	logrus.Debugf("Update has been called for %s", key)
	return 1, nil, false, fmt.Errorf("the update function is not supported with mustgather driver")
}

func (mg *MustGather) Watch(ctx context.Context, prefix string, revision int64) <-chan []*server.Event {
	logrus.Debugf("Watch has been called with prefix %s", prefix)
	return nil
}

func (mg *MustGather) DbSize(ctx context.Context) (int64, error) {
	logrus.Debug("DbSize has been called")
	return 0, nil
}
