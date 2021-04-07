package klog

import (
	"context"
	"fmt"
	"github.com/ks3sdk/klog-go-sdk/internal/apierr"
	pb "github.com/ks3sdk/klog-go-sdk/protobuf"
	"github.com/ks3sdk/klog-go-sdk/service"
	"google.golang.org/protobuf/proto"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	MaxKeyCount        = 900     // amount
	MaxKeySize         = 1 << 20 // byte
	MaxValueSize       = 1 << 20 // byte
	MaxBulkSize        = 4 << 10 // amount
	MaxLogSize         = 3000000 // byte
	MaxLogGroupSize    = 3000000 // byte
	LogGroupSizeToSend = 2000000 // byte
)

type AsyncClient struct {
	ProjectName string
	LogPoolName string
	KLog        *Klog

	dropIfLogPoolNotExists bool
	callback               func(*pb.Log, uint64, error)
	ch                     chan *event
	lastSendAt             time.Time
	idBuf                  []uint64
	buf                    []*pb.Log
	bufSize                int
	wg                     *sync.WaitGroup
	ctx                    context.Context
	cancel                 context.CancelFunc
}

type AsyncClientOptions struct {
	ProjectName string
	LogPoolName string

	// Callback: 每条日志在发送成功或丢弃时调用。
	// log: 日志数据
	// seqNo: 日志顺序号
	// err: nil表示发送成功。非nil表示错误，并且该条日志被丢弃。
	Callback            func(log *pb.Log, seqNo uint64, err error)
	DropIfPoolNotExists bool
	QueueSize           int
}

type event struct {
	seqNo uint64
	log   *pb.Log
}

// 新建异步发送客户端
func NewAsyncClient(options *AsyncClientOptions, kLogConfig *service.Config) *AsyncClient {
	ctx, cancel := context.WithCancel(context.Background())

	queueSize := 2048
	if options.QueueSize != 0 {
		queueSize = options.QueueSize
	}

	c := &AsyncClient{
		ProjectName:            options.ProjectName,
		LogPoolName:            options.LogPoolName,
		KLog:                   New(kLogConfig),
		callback:               options.Callback,
		dropIfLogPoolNotExists: options.DropIfPoolNotExists,
		ch:                     make(chan *event, queueSize),
		buf:                    make([]*pb.Log, 0),
		lastSendAt:             time.Now(),
		wg:                     new(sync.WaitGroup),
		ctx:                    ctx,
		cancel:                 cancel,
	}
	go c.run()
	return c
}

// 异步发送一条log，返回这条log的seq no.。
// seq no.用来在callback中跟踪发送情况。
// seq no.只在进程内有效。
func (o *AsyncClient) PushLog(log *pb.Log) uint64 {
	ev := &event{
		seqNo: service.GetSeqNo(),
		log:   log,
	}
	o.ch <- ev
	return ev.seqNo
}

// 停止发送。
// 调用Stop()之后，AsyncClient等待当前发送请求完成，然后停止。
func (o *AsyncClient) Stop(wait bool) {
	o.cancel()
	if wait {
		o.wg.Wait()
	}
}

func (o *AsyncClient) run() {
	o.wg.Add(1)
	defer o.wg.Done()
	ticker := time.NewTicker(time.Duration(200) * time.Millisecond)
	for {
		select {
		case <-o.ctx.Done():
			return
		case ev := <-o.ch:
			size := proto.Size(ev.log)
			if size > MaxLogSize {
				// 这条log过大，需要抛弃
				o.doCallback(ev.log, ev.seqNo, apierr.New("MaxLogSizeExceeded", fmt.Sprintf("the size of this log is %d and the MaxLogSize is %d", size, MaxLogSize), nil))
				continue
			} else if size+o.bufSize > MaxLogGroupSize {
				// 这条log与buf中的log size之和，超过限制，需要先把buf中的发送出去
				o.send()
			}

			// 处理这条log
			o.buf = append(o.buf, ev.log)
			o.idBuf = append(o.idBuf, ev.seqNo)
			o.bufSize += size
			if o.bufSize >= LogGroupSizeToSend || len(o.buf) >= MaxBulkSize {
				o.send()
			}
		case <-ticker.C:
			if time.Now().Sub(o.lastSendAt) > time.Duration(2)*time.Second && len(o.buf) > 0 {
				o.send()
			}
		}
	}
}

