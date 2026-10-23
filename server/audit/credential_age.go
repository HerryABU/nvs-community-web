// Package audit 审计与自检（需求文档 §8）。
package audit

import (
	"os"
	"time"
)

// CredentialAge 单个凭据文件的年龄信息（需求文档 §6.3）。
type CredentialAge struct {
	Path    string    `json:"path"`
	ModTime time.Time `json:"mod_time"`
	AgeDays int       `json:"age_days"`
	Missing bool      `json:"missing,omitempty"` // 文件不存在
}

// CredentialAgeReport 统计各凭据文件的修改时间与距今天数（需求文档 §6.3）。
// 文件不存在记为 Missing（不视为错误）；其余 stat 失败返回错误。
func CredentialAgeReport(paths []string) ([]CredentialAge, error) {
	now := time.Now()
	out := make([]CredentialAge, 0, len(paths))
	for _, p := range paths {
		if p == "" {
			continue
		}
		info, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) {
				out = append(out, CredentialAge{Path: p, Missing: true})
				continue
			}
			return nil, err
		}
		out = append(out, CredentialAge{
			Path:    p,
			ModTime: info.ModTime(),
			AgeDays: int(now.Sub(info.ModTime()).Hours() / 24),
		})
	}
	return out, nil
}
