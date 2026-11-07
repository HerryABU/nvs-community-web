package live

// F19 狂暴模式（评论区实时更新）：
//   - LiveStream：GET /api/live/stream?channel=comment:novel:<id> —— SSE 长连接，
//     管理员关闭时 403；全局连接超限 503；批量事件由 realtime Hub 内部 flush 保证，
//     本 handler 只负责拆帧写出 + 15s 心跳保活。
//   - GetRageConfigAdmin / UpdateRageConfigAdmin：管理后台开关与带宽参数。

import (
	"bytes"
	"encoding/json"
	"net/http"
	"nvs-server/handlers/admincommon"
	"nvs-server/realtime"
	"nvs-server/utils/infra"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	siteops "nvs-server/models/siteops"
)

// liveChannelRe 频道 key 白名单：仅放行 comment:novel:<数字> / comment:blog:<数字>，
// 从源头杜绝任意字符串频道（防扫描器枚举/垃圾频道占用内存）
var liveChannelRe = regexp.MustCompile(`^comment:(novel|blog):\d+$`)

// LiveStream GET /api/live/stream?channel=xxx —— SSE 实时评论推送
func LiveStream(c *gin.Context) {
	channel := c.Query("channel")
	if !liveChannelRe.MatchString(channel) {
		infra.BadRequest(c, "频道参数非法")
		return
	}

	// 管理员开关：默认关闭，关闭时拒绝连接。
	// 注意：EventSource 无法读取 HTTP 状态码，故不走 403 JSON，而是以 SSE 协议返回
	// 一个 `disabled` 事件后直接关闭——前端据此永久停止重连（避免对未开启的站点空转重试）。
	cfg := siteops.GetRageConfig()
	if !cfg.Enabled {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
		c.Status(http.StatusOK)
		_, _ = c.Writer.WriteString("event: disabled\ndata: {}\n\n")
		c.Writer.Flush()
		return
	}

	// 把管理后台的带宽配置同步进 Hub（连接建立即生效）
	realtime.DefaultHub.SetLimits(cfg.MaxConns,
		time.Duration(cfg.BatchIntervalSec)*time.Second, cfg.BatchSize)
	realtime.DefaultHub.SetEnabled(true)

	client, err := realtime.DefaultHub.Subscribe(channel)
	if err != nil {
		// 全局连接数上限：拒绝新连接，前端指数退避后自动重试
		infra.Error(c, http.StatusServiceUnavailable, infra.CodeInternalError, "实时连接数已达上限，请稍后重试")
		return
	}
	defer realtime.DefaultHub.Unsubscribe(channel, client)

	// SSE 响应头。X-Accel-Buffering: no —— 通知 nginx 等反代不要缓冲，立即下发
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	ctx := c.Request.Context()
	heartbeat := time.NewTicker(realtime.HeartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			// 客户端断开（关闭页面/切网络）
			return
		case <-client.Done():
			// 服务端主动断开（慢客户端被踢/连接被清理），前端按退避策略重连
			return
		case ev := <-client.Events():
			writeSSEEvent(c, ev)
		case <-heartbeat.C:
			// 心跳：一个注释行几十字节，保活 + 探测半死连接
			_, _ = c.Writer.WriteString(": ping\n\n")
			c.Writer.Flush()
		}
	}
}

// writeSSEEvent 写出一个事件。批量信封拆成多条 SSE 帧但只做一次 Write+Flush：
// 省系统调用与 TCP 包数，前端仍按「一条评论一个 message」处理
func writeSSEEvent(c *gin.Context, ev realtime.Event) {
	var buf bytes.Buffer
	if ev.Name == realtime.EventNameBatch {
		if batch, ok := ev.Data.(realtime.EventBatch); ok {
			for _, item := range batch.Events {
				writeSSEFrame(&buf, item.Name, item.Data)
			}
		}
	} else {
		writeSSEFrame(&buf, ev.Name, ev.Data)
	}
	if buf.Len() == 0 {
		return
	}
	_, _ = c.Writer.Write(buf.Bytes())
	c.Writer.Flush()
}

