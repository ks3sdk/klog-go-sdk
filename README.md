# !!!本项目已经迁移至 https://gitee.com/klogsdk/klog-go-sdk，并且已经重构。Github中的代码将不再更新。

# 金山云日志服务(KLog) SDK for golang
+ [金山云日志服务产品简介](https://www.ksyun.com/nv/product/KLog.html)
+ [金山云日志服务产品文档](https://docs.ksyun.com/products/123)

## 引用方法
```
go get github.com/ks3sdk/klog-go-sdk
```

## 使用批量同步客户端

```go
    import (
    	"time"
    	
        sdkCredentials "github.com/ks3sdk/klog-go-sdk/credentials"
        sdk "github.com/ks3sdk/klog-go-sdk/klog"
        sdkPb "github.com/ks3sdk/klog-go-sdk/protobuf"
        sdkService "github.com/ks3sdk/klog-go-sdk/service"
    )
    
    // 鉴权配置
    credentials := sdkCredentials.NewStaticCredentials("<AccessKeyID>", "<AccessKeySecret>", "")
    
    // klog配置
    klogConfig := &sdkService.Config{
        Credentials: credentials.NewStaticCredentials(AK, SK, ""),
        Endpoint:    "klog-cn-beijing-internal.ksyun.com",
        DisableSSL:  true,
       
        // sdk自身日志，可使用符合sdkService.Logger接口的日志对象。
        // 也可使用klog自带的简单日志模块sdkService.StdOutLogger。
        Logger:      nil,
        // 是否打印请求头、响应头等
        Debug:       false,
    }
    
    // klog客户端
    client := sdk.New(klogConfig)
    
    // 组装日志数据
    log1 := &sdkPb.Log{
        Time:     time.Now().UnixNano() / 1000000, // 举例日志产生时间的毫秒数
        Contents: []*sdkPb.Log_Content{
            {Key: "key1", Value: "value1"},
            {Key: "key2", Value: "value2"},
        },
    }
    log2 := &sdkPb.Log{
        Time:     time.Now().UnixNano() / 1000000, // 举例日志产生时间的毫秒数
        Contents: []*sdkPb.Log_Content{
            {Key: "key1", Value: "value3"},
            {Key: "key2", Value: "value4"},
        },
    }
    logGroup := &sdkPb.LogGroup{
        Logs:     make([]*sdkPb.Log, 0),
        Filename: filename,
        Source:   hostname,
    }
    logGroup.Logs = append(logGroup.Logs, log1)
    logGroup.Logs = append(logGroup.Logs, log2)
    
    // 同步发送
    err := client.PutLogs(logGroup, "<ProjectName>", "<LogPoolName>")
```

## 使用异步客户端(推荐)
```go
    // 异步配置
    asyncClientOptions := &sdk.AsyncClientOptions{
        ProjectName:         "<ProjectName>",
        LogPoolName:         "<LogPoolName>",

        // 用户接收每条日志发送状态的回调函数（选填）
        // func(log *pb.Log, seqNo uint64, err error)
        // log: 日志数据
        // seqNo: 日志顺序号
        // err: nil表示发送成功。非nil表示错误，并且该条日志被丢弃。
        Callback:            nil,

        // 日志池不存在时，是否丢弃日志（选填）
        DropIfPoolNotExists: false,

        // 等待发送日志时的缓冲队列长度（选填）
        QueueSize:           2048,   
    }
    
    // 异步客户端
    asyncClient := sdk.NewAsyncClient(asyncClientOptions, klogConfig)
    
    // 推入发送队列，返回用于跟踪状态的顺序号。
    seqNo1 := asyncClient.PushLog(log1)
    seqNo2 := asyncClient.PushLog(log2)
    
    // 用于进程退出
    asyncClient.Stop(true)
```
## 多日志池异步客户端
用于向同一个用户的多个日志池发送数据
```go
    // 
     asyncMultiPoolClientOptions := &sdk.AsyncMultiPoolClientOptions{
        // 同AsyncClientOptions
        Callback:            nil,

        // 同AsyncClientOptions
        DropIfPoolNotExists: false,

        // 同AsyncClientOptions
        QueueSize:           2048,   
    }
    
    // 多日志池异步客户端
    client := sdk.NewAsyncMultiPoolClient(asyncMultiPoolClientOptions, klogConfig)
    
    // 推入发送队列，返回用于跟踪状态的顺序号。
    seqNo1 := client.PushLog("<projectName1>", "<logPoolName1>", log1)
    seqNo2 := client.PushLog("<projectName2>", "<logPoolName2>", log2)
    
    // 用于进程退出
    client.Stop(true)
```

