package test

import (
	"flag"
	"github.com/ks3sdk/klog-go-sdk/credentials"
	"github.com/ks3sdk/klog-go-sdk/klog"
	pb "github.com/ks3sdk/klog-go-sdk/protobuf"
	"github.com/ks3sdk/klog-go-sdk/service"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

//测试环境
const (
	AK = "AKLTd6mJ5q6ETWKJCtrKvA6XoA"
	SK = "OM29D2LynDwoEnsYdfJNFoC7MBgdVqH830joLn7t5OnJnkIDeeYzKjCuHlcN7Bzy7g=="
	//Endpoint = "klog-cn-beijing-internal.ksyun.com"
	Endpoint = "127.0.0.1:8010"
)

func init() {
	_ = flag.Set("logtostderr", "true")
	_ = flag.Set("v", "2")
}

func makeLog() *pb.Log {
	return &pb.Log{
		Time: time.Now().Unix(),
		Contents: []*pb.Log_Content{
			{
				Key:   "key1",
				Value: "test1",
			}, {
				Key:   "key2",
				Value: "test2",
			},
		},
	}
}

func makeLogGroup() *pb.LogGroup {
	logLen := 10
	logs := make([]*pb.Log, 0)
	for i := 0; i < logLen; i++ {
		logs = append(logs, makeLog())
	}

	lg := &pb.LogGroup{
		Logs:     logs,
		Reserved: "mock reserved",
		Filename: "mock filename",
		Source:   "mock source",
	}
	return lg
}

func TestClient(t *testing.T) {
	a := assert.New(t)
	kk := klog.New(&service.Config{
		Credentials: credentials.NewStaticCredentials(AK, SK, ""),
		Endpoint:    Endpoint,
		DisableSSL:  true,
		Logger:      new(service.StdOutLogger),
		Debug:       true,
	})

	lg := makeLogGroup()

	err := kk.PutLogs(lg, "basic", "basic")
	a.Nil(err)
}

func TestAsyncClient(t *testing.T) {
	a := assert.New(t)
	kk := klog.NewAsyncClient(
		&klog.AsyncClientOptions{
			ProjectName:         "basic",
			LogPoolName:         "basic",
			DropIfPoolNotExists: true,
			Callback: func(log *pb.Log, seqNo uint64, err error) {
				a.Equal(err, nil)
			},
		},
		&service.Config{
			Credentials: credentials.NewStaticCredentials(AK, SK, ""),
			Endpoint:    Endpoint,
			DisableSSL:  true,
			Logger:      new(service.StdOutLogger),
			Debug:       true,
		},
	)

	seqNo := kk.PushLog(makeLog())
	a.Equal(seqNo, uint64(1))
	time.Sleep(time.Duration(3) * time.Second)
	kk.Stop(true)
}

func TestAsyncClientDrop(t *testing.T) {
	a := assert.New(t)
	kk := klog.NewAsyncClient(
		&klog.AsyncClientOptions{
			ProjectName:         "notExist",
			LogPoolName:         "notExist",
			DropIfPoolNotExists: true,
			Callback: func(log *pb.Log, seqNo uint64, err error) {
				isDropErr := klog.IsError(err, klog.UserNotExist) || klog.IsError(err, klog.ProjectOrLogPoolNotExist)
				a.Equal(isDropErr, true)
			},
		},
		&service.Config{
			Credentials: credentials.NewStaticCredentials(AK, SK, ""),
			Endpoint:    Endpoint,
			DisableSSL:  true,
			Logger:      new(service.StdOutLogger),
			Debug:       true,
		},
	)

	seqNo := kk.PushLog(makeLog())
	a.Equal(seqNo, uint64(1))
	time.Sleep(time.Duration(3) * time.Second)
	kk.Stop(true)
}
