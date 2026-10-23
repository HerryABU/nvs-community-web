// Package tlsconfig 提供 TLS/TLCP 握手结果的可观测能力（需求文档 §5.4）。
//
// 国际轨与国密轨统一经 Observer 记录：协议版本、密码套件、SNI、失败原因，
// 以 JSON Lines 写入站长服务器本地文件（默认关闭，不内置远程上报，N-04）。
package tlsconfig

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// VersionGMSSL TLCP 协议版本标识（GM/T 0024-2014，GB/T 38636-2020）。
const VersionGMSSL = 0x0101

// HandshakeRecord 单次握手的可观测记录。
type HandshakeRecord struct {
	Protocol    string    `json:"protocol"`     // "tlcp" / "tls12" / "tls13" ...
	CipherSuite uint16    `json:"cipher_suite"` // 0xE011 / 0xE051 / 0x1301 ...
	ServerName  string    `json:"server_name"`  // SNI（可能为空）
	Timestamp   time.Time `json:"timestamp"`
	Err         string    `json:"err,omitempty"` // 握手失败原因（成功为空）
}

// ProtocolName 协议版本号 → 可读名称。
func ProtocolName(version uint16) string {
	switch version {
	case VersionGMSSL:
		return "tlcp"
	case tls.VersionTLS10:
		return "tls10"
	case tls.VersionTLS11:
		return "tls11"
	case tls.VersionTLS12:
		return "tls12"
	case tls.VersionTLS13:
		return "tls13"
	}
	return fmt.Sprintf("0x%04x", version)
}

// Stats 握手统计（内存聚合，供 gm-audit / 运维查询）。
type Stats struct {
	Total    int64            `json:"total"`
	Failed   int64            `json:"failed"`
	ByProto  map[string]int64 `json:"by_proto"`
	BySuite  map[string]int64 `json:"by_suite"`
}

// Observer 全局握手观测器。零值可用（关闭状态）。
type Observer struct {
	mu     sync.Mutex
	file   *os.File
	path   string
	stats  Stats
}

var global = &Observer{stats: Stats{ByProto: map[string]int64{}, BySuite: map[string]int64{}}}

// Configure 配置 JSONL 输出路径；path 为空 = 关闭落盘（仅内存统计）。
func Configure(path string) error {
	global.mu.Lock()
	defer global.mu.Unlock()
	if global.file != nil {
		_ = global.file.Close()
		global.file = nil
	}
	global.path = path
	if path == "" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("打开握手记录文件失败: %w", err)
	}
	global.file = f
	return nil
}

// Record 记录一次握手结果（并发安全）。
func Record(rec HandshakeRecord) {
	global.mu.Lock()
	defer global.mu.Unlock()
	if rec.Timestamp.IsZero() {
		rec.Timestamp = time.Now()
	}
	global.stats.Total++
	if rec.Err != "" {
		global.stats.Failed++
	} else {
		global.stats.ByProto[rec.Protocol]++
		global.stats.BySuite[fmt.Sprintf("0x%04X", rec.CipherSuite)]++
	}
	if global.file == nil {
		return
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return
	}
	line = append(line, '\n')
	// 握手频率低，逐条落盘即可，无需缓冲
	_, _ = global.file.Write(line)
}

// StatsSnapshot 当前内存统计快照。
func StatsSnapshot() Stats {
	global.mu.Lock()
	defer global.mu.Unlock()
	out := Stats{
		Total:   global.stats.Total,
		Failed:  global.stats.Failed,
		ByProto: make(map[string]int64, len(global.stats.ByProto)),
		BySuite: make(map[string]int64, len(global.stats.BySuite)),
	}
	for k, v := range global.stats.ByProto {
		out.ByProto[k] = v
	}
	for k, v := range global.stats.BySuite {
		out.BySuite[k] = v
	}
	return out
}

// LogPath 当前配置的落盘路径（空 = 关闭）。
func LogPath() string {
	global.mu.Lock()
	defer global.mu.Unlock()
	return global.path
}

// Close 关闭观测器（进程退出时调用）。
func Close() {
	global.mu.Lock()
	defer global.mu.Unlock()
	if global.file != nil {
		_ = global.file.Close()
		global.file = nil
	}
}
