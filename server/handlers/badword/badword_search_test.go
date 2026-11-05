package badword

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nvs-server/aiclient"
	ai "nvs-server/models/ai"
	"nvs-server/sentiment/updater"
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

// ════════════════ 检索模式判定 ════════════════

// 三种联网方式（/search 检索、模型原生联网、纯提示词）必须能按可用性正确降级。
func TestResolveBadwordSearchMode(t *testing.T) {
	plain := &ai.AiModel{}
	native := &ai.AiModel{ModelID: "gpt-4o-mini-search-preview"}
	native.SearchNative = true
	endpoint := &ai.AiModel{ModelID: "perplexity"}
	endpoint.SearchEndpoint = true

	cases := []struct {
		name            string
		requested       string
		model           *ai.AiModel
		hasSearchSource bool
		want            string
	}{
		{"auto 有检索源走 endpoint", "auto", plain, true, badwordSearchModeEndpoint},
		{"auto 只有原生走 native", "auto", native, false, badwordSearchModeNative},
		{"auto 都没有走 prompt", "auto", plain, false, badwordSearchModePrompt},
		{"auto 没传等同 auto", "", plain, true, badwordSearchModeEndpoint},
		{"显式 endpoint 有检索源", "endpoint", plain, true, badwordSearchModeEndpoint},
		{"显式 endpoint 无检索源退回 auto", "endpoint", native, false, badwordSearchModeNative},
		{"显式 endpoint 无检索源且无原生退回 prompt", "endpoint", plain, false, badwordSearchModePrompt},
		{"显式 native 模型支持", "native", native, false, badwordSearchModeNative},
		{"显式 native 模型不支持退回 prompt", "native", plain, false, badwordSearchModePrompt},
		{"显式 prompt 不联网", "prompt", native, true, badwordSearchModePrompt},
		{"大小写不敏感", "NATIVE", native, false, badwordSearchModeNative},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveBadwordSearchMode(tc.requested, tc.model, tc.hasSearchSource)
			if got != tc.want {
				t.Fatalf("模式 = %q, 期望 %q", got, tc.want)
			}
		})
	}
}

// 原生联网参数只挂在被标注支持原生联网的模型上，且必须带上 web_search_options。
func TestAttachNativeSearch(t *testing.T) {
	// 不支持的模型：不挂任何参数
	req := &aiclient.ChatRequest{}
	attachNativeSearch(req, &ai.AiModel{})
	if req.WebSearchOptions != nil || len(req.Tools) > 0 {
		t.Fatalf("未标注支持的模型不应挂联网参数: %+v", req)
	}
	// nil 模型：不能 panic
	attachNativeSearch(req, nil)
	if req.WebSearchOptions != nil {
		t.Fatalf("nil 模型不应挂联网参数")
	}

	// 支持的模型：两种参数都带上
	m := &ai.AiModel{ModelID: "x"}
	m.SearchNative = true
	req2 := &aiclient.ChatRequest{}
	attachNativeSearch(req2, m)
	if req2.WebSearchOptions == nil {
		t.Fatalf("应带上 web_search_options")
	}
	if len(req2.Tools) == 0 {
		t.Fatalf("应带上 tools")
	}
}

// 上游不认联网参数时要能识别出来（用于降级重试），普通业务错误不能误判。
func TestIsUnsupportedSearchParamError(t *testing.T) {
	yes := []string{
		"上游返回 400：Unrecognized request argument supplied: web_search_options",
		"unknown parameter: tools",
		"unsupported parameter: web_search_options",
	}
	for _, s := range yes {
		if !isUnsupportedSearchParamError(errString(s)) {
			t.Errorf("应识别为参数不支持: %q", s)
		}
	}
	no := []string{"上游连接失败：dial tcp", "上游返回 429", "上游未返回内容"}
	for _, s := range no {
		if isUnsupportedSearchParamError(errString(s)) {
			t.Errorf("不应误判: %q", s)
		}
	}
	if isUnsupportedSearchParamError(nil) {
		t.Errorf("nil 错误不应判定为参数不支持")
	}
}

