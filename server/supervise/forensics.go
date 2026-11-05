package supervise

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"nvs-server/aisafety"
	"nvs-server/models"
	ai "nvs-server/models/ai"
)

// ForensicsBundle 一次名单取证导出。
type ForensicsBundle struct {
	Path     string    `json:"path"`
	SHA256   string    `json:"sha256"`
	Files    []string  `json:"files"`
	Bytes    int64     `json:"bytes"`
	Exported time.Time `json:"exported_at"`
}

// BuildSuperviseForensicsZip 为名入档用户生成取证 zip（supervised.json / sessions.json /
// messages.json / judge_logs.json / safety_events.json / manifest.json）。
//
// 设计取舍（与 risk / police / honeypot 同口径）：
//   - 取证包不含任何原始 PII 之外的额外采集，只汇总该用户在站内的对话留痕与监管结论；
//   - manifest 自带导出人与导出时间，且对每份文件算 SHA256，使「导出后是否被改动」可验证；
//   - 导出动作由调用方（admin 直接 / police 经审批）触发，审批留痕在 police 包完成。
func BuildSuperviseForensicsZip(targetUserID, operatorID uint) (*ForensicsBundle, error) {
	if targetUserID == 0 {
		return nil, fmt.Errorf("目标用户 ID 非法")
	}
	su, err := GetByUser(targetUserID)
	if err != nil {
		// 不在档也允许导出（可能只是被申请取证），记录留空
		su = nil
	}

	type entry struct {
		name string
		data []byte
	}
	entries := make([]entry, 0, 6)
	manifest := map[string]string{}

	add := func(name string, v interface{}) error {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		manifest[name] = hex.EncodeToString(sum[:])
		entries = append(entries, entry{name: name, data: b})
		return nil
	}

	if err := add("supervised.json", su); err != nil {
		return nil, err
	}

	// AI 对话留痕（绥靖/名单场景最核心的证据：用户实际问了什么、模型回了什么）
	sessions, _ := ai.ListAiChatSessions(targetUserID, 500)
	if err := add("sessions.json", sessions); err != nil {
		return nil, err
	}
	msgs := make([]ai.AiChatMessage, 0, 200)
	for _, s := range sessions {
		list, _ := ai.ListAiChatMessages(s.ID, 500)
		msgs = append(msgs, list...)
	}
	if err := add("messages.json", msgs); err != nil {
		return nil, err
	}

	// 监管模型 B 判定日志
	var judgeLogs []aisafety.JudgeLog
	_ = models.DB.Where("user_id = ?", targetUserID).Order("id ASC").Find(&judgeLogs).Error
	if err := add("judge_logs.json", judgeLogs); err != nil {
		return nil, err
	}

	// 安全事件留痕
	var events []aisafety.SafetyEvent
	_ = models.DB.Where("user_id = ?", targetUserID).Order("id ASC").Find(&events).Error
	if err := add("safety_events.json", events); err != nil {
		return nil, err
	}

	// manifest 自带导出人与时间，便于审批追溯
	meta := map[string]interface{}{
		"exported_at":    time.Now().Format(time.RFC3339),
		"operator_id":    operatorID,
		"target_user_id": targetUserID,
		"files":          manifest,
	}
	if err := add("manifest.json", meta); err != nil {
		return nil, err
	}

	dir, err := os.MkdirTemp("", "supervise-forensics-")
	if err != nil {
		return nil, err
	}
	name := fmt.Sprintf("supervise_%d_%d.zip", targetUserID, time.Now().Unix())
	p := filepath.Join(dir, name)
	f, err := os.Create(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	bundle := &ForensicsBundle{Path: p, Exported: time.Now()}
	for _, e := range entries {
		w, werr := zw.Create(e.name)
		if werr != nil {
			return nil, werr
		}
		if _, werr := w.Write(e.data); werr != nil {
			return nil, werr
		}
		bundle.Files = append(bundle.Files, e.name)
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	if st, err := os.Stat(p); err == nil {
		bundle.Bytes = st.Size()
	}
	if sum, err := fileSHA256(p); err == nil {
		bundle.SHA256 = sum
	}
	return bundle, nil
}

func fileSHA256(p string) (string, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
