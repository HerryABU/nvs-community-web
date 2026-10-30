package handlers

import (
	"strings"
	"testing"
)

// 模型返回格式千奇百怪，解析器必须扛住常见的几种跑偏。
func TestParseBadWordJSON(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    int
		wantErr bool
	}{
		{
			name:    "纯数组",
			content: `[{"name":"测试词","pattern":"测试词","category":"其他"}]`,
			want:    1,
		},
		{
			name:    "带说明前缀且前缀含方括号",
			content: "以下是清单（参考[1]）：\n[{\"name\":\"甲\",\"pattern\":\"甲\"},{\"name\":\"乙\",\"pattern\":\"乙\"}]",
			want:    2,
		},
		{
			name:    "带 Markdown 代码围栏",
			content: "```json\n[{\"name\":\"甲\",\"pattern\":\"甲\"}]\n```",
			want:    1,
		},
		{
			name:    "对象包数组",
			content: `{"words":[{"name":"甲","pattern":"甲"}],"total":1}`,
			want:    1,
		},
		{
			name:    "JSONL 一行一个",
			content: "{\"name\":\"甲\",\"pattern\":\"甲\"}\n{\"name\":\"乙\",\"pattern\":\"乙\"}",
			want:    2,
		},
		{
			name:    "数组后有多余文字",
			content: `[{"name":"甲","pattern":"甲"}]\n\n希望对你有帮助！`,
			want:    1,
		},
		{
			name:    "空返回",
			content: "",
			wantErr: true,
		},
		{
			name:    "完全没有 JSON",
			content: "抱歉，我无法生成违禁词清单。",
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items, err := parseBadWordJSON(tc.content)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("期望报错，实际解析出 %d 条", len(items))
				}
				return
			}
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			if len(items) != tc.want {
				t.Fatalf("条数 = %d, 期望 %d (%+v)", len(items), tc.want, items)
			}
			for _, it := range items {
				if it.Pattern == "" {
					t.Errorf("pattern 不应为空: %+v", it)
				}
			}
		})
	}
}

// 解析出来的 pattern 必须能原样落到字面匹配（不接受正则元字符）。
func TestParseBadWordJSON_KeepsPattern(t *testing.T) {
	items, err := parseBadWordJSON(`[{"name":"甲","pattern":"甲乙丙","category":"辱骂"}]`)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(items) != 1 || items[0].Pattern != "甲乙丙" {
		t.Fatalf("pattern 解析错误: %+v", items)
	}
}

// 日志截断：超长内容被截断且带标记，换行被压平（不破坏日志行）。
func TestTruncateForLog(t *testing.T) {
	short := "abc"
	if got := truncateForLog(short, 10); got != "abc" {
		t.Errorf("短内容不应被改: %q", got)
	}
	long := strings.Repeat("字", 100) + "\n第二行"
	got := truncateForLog(long, 10)
	if !strings.Contains(got, "截断") {
		t.Errorf("超长应标记截断: %q", got)
	}
	if strings.Contains(got, "\n") {
		t.Errorf("换行应被压平: %q", got)
	}
}
