package realtime

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestStartGRPCAndStream 回归测试：
//
//  1. StartGRPC 不得 panic —— RegisterService 的 HandlerType 必须是「接口类型」指针，
//     传具体类型（(*LiveService)(nil)）会触发 reflect Implements panic
//     （"reflect: non-interface type passed to Type.Implements"）。线上 50051 端口被
//     残留监听器占住、后续重启一直 bind 失败，根因就是首次注册 panic 时监听器未释放。
//  2. Stream 应能订阅 Hub 频道并收到事件。
func TestStartGRPCAndStream(t *testing.T) {
	DefaultHub.SetEnabled(true)
	defer DefaultHub.SetEnabled(false)
	// 批量参数调小，令单条事件立即 flush（默认 3s 周期会让接收超时）
	DefaultHub.SetLimits(100, 10*time.Millisecond, 1)
	defer DefaultHub.SetLimits(DefaultMaxConns, DefaultBatchInterval, DefaultBatchSize)

	const (
		host = "127.0.0.1"
		addr = "127.0.0.1:50099"
	)
	g, err := StartGRPC(host, 50099)
	if err != nil {
		t.Skipf("gRPC 测试端口不可用，跳过: %v", err)
	}
	defer g.Stop()

	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(jsonCodec{})),
	)
	if err != nil {
		t.Fatalf("dial 失败: %v", err)
	}
	defer conn.Close()

	desc := &grpc.StreamDesc{StreamName: "Stream", ServerStreams: true}
	cs, err := conn.NewStream(context.Background(), desc, "/nvs.realtime.Live/Stream")
	if err != nil {
		t.Fatalf("新建流失败: %v", err)
	}
	if err := cs.SendMsg(&StreamRequest{Channel: "comment:novel:9"}); err != nil {
		t.Fatalf("发送订阅请求失败: %v", err)
	}

	// 等订阅在服务端建立后再广播
	time.Sleep(200 * time.Millisecond)
	DefaultHub.Publish("comment:novel:9", Event{Name: "comment", Data: map[string]any{"id": 42}})

	ev := &Event{}
	recvd := make(chan error, 1)
	go func() { recvd <- cs.RecvMsg(ev) }()
	select {
	case err := <-recvd:
		if err != nil {
			t.Fatalf("接收事件失败: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("接收事件超时")
	}
	if ev.Name != "comment" {
		t.Fatalf("事件名不符: %q", ev.Name)
	}
}
