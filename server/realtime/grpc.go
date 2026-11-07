package realtime

// grpc.go —— 狂暴模式可选传输层之二：内嵌 gRPC 服务端（HTTP/2 流式）。
//
// 设计要点：
//   - 复用 Hub.Client 订阅模型：每条 gRPC 流式订阅 = 一个 Hub 订阅者，事件由 Hub 统一
//     批量 flush 后逐条下发，与 SSE/WS 同源，带宽闸（MaxConns/批量）天然复用。
//   - 免 protoc：用自定义 JSON Codec（encoding.RegisterCodec），线格式为 JSON，
//     因此无需编译 .proto；调用方须使用同名 JSON Codec（见 NewJSONCodec / 文档）。
//     适合原生/移动/后端客户端（浏览器需 grpc-web，不在本特性内）。
//   - 仅服务端流式（StreamRequest → 多 Event）。入站写路径仍走平台原生接口，不暴露给 gRPC。
//   - 独立端口（默认 50051），由 Manager 按 grpc_enabled 拉起/关停。

import (
	"encoding/json"
	"log"
	"net"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/status"
)

// jsonCodec gRPC 自定义 JSON 编解码（免 protoc）。
type jsonCodec struct{}

func (jsonCodec) Marshal(v any) ([]byte, error)      { return json.Marshal(v) }
func (jsonCodec) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }
func (jsonCodec) Name() string                       { return "json" }

func init() {
	// 注册后，content-type 为 application/grpc+json 的请求可由本 Codec 解码；
	// 服务端通过 ForceCodec 强制使用，调用方亦须使用同名 Codec。
	encoding.RegisterCodec(jsonCodec{})
}

// StreamRequest gRPC 流式订阅请求（JSON 线格式）。
type StreamRequest struct {
	Channel string `json:"channel"`
}

// LiveService 狂暴模式实时流服务（手动注册，无 protobuf 生成代码）。
type LiveService struct{}

// liveServiceDesc 手工构造的 gRPC 服务描述（服务端流式方法 Stream）。
var liveServiceDesc = grpc.ServiceDesc{
	ServiceName: "nvs.realtime.Live",
	HandlerType: (*LiveService)(nil),
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Stream",
			Handler:       streamHandler,
			ServerStreams: true,
		},
	},
	Metadata: "nvs-realtime",
}

// streamHandler 桥接 grpc.ServerStream 与 LiveService.Stream。
func streamHandler(srv any, stream grpc.ServerStream) error {
	req := new(StreamRequest)
	if err := stream.RecvMsg(req); err != nil {
		return err
	}
	return srv.(*LiveService).Stream(req, stream)
}

// Stream 服务端流式：订阅频道，逐条下发事件（comment 原样 / batch 拆帧），15s 心跳保活。
func (s *LiveService) Stream(req *StreamRequest, stream grpc.ServerStream) error {
	if !DefaultHub.Enabled() {
		return status.Error(codes.Unavailable, "狂暴模式未开启")
	}
	channel := req.Channel
	if !liveChannelRe.MatchString(channel) {
		return status.Error(codes.InvalidArgument, "频道参数非法")
	}
	client, err := DefaultHub.Subscribe(channel)
	if err != nil {
		return status.Error(codes.ResourceExhausted, "实时连接数已达上限")
	}
	defer DefaultHub.Unsubscribe(channel, client)

	hb := time.NewTicker(15 * time.Second)
	defer hb.Stop()
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-client.Done():
			return nil
		case ev := <-client.Events():
			if err := sendStreamEvent(stream, ev); err != nil {
				return err
			}
		case <-hb.C:
			// gRPC 心跳：发一个 ping 事件（客户端可忽略，仅用于保活/探测半死连接）
			_ = stream.SendMsg(&Event{Name: "ping", Data: nil})
		}
	}
}

// sendStreamEvent 把一个 Hub 事件写入 gRPC 流；批量信封拆成多条（与 SSE/WS 语义一致）。
func sendStreamEvent(stream grpc.ServerStream, ev Event) error {
	if ev.Name == EventNameBatch {
		if batch, ok := ev.Data.(EventBatch); ok {
			for _, item := range batch.Events {
				if err := stream.SendMsg(&item); err != nil {
					return err
				}
			}
			return nil
		}
	}
	return stream.SendMsg(&ev)
}

// GRPCServer 内嵌 gRPC 服务端包装。
type GRPCServer struct {
	Bind   string
	Port   int
	server *grpc.Server
	lis    net.Listener
}

// StartGRPC 拉起 gRPC 服务端（plaintext h2c，独立端口）。失败返回 error。
func StartGRPC(bind string, port int) (*GRPCServer, error) {
	g := &GRPCServer{Bind: bind, Port: port}
	lis, err := net.Listen("tcp", net.JoinHostPort(bind, strconv.Itoa(port)))
	if err != nil {
		return nil, err
	}
	srv := grpc.NewServer(grpc.ForceServerCodec(jsonCodec{}))
	srv.RegisterService(&liveServiceDesc, &LiveService{})
	g.server = srv
	g.lis = lis
	go func() {
		if err := srv.Serve(lis); err != nil {
			log.Printf("[grpc] 服务端已停止: %v", err)
		}
	}()
	log.Printf("[grpc] 服务端已监听 %s（content-type: application/grpc+json）", lis.Addr())
	return g, nil
}

// Stop 优雅关停 gRPC 服务端。
func (g *GRPCServer) Stop() {
	if g.server != nil {
		g.server.GracefulStop()
	}
	g.server = nil
}