func (o *AsyncClient) send() {
	var count int
	var err error
	defer func() {
		for i := range o.buf {
			o.doCallback(o.buf[i], o.idBuf[i], err)
		}
		o.bufSize = 0
		o.buf = o.buf[:0]
		o.idBuf = o.idBuf[:0]
		o.lastSendAt = time.Now()
	}()

	for {
		lg := &pb.LogGroup{Logs: o.buf}

		// 发送请求
		err = o.KLog.PutLogs(lg, o.ProjectName, o.LogPoolName)
		if err == nil {
			// 成功
			return
		}

		if IsError(err, MaxKeyCountExceeded) || IsError(err, MaxKeySizeExceeded) || IsError(err, MaxValueSizeExceeded) || IsError(err, PostBodyInvalid) || err.Error() == "string field contains invalid UTF-8" {
			// 存在有问题的日志，而且不可能发出去，丢弃后重试
			o.removeInvalidLogs()

			if len(o.buf) == 0 {
				return
			}
			if err.Error() == "string field contains invalid UTF-8" {
				continue
			}

		} else if IsError(err, UserNotExist) || IsError(err, ProjectOrLogPoolNotExist) {
			// 用户未开通kLog或日志池不存在
			if o.dropIfLogPoolNotExists {
				// 根据用户配置，丢弃但标记成功
				return
			}
		}

		o.KLog.Config.Logger.Errorf("klog.AsyncClient.Send: sleep then retry, project=%s, pool=%s, err=%s", o.ProjectName, o.LogPoolName, err.Error())

		// 其他问题都需要重试
		count++
		timer := service.MakeRandomTimer(count)
		select {
		case <-timer.C:
			continue
		case <-o.ctx.Done():
			// 收到停止信号
			o.KLog.Config.Logger.Infof("INFO AsyncClient.Send: cancel received, stop retry, project=%s, pool=%s", o.ProjectName, o.LogPoolName)
			return
		}
	}
}

func (o *AsyncClient) removeInvalidLogs() {
	var err error
	newBuf := make([]*pb.Log, 0)
	newIdBuf := make([]uint64, 0)
	for i, log := range o.buf {
		if err = CheckLog(log); err != nil {
			o.doCallback(log, o.idBuf[i], err)
		} else {
			newBuf = append(newBuf, log)
			newIdBuf = append(newIdBuf, o.idBuf[i])
		}
	}
	o.buf = newBuf
	o.idBuf = newIdBuf
}

func (o *AsyncClient) doCallback(log *pb.Log, seqNo uint64, err error) {
	if o.callback != nil {
		o.callback(log, seqNo, err)
	}
}

func CheckLog(log *pb.Log) error {
	contents := log.GetContents()
	if len(contents) > MaxKeyCount {
		return apierr.New(MaxKeyCountExceeded, fmt.Sprintf("the amount[%d] of keys in one log should not be greater than %d", len(contents), MaxKeyCount), nil)
	}
	for j := 0; j < len(contents); j++ {
		if !utf8.ValidString(contents[j].Key) {
			return apierr.New(InvalidUtf8InKey, fmt.Sprintf("invalid UTF-8 in key"), nil)
		}

		if !utf8.ValidString(contents[j].Value) {
			return apierr.New(InvalidUtf8InValue, fmt.Sprintf("invalid UTF-8 in value"), nil)
		}

		if len([]byte(contents[j].Key)) > MaxKeySize {
			return apierr.New(MaxKeySizeExceeded, fmt.Sprintf("the size[%d] of a key should not be greater than %d", len([]byte(contents[j].Key)), MaxKeySize), nil)
		}

		if len([]byte(contents[j].Value)) > MaxValueSize {
			return apierr.New(MaxValueSizeExceeded, fmt.Sprintf("the size[%d] of a value should not be greater than %d", len([]byte(contents[j].Value)), MaxValueSize), nil)
		}
	}
	return nil
}