type errString string

func (e errString) Error() string { return string(e) }

// ════════════════ 提示词 ════════════════

// 已经拿到检索摘要时，提示词必须改成「只依据摘要」，不能再让模型自己联网/脑补。
func TestBuildSearchAugmentSystemPrompt_Searched(t *testing.T) {
	withCtx := buildSearchAugmentSystemPrompt("赌博", true)
	if !strings.Contains(withCtx, "只依据这些检索摘要") {
		t.Errorf("检索模式应要求只依据摘要: %q", withCtx)
	}
	if strings.Contains(withCtx, "请基于联网检索") {
		t.Errorf("检索模式不应再要求模型自己联网: %q", withCtx)
	}
	if !strings.Contains(withCtx, "本次侧重方向：赌博") {
		t.Errorf("侧重方向应写入提示词: %q", withCtx)
	}

	noCtx := buildSearchAugmentSystemPrompt("", false)
	if !strings.Contains(noCtx, "请基于联网检索") {
		t.Errorf("非检索模式应保留联网提示: %q", noCtx)
	}
}

// RAG 提示词要带上标题与摘要，超长摘要要截断，且注明不要照抄原文。
func TestBuildBadwordRAGPrompt(t *testing.T) {
	hits := []updater.Snippet{
		{Title: "某论坛广告帖", Summary: "里面出现了博彩推广这个词"},
		{Title: "另一个页面", Summary: strings.Repeat("长", 500)},
	}
	p := buildBadwordRAGPrompt("赌博", hits)
	if !strings.Contains(p, "某论坛广告帖") {
		t.Errorf("应包含检索标题: %q", p)
	}
	if !strings.Contains(p, "博彩推广") {
		t.Errorf("应包含检索摘要: %q", p)
	}
	if !strings.Contains(p, "…") {
		t.Errorf("超长摘要应被截断")
	}
	if !strings.Contains(p, "不要输出原文") {
		t.Errorf("应保留不照抄原文的合规边界")
	}
	if !strings.Contains(p, "本次侧重方向：赌博") {
		t.Errorf("侧重方向应写入 RAG 提示词: %q", p)
	}
}

// ════════════════ /search 检索 ════════════════

// /search 路径要能把厂商返回的两种结构（results / data）都解析出来。
func TestBadwordSearchSnippets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["query"] == nil {
			t.Errorf("请求体应带 query: %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"title":"标题A","url":"https://a","snippet":"摘要A"}],
			"data":[{"title":"标题B","url":"https://b","description":"摘要B"}]}`))
	}))
	defer srv.Close()

	m := &ai.AiModel{BaseURL: srv.URL, ModelID: "x"}
	hits, err := badwordSearchSnippets(context.Background(), m, "sk-test", "/search", "违禁词 黑话")
	if err != nil {
		t.Fatalf("检索失败: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("条数 = %d, 期望 2 (%+v)", len(hits), hits)
	}
	if hits[0].Title != "标题A" || hits[0].Summary != "摘要A" {
		t.Errorf("results 结构解析错误: %+v", hits[0])
	}
	if hits[1].Title != "标题B" || hits[1].Summary != "摘要B" {
		t.Errorf("data 结构解析错误: %+v", hits[1])
	}
}

// 检索模型为空时要报错而不是 panic。
func TestBadwordSearchSnippets_NilModel(t *testing.T) {
	if _, err := badwordSearchSnippets(context.Background(), nil, "k", "/search", "q"); err == nil {
		t.Fatalf("nil 模型应报错")
	}
}

// 检索词要带上侧重方向；「全部」用通用词。
func TestBadwordSearchQuery(t *testing.T) {
	if q := badwordSearchQuery("赌博"); !strings.Contains(q, "赌博") {
		t.Errorf("检索词应含侧重方向: %q", q)
	}
	if q := badwordSearchQuery("全部"); strings.Contains(q, "全部") {
		t.Errorf("「全部」不应出现在检索词里: %q", q)
	}
	if badwordSearchQuery("") == "" {
		t.Errorf("空侧重方向也应给出通用检索词")
	}
}
