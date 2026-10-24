package honeypot

// ==================== 冷归档（M4：TTL 不是直接删） ====================
// 合规留存要求「能查到、能解释」，所以超期事件先归档成 gzip JSONL，校验通过后再删库。
// 归档文件名带月份：archive/honeypot-events-202610.jsonl.gz，便于按时间取证与对账。

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ArchiveResult 归档结果
type ArchiveResult struct {
	Archived int64  `json:"archived"`
	File     string `json:"file"`
	Bytes    int64  `json:"bytes"`
	CutoffAt string `json:"cutoff_at"`
	Skipped  bool   `json:"skipped"`
}

// archiveDir 归档目录（与密钥同根，独立库模式下也在 data/honeypot 下）
func archiveDir() string {
	return filepath.Join(filepath.Dir(secretPath()), "archive")
}

// ArchiveAndCleanup 归档超期事件后删除（先落盘、再删库）
func ArchiveAndCleanup() (*ArchiveResult, error) {
	set := Current()
	days := set.RetentionDays
	if days <= 0 {
		days = 90
	}
	cutoff := time.Now().AddDate(0, 0, -days)

	var events []Event
	if err := db().Where("created_at < ?", cutoff).Order("id ASC").Limit(50000).Find(&events).Error; err != nil {
		return nil, err
	}
	res := &ArchiveResult{CutoffAt: cutoff.Format(time.RFC3339)}
	if len(events) == 0 {
		res.Skipped = true
		return res, nil
	}

	if err := os.MkdirAll(archiveDir(), 0700); err != nil {
		return nil, err
	}
	name := fmt.Sprintf("honeypot-events-%s.jsonl.gz", time.Now().Format("200601"))
	p := filepath.Join(archiveDir(), name)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	zw := gzip.NewWriter(f)
	w := bufio.NewWriter(zw)
	written := int64(0)
	ids := make([]uint, 0, len(events))
	for _, e := range events {
		b, jerr := json.Marshal(e)
		if jerr != nil {
			continue
		}
		if _, werr := w.Write(append(b, '\n')); werr != nil {
			_ = zw.Close()
			_ = f.Close()
			return nil, werr
		}
		ids = append(ids, e.ID)
		written++
	}
	if err := w.Flush(); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	if err := f.Sync(); err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	// 归档落盘成功才删库（分批，避免超长 IN 列表）
	for i := 0; i < len(ids); i += 500 {
		end := i + 500
		if end > len(ids) {
			end = len(ids)
		}
		if err := db().Where("id IN ?", ids[i:end]).Delete(&Event{}).Error; err != nil {
			return nil, err
		}
	}
	if st, err := os.Stat(p); err == nil {
		res.Bytes = st.Size()
	}
	res.Archived = written
	res.File = p
	return res, nil
}

// ListArchives 已归档文件（后台展示/下载）
type ArchiveFile struct {
	Name    string    `json:"name"`
	Bytes   int64     `json:"bytes"`
	ModTime time.Time `json:"mod_time"`
}

// ListArchives 列出归档
func ListArchives() ([]ArchiveFile, error) {
	dir := archiveDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []ArchiveFile{}, nil
		}
		return nil, err
	}
	out := make([]ArchiveFile, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, ierr := e.Info()
		if ierr != nil {
			continue
		}
		out = append(out, ArchiveFile{Name: e.Name(), Bytes: info.Size(), ModTime: info.ModTime()})
	}
	return out, nil
}
