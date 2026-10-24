package grouphandler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"nvs-server/middleware"
	"nvs-server/realtime"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 群组 G4：实时消息（SSE） ====================
// 技术选型：**SSE**（Server-Sent Events），不引入 WebSocket。
//   - 群内消息是「服务端 → 客户端」单向广播，SSE 语义完全够用；
//   - 复用站内既有 realtime.Hub（批量 flush + 慢客户端踢除 + 心跳），零新增依赖；
//   - SSE 走普通 HTTP，反代（nginx）与 cookie 鉴权都无需额外改动；
//   - 前端用 fetch + ReadableStream 读取（可携带 Cookie），不需要把 token 放进 URL。
//
// 隔离设计：群组用**独立 Hub 实例**，不复用狂暴模式的 DefaultHub——
// 后者受「狂暴模式」开关与评论带宽参数约束，群消息不该被那个开关连带关掉。

var groupHub = realtime.NewHub()

func init() {
	// 群消息偏即时：批量窗口压到 800ms、批大小 8（比评论弹幕更激进）
	groupHub.SetLimits(2000, 800*time.Millisecond, 8)
	groupHub.SetEnabled(true)
}

// groupChannelRe 频道白名单（内部拼装后校验，杜绝外部任意频道）
var groupChannelRe = regexp.MustCompile(`^group:\d+$`)

func groupChannel(groupID uint) string {
	return "group:" + strconv.FormatUint(uint64(groupID), 10)
}

// 事件名（前端按名分派）
const (
	GroupEventPostCreated  = "post.created"
	GroupEventPostReviewed = "post.reviewed"
	GroupEventReplyCreated = "reply.created"
	GroupEventAnnouncement = "announcement"
	GroupEventMemberJoined = "member.joined"
	GroupEventMemberLeft   = "member.left"
)

// publishGroupEvent 向群频道广播一条事件（非阻塞；无订阅者时为空操作）
func publishGroupEvent(groupID uint, name string, data any) {
	key := groupChannel(groupID)
	if !groupChannelRe.MatchString(key) {
		return
	}
	groupHub.Publish(key, realtime.Event{Name: name, Data: data})
}

// Stream GET /api/groups/:id/stream —— 群内实时消息（SSE 长连接）
// 权限：公开群任何登录用户可订阅；半公开/私密仅正常成员。
func Stream(c *gin.Context) {
	gid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if gid == 0 {
		utils.BadRequest(c, "群组 ID 无效")
		return
	}
	if middleware.UserID(c) == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	ctx, ok := loadGroupCtx(c, uint(gid))
	if !ok {
		return
	}
	if ctx.Status != "active" && !ctx.CanRead && !ctx.IsAdmin {
		utils.Forbidden(c, "仅群成员可接收实时消息")
		return
	}

	key := groupChannel(uint(gid))
	client, err := groupHub.Subscribe(key)
	if err != nil {
		utils.Error(c, http.StatusServiceUnavailable, utils.CodeInternalError, "实时连接数已达上限，请稍后重试")
		return
	}
	defer groupHub.Unsubscribe(key, client)

	// SSE 响应头；X-Accel-Buffering: no 让 nginx 不缓冲、立即下发
	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	// 建连即回一个 hello，前端据此确认订阅成功（也便于排查反代缓冲）
	_, _ = c.Writer.WriteString("event: hello\ndata: {\"group_id\":" + strconv.FormatUint(gid, 10) + "}\n\n")
	c.Writer.Flush()

	reqCtx := c.Request.Context()
	heartbeat := time.NewTicker(realtime.HeartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-reqCtx.Done():
			return // 客户端断开
		case <-client.Done():
			return // 慢客户端被服务端踢除
		case ev := <-client.Events():
			writeGroupSSE(c, ev)
		case <-heartbeat.C:
			_, _ = c.Writer.WriteString(": ping\n\n")
			c.Writer.Flush()
		}
	}
}

// writeGroupSSE 写出事件；批量信封拆成多条帧、只 Flush 一次
func writeGroupSSE(c *gin.Context, ev realtime.Event) {
	var buf bytes.Buffer
	if ev.Name == realtime.EventNameBatch {
		if batch, ok := ev.Data.(realtime.EventBatch); ok {
			for _, item := range batch.Events {
				writeGroupFrame(&buf, item.Name, item.Data)
			}
		}
	} else {
		writeGroupFrame(&buf, ev.Name, ev.Data)
	}
	if buf.Len() == 0 {
		return
	}
	_, _ = c.Writer.Write(buf.Bytes())
	c.Writer.Flush()
}

func writeGroupFrame(buf *bytes.Buffer, name string, data any) {
	payload, err := json.Marshal(data)
	if err != nil {
		return
	}
	buf.WriteString("event: ")
	buf.WriteString(name)
	buf.WriteString("\ndata: ")
	buf.Write(payload)
	buf.WriteString("\n\n")
}
