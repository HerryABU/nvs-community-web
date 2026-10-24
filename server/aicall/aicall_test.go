package aicall

import (
	"strings"
	"testing"
)

// TestNoticeIsExplicit 降级必须能被看见：文案里要有「从哪到哪 + 原因」
func TestNoticeIsExplicit(t *testing.T) {
	r := &Result{
		ModelName: "备用B",
		Fallbacks: []FallbackNote{
			{From: "主模型A", To: "备用B", Reason: "timeout"},
		},
	}
	n := r.Notice()
	if n == "" {
		t.Fatal("降级必须有提示，不允许静默换模型")
	}
	for _, want := range []string{"主模型A", "备用B", "timeout"} {
		if !strings.Contains(n, want) {
			t.Fatalf("提示缺少 %q：%s", want, n)
		}
	}
	if !strings.Contains(r.Suffix(), "模型已降级") {
		t.Fatalf("正文后缀应包含提示：%q", r.Suffix())
	}
}

// TestNoFallbackNoNoise 没降级就不要往正文里塞提示
func TestNoFallbackNoNoise(t *testing.T) {
	r := &Result{ModelName: "主模型A"}
	if r.Notice() != "" || r.Suffix() != "" {
		t.Fatal("无降级时不应有提示")
	}
	var nilRes *Result
	if nilRes.Notice() != "" || nilRes.Suffix() != "" {
		t.Fatal("nil 结果不应 panic 或产生提示")
	}
}

// TestNoBackupIsStated 没有备用可切时也要说清楚（否则用户以为「没换模型但失败」）
func TestNoBackupIsStated(t *testing.T) {
	r := &Result{Fallbacks: []FallbackNote{{From: "唯一模型", To: "无可用备用模型", Reason: "upstream 500"}}}
	n := r.Notice()
	if !strings.Contains(n, "无可用备用模型") {
		t.Fatalf("应说明没有备用：%s", n)
	}
}
