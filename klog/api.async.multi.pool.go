package klog

import (
	"fmt"
	pb "github.com/ks3sdk/klog-go-sdk/protobuf"
	"github.com/ks3sdk/klog-go-sdk/service"
	"sync"
)

type AsyncMultiPoolClient struct {
	AsyncClients sync.Map
	KLogConfig   *service.Config
	Options      *AsyncMultiPoolClientOptions
}

type AsyncMultiPoolClientOptions struct {
	Callback            func(*pb.Log, uint64, error)
	DropIfPoolNotExists bool
	QueueSize           int
}

func NewAsyncMultiPoolClient(options *AsyncMultiPoolClientOptions, kLogConfig *service.Config) *AsyncMultiPoolClient {
	return &AsyncMultiPoolClient{
		AsyncClients: sync.Map{},
		KLogConfig:   kLogConfig,
		Options:      options,
	}
}

func (o *AsyncMultiPoolClient) PushLog(projectName, logPoolName string, log *pb.Log) uint64 {
	var client *AsyncClient
	key := fmt.Sprintf("%s\001%s", projectName, logPoolName)

	if itf, ok := o.AsyncClients.Load(key); !ok {
		client = NewAsyncClient(&AsyncClientOptions{
			ProjectName:         projectName,
			LogPoolName:         logPoolName,
			Callback:            o.Options.Callback,
			DropIfPoolNotExists: o.Options.DropIfPoolNotExists,
			QueueSize:           o.Options.QueueSize,
		}, o.KLogConfig)
		o.AsyncClients.Store(key, client)
	} else {
		client, _ = itf.(*AsyncClient)
	}
	return client.PushLog(log)
}

func (o *AsyncMultiPoolClient) Stop() {
	o.AsyncClients.Range(func(_, clientInterface interface{}) bool {
		client, _ := clientInterface.(*AsyncClient)
		client.Stop(false)
		return true
	})
	o.AsyncClients.Range(func(_, clientInterface interface{}) bool {
		client, _ := clientInterface.(*AsyncClient)
		client.wg.Wait()
		return true
	})
}
