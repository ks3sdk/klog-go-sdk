package klog

import (
	"github.com/golang/protobuf/proto"
	v2 "ksyun.com/cbd/klog-sdk/internal/signer"
	pb "ksyun.com/cbd/klog-sdk/protobuf"
	"ksyun.com/cbd/klog-sdk/service"
	"net/url"
	"sync"
)

var oprw sync.Mutex

type Klog struct {
	*service.Service
}

// Used for custom service initialization logic
var initService func(*service.Service)

// Used for custom request initialization logic
var initRequest func(*service.Request)

func New(config *service.Config) *Klog {
	s := &service.Service{
		Config:     service.DefaultConfig.Merge(config),
		APIVersion: "0.1.0",
	}

	s.Initialize()

	s.Handlers.Sign.PushBack(v2.Sign)

	return &Klog{s}
}

// newRequest creates a new request for a S3 operation and runs any
// custom request initialization.
func (k *Klog) newRequest(op *service.Operation, inputs []byte) *service.Request {
	req := service.NewRequest(k.Service, op, inputs)

	// Run custom request initialization if present
	if initRequest != nil {
		initRequest(req)
	}

	return req
}

//生成请求内容
func (k *Klog) PutLogsRequest(input []byte, params *url.Values) (req *service.Request) {
	oprw.Lock()
	defer oprw.Unlock()

	opPutLogs := &service.Operation{
		Name:   "PutLogs",
		Method: "POST",
		Path:   "/PutLogs",
		Params: params,
	}

	req = k.newRequest(opPutLogs, input)

	return
}

// 底层API，一次上传多条log到指定日志池。是同步调用。
func (k *Klog) PutLogs(input *pb.LogGroup, targetProject, targetLogPool string) error {
	params := &url.Values{}
	params.Add("ProjectName", targetProject)
	params.Add("LogPoolName", targetLogPool)

	bb, err := proto.Marshal(input)
	if err != nil {
		return err
	}
	req := k.PutLogsRequest(bb, params)
	err = req.Send()
	return err
}
