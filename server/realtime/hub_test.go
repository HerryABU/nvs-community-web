package realtime

import (
	"testing"
	"time"
)

// waitEvent 短超时等待一个事件（避免测试挂死）
func waitEvent(t *testing.T, cl *Client, timeout time.Duration) Event {
	t.Helper()
	select {
	case ev := <-cl.Events():
		return ev
	case <-time.After(timeout):
		t.Fatalf("等待事件超时（%v）", timeout)
		return Event{}
	}
}

// TestSubscribeUnsubscribe 订阅/退订：计数、频道回收、done 关闭
func TestSubscribeUnsubscribe(t *testing.T) {
	h := NewHub()
	defer h.Close()

	c1, err := h.Subscribe("comment:novel:1")
	if err != nil {
		t.Fatalf("订阅失败: %v", err)
	}
	c2, _ := h.Subscribe("comment:novel:1")
	c3, _ := h.Subscribe("comment:blog:2")

	st := h.Stats()
	if st["clients"].(int) != 3 || st["channels"].(int) != 2 {
		t.Fatalf("订阅统计异常: %v", st)
	}

	h.Unsubscribe("comment:novel:1", c1)
	h.Unsubscribe("comment:novel:1", c2)
	h.Unsubscribe("comment:blog:2", c3)

	if st := h.Stats(); st["channels"].(int) != 0 || st["clients"].(int) != 0 {
		t.Fatalf("全部退订后应回收空频道: %v", st)
	}
	select {
	case <-c1.Done():
	default:
		t.Fatal("退订后 done 未关闭")
	}
}

// TestPublishDelivery 发布后订阅者能收到（经批量信封，单条也包 batch 保持帧格式统一）
func TestPublishDelivery(t *testing.T) {
	h := NewHub()
	defer h.Close()
	h.SetLimits(DefaultMaxConns, 50*time.Millisecond, DefaultBatchSize)

	cl, _ := h.Subscribe("comment:novel:1")
	h.Publish("comment:novel:1", Event{Name: EventNameComment, Data: map[string]any{"id": 1}})

	ev := waitEvent(t, cl, 2*time.Second)
	batch, ok := ev.Data.(EventBatch)
	if !ok || len(batch.Events) != 1 {
		t.Fatalf("应收到含 1 条事件的批量信封: %+v", ev)
	}
	if batch.Events[0].Name != EventNameComment {
		t.Fatalf("事件名应为 comment: %s", batch.Events[0].Name)
	}
}

// TestSlowClientDropped 慢客户端（不读 channel）缓冲写满后被服务端断开
func TestSlowClientDropped(t *testing.T) {
	h := NewHub()
	defer h.Close()
	// BatchSize=1 → 每条立即 flush，快速填满 64 缓冲
	h.SetLimits(DefaultMaxConns, 50*time.Millisecond, 1)

	cl, _ := h.Subscribe("comment:novel:1")
	for i := 0; i < ClientBufferSize+10; i++ {
		h.Publish("comment:novel:1", Event{Name: EventNameComment, Data: i})
	}

	select {
	case <-cl.Done():
		// 已被服务端判定慢客户端并断开
	case <-time.After(2 * time.Second):
		t.Fatal("慢客户端未被断开（内存膨胀风险）")
	}
	if st := h.Stats(); st["clients"].(int) != 0 {
		t.Fatalf("断开后应移除该客户端: %v", st)
	}
}

// TestMaxConnsLimit 全局连接数上限：超限拒绝，退订后恢复可订阅
func TestMaxConnsLimit(t *testing.T) {
	h := NewHub()
	defer h.Close()
	h.SetLimits(2, time.Second, DefaultBatchSize)

	c1, err := h.Subscribe("comment:novel:1")
	if err != nil {
		t.Fatalf("第 1 个连接应成功: %v", err)
	}
	if _, err := h.Subscribe("comment:novel:1"); err != nil {
		t.Fatalf("第 2 个连接应成功: %v", err)
	}
	if _, err := h.Subscribe("comment:novel:1"); err == nil {
		t.Fatal("超出 MaxConns 应拒绝新连接")
	}
	h.Unsubscribe("comment:novel:1", c1)
	if _, err := h.Subscribe("comment:novel:1"); err != nil {
		t.Fatalf("退订后应恢复可订阅: %v", err)
	}
}

// TestBatchFlush 周期性批量 flush：间隔内多条评论合并为一个信封一次到达
func TestBatchFlush(t *testing.T) {
	h := NewHub()
	defer h.Close()
	// 周期 100ms、阈值 100（不会触发立即 flush，只验证周期合并）
	h.SetLimits(DefaultMaxConns, 100*time.Millisecond, 100)

	cl, _ := h.Subscribe("comment:novel:1")
	for i := 0; i < 3; i++ {
		h.Publish("comment:novel:1", Event{Name: EventNameComment, Data: i})
	}

	ev := waitEvent(t, cl, 2*time.Second)
	batch, ok := ev.Data.(EventBatch)
	if !ok || len(batch.Events) != 3 {
		t.Fatalf("批量 flush 应合并 3 条: %+v", ev)
	}
}

// TestBatchFlushImmediate 达到 BatchSize 立即 flush，不等周期
func TestBatchFlushImmediate(t *testing.T) {
	h := NewHub()
	defer h.Close()
	// 周期 30s（远大于测试超时）、阈值 2：两条一到必须立即下发
	h.SetLimits(DefaultMaxConns, 30*time.Second, 2)

	cl, _ := h.Subscribe("comment:novel:1")
	h.Publish("comment:novel:1", Event{Name: EventNameComment, Data: 1})
	h.Publish("comment:novel:1", Event{Name: EventNameComment, Data: 2})

	ev := waitEvent(t, cl, 2*time.Second)
	batch, ok := ev.Data.(EventBatch)
	if !ok || len(batch.Events) != 2 {
		t.Fatalf("达阈值应立即 flush 2 条: %+v", ev)
	}
}

// TestPublishCommentEventGate 便捷广播受全局开关控制：
// 关闭时短路（无投递），开启后经 DefaultHub 正常到达订阅者
func TestPublishCommentEventGate(t *testing.T) {
	// 便捷函数固定走 DefaultHub，直接订阅它并调短批量周期
	DefaultHub.SetEnabled(false)
	DefaultHub.SetLimits(DefaultMaxConns, 50*time.Millisecond, 1)
	cl, err := DefaultHub.Subscribe(ChannelKey("novel", 7))
	if err != nil {
		t.Fatalf("订阅 DefaultHub 失败: %v", err)
	}
	defer DefaultHub.Unsubscribe(ChannelKey("novel", 7), cl)

	PublishCommentEvent("novel", 7, map[string]any{"id": 1})
	select {
	case <-cl.Events():
		t.Fatal("未开启时不应有事件投递")
	case <-time.After(300 * time.Millisecond):
	}

	DefaultHub.SetEnabled(true)
	PublishCommentEvent("novel", 7, map[string]any{"id": 2})
	waitEvent(t, cl, 2*time.Second)
	// targetID=0 的非法调用直接忽略，不 panic
	PublishCommentEvent("novel", 0, nil)
}
