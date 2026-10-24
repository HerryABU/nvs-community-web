package honeypot

// ==================== 取证包（M4） ====================
// 导出 zip：session.json / events.jsonl / fingerprints.json / manifest.json（SHA256 清单）。
// 清单让「导出后有没有被改过」可验证；导出动作本身写一条 export 事件（审批留痕）。

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"nvs-server/models"
)

// ForensicsBundle 一次取证导出
type ForensicsBundle struct {
	Path     string    `json:"path"`
	SHA256   string    `json:"sha256"`
	Files    []string  `json:"files"`
	Bytes    int64     `json:"bytes"`
	Exported time.Time `json:"exported_at"`
}

// BuildForensicsZip 生成取证 zip（sessionID=0 导出全部）
func BuildForensicsZip(sessionID, operatorID uint) (*ForensicsBundle, error) {
	type entry struct {
		name string
		data []byte
	}
	entries := make([]entry, 0, 5)
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

	if sessionID > 0 {
		s, events, err := SessionDetail(sessionID)
		if err != nil {
			return nil, err
		}
		fps, _ := ListFingerprints(sessionID)
		if err := add("session.json", s); err != nil {
			return nil, err
		}
		if err := add("events.jsonl", events); err != nil {
			return nil, err
		}
		if err := add("fingerprints.json", fps); err != nil {
			return nil, err
		}
	} else {
		sessions, _, err := ListSessions(1, 500, "", 0)
		if err != nil {
			return nil, err
		}
		var events []Event
		_ = models.DB.Order("created_at DESC").Limit(5000).Find(&events).Error
		var fps []Fingerprint
		_ = models.DB.Order("id DESC").Limit(1000).Find(&fps).Error
		if err := add("sessions.json", sessions); err != nil {
			return nil, err
		}
		if err := add("events.jsonl", events); err != nil {
			return nil, err
		}
		if err := add("fingerprints.json", fps); err != nil {
			return nil, err
		}
	}

	// manifest 自身也带导出人与时间，便于审批追溯
	meta := map[string]interface{}{
		"exported_at": time.Now().Format(time.RFC3339),
		"operator_id": operatorID,
		"session_id":  sessionID,
		"files":       manifest,
	}
	if err := add("manifest.json", meta); err != nil {
		return nil, err
	}

	dir := filepath.Join(filepath.Dir(secretPath()), "export")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	name := fmt.Sprintf("honeypot-%d-%d.zip", sessionID, time.Now().Unix())
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
	sum, err := fileSHA256(p)
	if err == nil {
		bundle.SHA256 = sum
	}
	LogExport(sessionID, operatorID)
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
