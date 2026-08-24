package config

import (
	"log"
	"strings"
	"sync"
)

// ==================== 日志敏感信息脱敏配置（公安部令第176号合规） ====================
//
// 设计要点：
//   - .env 提供初始默认值（LOG_REDACT_SENSITIVE / LOG_REDACT_MASK / LOG_SENSITIVE_FIELDS）；
//   - 管理后台可运行时修改并持久化到 platform_configs 表（无需重启，即时生效）；
//   - 启动顺序：config.Init() 先用 .env 种子化 → DB 就绪后由 main.go 注入持久化钩子
//     并调用 LoadFromDB() 用已保存的配置覆盖 .env 默认；
//   - 脱敏核心实现（Redactor）位于 middleware/redact.go，本文件只负责配置的加载与管理，
//     避免 config → middleware / config → models 的循环依赖（models 已依赖 config）。

// 脱敏掩码常量
const (
	// DefaultRedactMask 默认脱敏掩码（非密码类敏感字段）
	DefaultRedactMask = "***REDACTED***"
	// PasswordRedactMask 密码类字段固定掩码（需求 3.2.1：password 替换为 ******）
	PasswordRedactMask = "******"
)

// DefaultSensitiveFields 默认敏感字段列表（需求 4.2，与 .env 默认一致）
var DefaultSensitiveFields = []string{"password", "token", "authorization", "cookie", "secret", "apikey"}

// RedactConfig 脱敏配置（需求 4.2）
type RedactConfig struct {
	Enabled       bool     `json:"enabled"`        // 是否启用脱敏（生产模式）
	Mask          string   `json:"mask"`           // 自定义掩码字符，默认 "***REDACTED***"
	SensitiveKeys []string `json:"sensitive_keys"` // 敏感字段列表，默认 [password token authorization cookie secret apikey]
}

// Normalize 校验并规范化配置：掩码缺省回退默认值、字段小写去重、空列表回退默认列表。
// 所有写入路径（env 种子 / DB 加载 / 后台保存）都应先经过 Normalize，保证配置永远合法。
func (rc *RedactConfig) Normalize() {
	if strings.TrimSpace(rc.Mask) == "" {
		rc.Mask = DefaultRedactMask
	}
	seen := make(map[string]bool, len(rc.SensitiveKeys))
	out := make([]string, 0, len(rc.SensitiveKeys))
	for _, k := range rc.SensitiveKeys {
		k = strings.ToLower(strings.TrimSpace(k))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	// 列表被清空时回退默认列表（安全优先：宁可多脱敏，不可明文落盘）
	if len(out) == 0 {
		out = append([]string(nil), DefaultSensitiveFields...)
	}
	rc.SensitiveKeys = out
}

// RedactManager 脱敏配置管理器：进程内单例，带读写锁，支持运行时热更新。
type RedactManager struct {
	mu      sync.RWMutex
	current RedactConfig
	// 数据库持久化钩子（由 main.go 在 DB 就绪后注入 models 的实现，避免 config 反向依赖 models）
	loadDB func() (RedactConfig, bool, error)
	saveDB func(RedactConfig) error
}

// RedactMgr 全局脱敏配置管理器
var RedactMgr = &RedactManager{}

// SeedFromEnv 以 .env 配置作为初始值（config.Init 时调用；不覆盖已加载的 DB 配置）
func (m *RedactManager) SeedFromEnv(cfg RedactConfig) {
	cfg.Normalize()
	m.mu.Lock()
	m.current = cfg
	m.mu.Unlock()
}

// SetPersistence 注入数据库持久化钩子（main.go 在 DB 就绪后调用）。
// load 返回 (配置, 是否已保存过, 错误)；save 持久化配置。
func (m *RedactManager) SetPersistence(load func() (RedactConfig, bool, error), save func(RedactConfig) error) {
	m.mu.Lock()
	m.loadDB, m.saveDB = load, save
	m.mu.Unlock()
}

// LoadFromDB 用数据库持久化配置覆盖 .env 默认（DB 就绪后调用；从未保存过则保持 .env 默认）
func (m *RedactManager) LoadFromDB() {
	m.mu.RLock()
	load := m.loadDB
	m.mu.RUnlock()
	if load == nil {
		return
	}
	cfg, found, err := load()
	if err != nil {
		log.Printf("[redact] 加载数据库脱敏配置失败: %v（继续使用 .env 默认）", err)
		return
	}
	if !found {
		return
	}
	cfg.Normalize()
	m.mu.Lock()
	m.current = cfg
	m.mu.Unlock()
	log.Printf("[redact] 已加载数据库持久化的日志脱敏配置：enabled=%v mask=%q fields=%v", cfg.Enabled, cfg.Mask, cfg.SensitiveKeys)
}

// GetConfig 返回当前脱敏配置副本（调用方可安全持有）
func (m *RedactManager) GetConfig() RedactConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

// IsEnabled 当前是否启用脱敏（日志中间件等热点路径的快捷判断）
func (m *RedactManager) IsEnabled() bool {
	return m.GetConfig().Enabled
}

// Update 更新运行时配置并持久化（管理后台调用，先落库成功再切换内存，避免内存与持久化不一致）
func (m *RedactManager) Update(cfg RedactConfig) error {
	cfg.Normalize()
	m.mu.RLock()
	save := m.saveDB
	m.mu.RUnlock()
	if save != nil {
		if err := save(cfg); err != nil {
			return err
		}
	}
	m.mu.Lock()
	m.current = cfg
	m.mu.Unlock()
	return nil
}
