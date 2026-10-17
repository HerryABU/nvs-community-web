package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withTempEnvFile 把 .env 同步目标切到临时文件，返回该路径
func withTempEnvFile(t *testing.T, initial string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	if initial != "" {
		if err := os.WriteFile(p, []byte(initial), 0644); err != nil {
			t.Fatalf("准备临时 .env 失败: %v", err)
		}
	}
	SetEnvFilePath(p)
	t.Cleanup(func() { SetEnvFilePath("") })
	return p
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", p, err)
	}
	return string(b)
}

func TestSyncEnvFileReplaceInPlace(t *testing.T) {
	initial := "# 注释1\nSITE_NAME=旧站名\nEMAIL_VERIFY_ENABLED=false\n# 注释2\nSMTP_HOST=smtp.old.com\n"
	p := withTempEnvFile(t, initial)

	appended, err := SyncEnvFile(map[string]string{
		"SITE_NAME":            "新站名 星海",
		"EMAIL_VERIFY_ENABLED": "true",
	})
	if err != nil {
		t.Fatalf("SyncEnvFile 失败: %v", err)
	}
	if len(appended) != 0 {
		t.Errorf("不应有追加键，实际 %v", appended)
	}
	out := readFile(t, p)
	// 原位替换：注释与无关行保留，顺序不变
	if !strings.Contains(out, "# 注释1\n") || !strings.Contains(out, "# 注释2\n") {
		t.Errorf("注释行丢失:\n%s", out)
	}
	if !strings.Contains(out, "SITE_NAME=\"新站名 星海\"\n") {
		t.Errorf("含空格值应加引号:\n%s", out)
	}
	if !strings.Contains(out, "EMAIL_VERIFY_ENABLED=true\n") {
		t.Errorf("布尔值替换失败:\n%s", out)
	}
	if !strings.Contains(out, "SMTP_HOST=smtp.old.com\n") {
		t.Errorf("无关行被误改:\n%s", out)
	}
}

func TestSyncEnvFileAppendMissing(t *testing.T) {
	p := withTempEnvFile(t, "SERVER_PORT=8080\n")

	appended, err := SyncEnvFile(map[string]string{
		"SITE_NAME":     "星海文学",
		"SMTP_PASSWORD": "auth code#1",
	})
	if err != nil {
		t.Fatalf("SyncEnvFile 失败: %v", err)
	}
	if len(appended) != 2 || appended[0] != "SITE_NAME" || appended[1] != "SMTP_PASSWORD" {
		t.Errorf("追加键列表应为排序后两项，实际 %v", appended)
	}
	out := readFile(t, p)
	if !strings.Contains(out, "SERVER_PORT=8080\n") {
		t.Errorf("原有行丢失:\n%s", out)
	}
	if !strings.Contains(out, "SITE_NAME=星海文学\n") {
		t.Errorf("中文无空格值不应加引号:\n%s", out)
	}
	if !strings.Contains(out, "SMTP_PASSWORD=\"auth code#1\"\n") {
		t.Errorf("含#值应加引号:\n%s", out)
	}
}

func TestSyncEnvFileCreateWhenMissing(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env") // 不存在
	SetEnvFilePath(p)
	t.Cleanup(func() { SetEnvFilePath("") })

	if _, err := SyncEnvFile(map[string]string{"SITE_NAME": "新站"}); err != nil {
		t.Fatalf("SyncEnvFile 失败: %v", err)
	}
	out := readFile(t, p)
	if !strings.Contains(out, "SITE_NAME=新站\n") {
		t.Errorf("新建 .env 内容异常:\n%s", out)
	}
}

func TestSyncEnvFileCRLFCompatible(t *testing.T) {
	initial := "# c\r\nSITE_NAME=旧\r\nSMTP_HOST=smtp.old.com\r\n"
	p := withTempEnvFile(t, initial)

	if _, err := SyncEnvFile(map[string]string{"SITE_NAME": "新站"}); err != nil {
		t.Fatalf("SyncEnvFile 失败: %v", err)
	}
	out := readFile(t, p)
	if !strings.Contains(out, "\r\n") {
		t.Errorf("CRLF 行尾应保留:\n%q", out)
	}
	if !strings.Contains(out, "SITE_NAME=新站\r\n") {
		t.Errorf("CRLF 文件替换失败:\n%q", out)
	}
	if !strings.Contains(out, "SMTP_HOST=smtp.old.com\r\n") {
		t.Errorf("CRLF 无关行被误改:\n%q", out)
	}
}