// writeSSEFrame 写单条 SSE 帧：event: <name>\ndata: <json>\n\n
func writeSSEFrame(buf *bytes.Buffer, name string, data any) {
	payload, err := json.Marshal(data)
	if err != nil {
		return // 丢弃无法序列化的坏事件，避免污染整个流
	}
	buf.WriteString("event: ")
	buf.WriteString(name)
	buf.WriteString("\ndata: ")
	buf.Write(payload)
	buf.WriteString("\n\n")
}

// ─────────────────────────────────────────────────────────────
// WebSocket 传输（狂暴模式可选层之一）：GET /api/live/ws?channel=comment:novel:<id>
// 与 SSE 同源复用 Hub.Client，仅传输通道由 HTTP 长轮询改为 WebSocket 双向帧。
// 浏览器友好（相比 gRPC），又比 SSE 支持客户端→服务端信令（预留扩展）。
// ─────────────────────────────────────────────────────────────

// wsUpgrader WebSocket 升级器。CheckOrigin 放宽：跨域由 CSRF/登录态保证，
// 且独立页 /federation 等嵌入场景可能跨子域，过度收紧会误杀合法连接。
var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// LiveWS GET /api/live/ws —— WebSocket 实时评论订阅。
func LiveWS(c *gin.Context) {
	channel := c.Query("channel")
	if !liveChannelRe.MatchString(channel) {
		infra.BadRequest(c, "频道参数非法")
		return
	}
	cfg := siteops.GetRageConfig()
	if !cfg.Enabled || !cfg.WsEnabled {
		infra.Error(c, http.StatusServiceUnavailable, infra.CodeInternalError, "WebSocket 实时通道未开启")
		return
	}

	// 把管理后台的带宽配置同步进 Hub（连接建立即生效）
	realtime.DefaultHub.SetLimits(cfg.MaxConns,
		time.Duration(cfg.BatchIntervalSec)*time.Second, cfg.BatchSize)
	realtime.DefaultHub.SetEnabled(true)

	client, err := realtime.DefaultHub.Subscribe(channel)
	if err != nil {
		infra.Error(c, http.StatusServiceUnavailable, infra.CodeInternalError, "实时连接数已达上限，请稍后重试")
		return
	}
	defer realtime.DefaultHub.Unsubscribe(channel, client)

	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return // 升级失败（如反向代理不支持），直接返回，连接已关闭
	}
	defer conn.Close()

	// 读取协程：仅用于探测客户端断开（不处理业务消息）
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			if _, _, rerr := conn.ReadMessage(); rerr != nil {
				return // 客户端断开/出错
			}
		}
	}()

	hb := time.NewTicker(realtime.HeartbeatInterval)
	defer hb.Stop()
	for {
		select {
		case <-readDone:
			return
		case <-client.Done():
			return
		case ev := <-client.Events():
			writeWSEvent(conn, ev)
		case <-hb.C:
			// WebSocket 应用层心跳：Ping 帧（浏览器自动回 Pong）
			_ = conn.WriteMessage(websocket.PingMessage, nil)
		}
	}
}

// writeWSEvent 把一个 Hub 事件写成 WebSocket 文本消息（JSON：{"name":..,"data":..}）。
// 批量信封拆成多条消息，与 SSE 帧语义一致（前端仍按「一条评论一个 message」处理）。
func writeWSEvent(conn *websocket.Conn, ev realtime.Event) {
	var msgs []realtime.Event
	if ev.Name == realtime.EventNameBatch {
		if batch, ok := ev.Data.(realtime.EventBatch); ok {
			msgs = batch.Events
		}
	} else {
		msgs = []realtime.Event{ev}
	}
	for _, m := range msgs {
		payload, err := json.Marshal(gin.H{"name": m.Name, "data": m.Data})
		if err != nil {
			continue
		}
		if werr := conn.WriteMessage(websocket.TextMessage, payload); werr != nil {
			return // 写失败即视为断开，主循环下次 select 会退出
		}
	}
}

