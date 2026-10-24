package grouphandler

import (
	"testing"
	"time"

	"nvs-server/realtime"
)

// TestGroupHubDelivers 群组 Hub：订阅后能收到批量投递的事件（校验批量信封拆分前的内容）
func TestGroupHubDelivers(t *testing.T) {
	const key = "group:900001"
	cl, err := groupHub.Subscribe(key)
	if err != nil {
		t.Fatalf("订阅失败：%v", err)
	}
	defer groupHub.Unsubscribe(key, cl)

	publishGroupEvent(900001, GroupEventPostCreated, map[string]any{"post_id": 12, "title": "测试"})

	deadline := time.After(4 * time.Second)
	for {
		select {
		case ev := <-cl.Events():
			if ev.Name == realtime.EventNameBatch {
				batch, ok := ev.Data.(realtime.EventBatch)
				if !ok || len(batch.Events) == 0 {
					t.Fatalf("批量事件为空：%+v", ev.Data)
				}
				if batch.Events[0].Name != GroupEventPostCreated {
					t.Fatalf("事件名不符：%s", batch.Events[0].Name)
				}
				return
			}
			if ev.Name == GroupEventPostCreated {
				return
			}
		case <-deadline:
			t.Fatalf("4s 内未收到群组事件（批量 flush 未生效）")
		}
	}
}

// TestGroupChannelIndependent 不同群组频道互不串消息
func TestGroupChannelIndependent(t *testing.T) {
	clA, err := groupHub.Subscribe("group:900002")
	if err != nil {
		t.Fatalf("订阅失败：%v", err)
	}
	defer groupHub.Unsubscribe("group:900002", clA)

	publishGroupEvent(900003, GroupEventAnnouncement, map[string]any{"preview": "别的群"})

	select {
	case ev := <-clA.Events():
		t.Fatalf("不应收到其他群的事件：%+v", ev)
	case <-time.After(1200 * time.Millisecond):
		// 预期：另一个群的事件不会投递到本频道
	}
}