func TestSyncEnvFileRejectsInvalidKey(t *testing.T) {
	p := withTempEnvFile(t, "SITE_NAME=旧\n")

	// 非法键（含换行/等号/空格）必须被跳过，不得写入文件
	if _, err := SyncEnvFile(map[string]string{
		"BAD KEY":  "x",
		"BAD=KEY":  "x",
		"BAD\nKEY": "x",
		"":         "x",
	}); err != nil {
		t.Fatalf("SyncEnvFile 失败: %v", err)
	}
	out := readFile(t, p)
	if out != "SITE_NAME=旧\n" {
		t.Errorf("非法键污染了文件:\n%q", out)
	}
}

func TestSyncEnvFileDuplicateKeyFirstWins(t *testing.T) {
	// 与加载端语义一致：同键多次出现只改第一处
	p := withTempEnvFile(t, "SITE_NAME=第一处\nSITE_NAME=第二处\n")

	if _, err := SyncEnvFile(map[string]string{"SITE_NAME": "新值"}); err != nil {
		t.Fatalf("SyncEnvFile 失败: %v", err)
	}
	out := readFile(t, p)
	if !strings.Contains(out, "SITE_NAME=新值\n") || !strings.Contains(out, "SITE_NAME=第二处\n") {
		t.Errorf("重复键处理异常:\n%q", out)
	}
}

func TestSyncPlatformConfigEnvMapping(t *testing.T) {
	p := withTempEnvFile(t, "SITE_NAME=旧\nLOG_SENSITIVE_FIELDS=\"password,token\"\n")

	synced, err := SyncPlatformConfigEnv(map[string]string{
		"site_name":          "星海文学",
		"email_verify":       "true",
		"captcha_enabled":    "false",
		"smtp_host":          "smtp.qq.com",
		"log_redact_fields":  `["password","token","id_card"]`,
		"novel_visitor_mode": "login_required", // 无 env 映射，应被忽略
		"unknown_key":        "x",
	})
	if err != nil {
		t.Fatalf("SyncPlatformConfigEnv 失败: %v", err)
	}
	if len(synced) != 5 {
		t.Errorf("应同步 5 个 env 键，实际 %v", synced)
	}
	out := readFile(t, p)
	for _, want := range []string{
		"SITE_NAME=星海文学\n",
		"EMAIL_VERIFY_ENABLED=true\n",
		"CAPTCHA_ENABLED=false\n",
		"SMTP_HOST=smtp.qq.com\n",
		"LOG_SENSITIVE_FIELDS=password,token,id_card\n", // JSON 数组 → CSV
	} {
		if !strings.Contains(out, want) {
			t.Errorf("缺少 %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "novel_visitor_mode") || strings.Contains(out, "login_required") {
		t.Errorf("无映射键不应写入 .env:\n%s", out)
	}
}

func TestSyncPlatformConfigEnvNoMappedKeys(t *testing.T) {
	withTempEnvFile(t, "SITE_NAME=旧\n")
	synced, err := SyncPlatformConfigEnv(map[string]string{
		"home_module": "blogs",
		"minor_mode":  "strict",
	})
	if err != nil {
		t.Fatalf("SyncPlatformConfigEnv 失败: %v", err)
	}
	if synced != nil {
		t.Errorf("无映射键应返回 nil，实际 %v", synced)
	}
}

func TestJsonArrayToCSV(t *testing.T) {
	cases := map[string]string{
		`["a","b","c"]`:    "a,b,c",
		`[" a ", "", "b"]`: "a,b",
		"a,b,c":            "a,b,c",
		"":                 "",
		"  ":               "",
	}
	for in, want := range cases {
		if got := jsonArrayToCSV(in); got != want {
			t.Errorf("jsonArrayToCSV(%q) = %q，期望 %q", in, got, want)
		}
	}
}
