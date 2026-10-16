package handlers

// F19 狂暴模式（评论区实时更新）：
//   - LiveStream：GET /api/live/stream?channel=comment:novel:<id> —— SSE 长连接，
//     管理员关闭时 403；全局连接超限 503；批量事件由 realtime Hub 内部 flush 保证，
//     本 handler 只负责拆帧写出 + 15s 心跳保活。
//   - GetRageConfigAdmin / UpdateRageConfigAdmin：管理后台开关与带宽参数。

import (
	"bytes"
	"encoding/json"
	"net/http"
	"regexp"
	"time"

	"nvs-server/models"
	"nvs-server/realtime"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// liveChannelRe 频道 key 白名单：仅放行 comment:novel:<数字> / comment:blog:<数字>，
// 从源头杜绝任意字符串频道（防扫描器枚举/垃圾频道占用内存）
var liveChannelRe = regexp.MustCompile(`^comment:(novel|blog):\d+$`)

// LiveStream GET /api/live/stream?channel=xxx —— SSE 实时评论推送
func LiveStream(c *gin.Context) {
	channel := c.Query("channel")
	if !liveChannelRe.MatchString(channel) {
		utils.BadRequest(c, "频道参数非法")
		return
	}

	// 管理员开关：默认关闭，关闭时拒绝连接。
	// 注意：EventSource 无法读取 HTTP 状态码，故不走 403 JSON，而是以 SSE 协议返回
	// 一个 `disabled` 事件后直接关闭——前端据此永久停止重连（避免对未开启的站点空转重试）。
	cfg := models.GetRageConfig()
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
		utils.Error(c, http.StatusServiceUnavailable, utils.CodeInternalError, "实时连接数已达上限，请稍后重试")
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

// LiveConfig GET /api/live/config —— 公开探测：狂暴模式是否开启。
// 前端评论区挂载时先查一次，关闭则根本不建 SSE 连接（避免 403 后退避空转）；
// 仅暴露一个布尔开关，不泄露任何带宽参数。
func LiveConfig(c *gin.Context) {
	utils.Success(c, gin.H{"enabled": models.GetRageConfig().Enabled})
}

// GetRageConfigAdmin GET /api/admin/rage-config —— 读取狂暴模式配置
func GetRageConfigAdmin(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	utils.Success(c, models.GetRageConfig())
}

// UpdateRageConfigAdmin PUT /api/admin/rage-config —— 保存狂暴模式配置（立即生效）
func UpdateRageConfigAdmin(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Enabled          bool `json:"enabled"`
		BatchIntervalSec int  `json:"batch_interval_sec"`
		BatchSize        int  `json:"batch_size"`
		MaxConns         int  `json:"max_conns"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if req.BatchIntervalSec < 1 || req.BatchIntervalSec > 60 {
		utils.BadRequest(c, "批量周期需在 1 ~ 60 秒之间")
		return
	}
	if req.BatchSize < 1 || req.BatchSize > 100 {
		utils.BadRequest(c, "批量条数需在 1 ~ 100 之间")
		return
	}
	if req.MaxConns < 100 || req.MaxConns > 10000 {
		utils.BadRequest(c, "连接数上限需在 100 ~ 10000 之间")
		return
	}

	cfg := models.RageConfig{
		Enabled:          req.Enabled,
		BatchIntervalSec: req.BatchIntervalSec,
		BatchSize:        req.BatchSize,
		MaxConns:         req.MaxConns,
	}
	if err := models.UpdateRageConfig(cfg); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	// 即时生效：推送层开关 + Hub 限额
	realtime.DefaultHub.SetEnabled(cfg.Enabled)
	realtime.DefaultHub.SetLimits(cfg.MaxConns,
		time.Duration(cfg.BatchIntervalSec)*time.Second, cfg.BatchSize)
	utils.Success(c, gin.H{"message": "已保存（立即生效）", "config": models.GetRageConfig()})
}