// LiveConfig GET /api/live/config —— 公开探测：狂暴模式与各传输层可用状态。
// 前端评论区挂载时先查一次，按可用性择优（ws > sse），关闭则根本不建连接（避免空转重试）；
// 仅暴露布尔/端口等必要信息，不泄露任何带宽参数。
func LiveConfig(c *gin.Context) {
	cfg := siteops.GetRageConfig()
	infra.Success(c, gin.H{
		"enabled":      cfg.Enabled,
		"ws_enabled":   cfg.WsEnabled,
		"mqtt_enabled": cfg.MqttEnabled,
		"mqtt_port":    cfg.MqttPort,
		"grpc_enabled": cfg.GrpcEnabled,
		"grpc_port":    cfg.GrpcPort,
	})
}

// GetRageConfigAdmin GET /api/admin/rage-config —— 读取狂暴模式配置
func GetRageConfigAdmin(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	infra.Success(c, siteops.GetRageConfig())
}

// UpdateRageConfigAdmin PUT /api/admin/rage-config —— 保存狂暴模式配置（立即生效）
func UpdateRageConfigAdmin(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	var req struct {
		Enabled          bool   `json:"enabled"`
		BatchIntervalSec int    `json:"batch_interval_sec"`
		BatchSize        int    `json:"batch_size"`
		MaxConns         int    `json:"max_conns"`
		WsEnabled        bool   `json:"ws_enabled"`
		MqttEnabled      bool   `json:"mqtt_enabled"`
		MqttBind         string `json:"mqtt_bind"`
		MqttPort         int    `json:"mqtt_port"`
		GrpcEnabled      bool   `json:"grpc_enabled"`
		GrpcBind         string `json:"grpc_bind"`
		GrpcPort         int    `json:"grpc_port"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if req.BatchIntervalSec < 1 || req.BatchIntervalSec > 60 {
		infra.BadRequest(c, "批量周期需在 1 ~ 60 秒之间")
		return
	}
	if req.BatchSize < 1 || req.BatchSize > 100 {
		infra.BadRequest(c, "批量条数需在 1 ~ 100 之间")
		return
	}
	if req.MaxConns < 100 || req.MaxConns > 10000 {
		infra.BadRequest(c, "连接数上限需在 100 ~ 10000 之间")
		return
	}
	if req.MqttEnabled {
		if req.MqttPort < 1 || req.MqttPort > 65535 {
			infra.BadRequest(c, "MQTT 端口需在 1 ~ 65535 之间")
			return
		}
		if req.MqttBind == "" {
			req.MqttBind = "0.0.0.0"
		}
	}
	if req.GrpcEnabled {
		if req.GrpcPort < 1 || req.GrpcPort > 65535 {
			infra.BadRequest(c, "gRPC 端口需在 1 ~ 65535 之间")
			return
		}
		if req.GrpcBind == "" {
			req.GrpcBind = "0.0.0.0"
		}
	}

	cfg := siteops.RageConfig{
		Enabled:          req.Enabled,
		BatchIntervalSec: req.BatchIntervalSec,
		BatchSize:        req.BatchSize,
		MaxConns:         req.MaxConns,
		WsEnabled:        req.WsEnabled,
		MqttEnabled:      req.MqttEnabled,
		MqttBind:         req.MqttBind,
		MqttPort:         req.MqttPort,
		GrpcEnabled:      req.GrpcEnabled,
		GrpcBind:         req.GrpcBind,
		GrpcPort:         req.GrpcPort,
	}
	if err := siteops.UpdateRageConfig(cfg); err != nil {
		infra.InternalError(c, "保存失败")
		return
	}
	// 即时生效：推送层开关 + Hub 限额（MQTT/gRPC 由传输管理器按配置起停，见 realtime.Manager）
	realtime.DefaultHub.SetEnabled(cfg.Enabled)
	realtime.DefaultHub.SetLimits(cfg.MaxConns,
		time.Duration(cfg.BatchIntervalSec)*time.Second, cfg.BatchSize)
	// 触发传输管理器：按需拉起/关停 WS/MQTT/gRPC（保存即生效，无需重启）
	realtime.GetManager().ApplyConfig(cfg)
	infra.Success(c, gin.H{"message": "已保存（立即生效）", "config": siteops.GetRageConfig()})
}
