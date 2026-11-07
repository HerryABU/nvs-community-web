package realtime

// manager.go —— 传输管理器：按 RageConfig 统一起停可选传输层。
//
// 职责边界：
//   - WS：复用 Hub.Client，由 /api/live/ws 路由直接处理，无需独立进程，Manager 仅记录开关；
//   - MQTT / gRPC：各自拉起独立端口的服务，由 Manager 按配置拉起/关停（保存即生效，无需重启）。
//   - 端口/绑定变化：先停后起，保证新配置落地。
// 单例 defaultManager 由 main 启动时 ApplyConfig(当前配置) 装配，管理后台保存配置时即时重配。

import (
	"log"
	"sync"
	"time"

	"nvs-server/models/siteops"
)

// Manager 传输管理器。
type Manager struct {
	mu   sync.Mutex
	cfg  siteops.RageConfig
	mqtt *MQTTBroker
	grpc *GRPCServer
}

// defaultManager 全局单例。
var defaultManager = &Manager{}

// GetManager 返回传输管理器单例。
func GetManager() *Manager { return defaultManager }

// ApplyConfig 应用配置：按需拉起/关停 MQTT 与 gRPC。
func (m *Manager) ApplyConfig(cfg siteops.RageConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = cfg

	// 同步 Hub 开关与限额：启动（main 装配）与后台保存都要立即生效，
	// 否则重启后 gRPC Stream 会因 Hub.enabled 仍为 false 而返回「未开启」。
	DefaultHub.SetEnabled(cfg.Enabled)
	DefaultHub.SetLimits(cfg.MaxConns, time.Duration(cfg.BatchIntervalSec)*time.Second, cfg.BatchSize)

	// ── MQTT ──
	if cfg.MqttEnabled {
		if m.mqtt == nil {
			b, err := StartMQTT(cfg.MqttBind, cfg.MqttPort)
			if err != nil {
				log.Printf("[rage] MQTT 启动失败: %v", err)
			} else {
				m.mqtt = b
			}
		} else if m.mqtt.Bind != cfg.MqttBind || m.mqtt.Port != cfg.MqttPort {
			m.mqtt.Stop()
			b, err := StartMQTT(cfg.MqttBind, cfg.MqttPort)
			if err != nil {
				log.Printf("[rage] MQTT 重启失败: %v", err)
				m.mqtt = nil
			} else {
				m.mqtt = b
			}
		}
	} else if m.mqtt != nil {
		m.mqtt.Stop()
		m.mqtt = nil
	}

	// ── gRPC ──
	if cfg.GrpcEnabled {
		if m.grpc == nil {
			g, err := StartGRPC(cfg.GrpcBind, cfg.GrpcPort)
			if err != nil {
				log.Printf("[rage] gRPC 启动失败: %v", err)
			} else {
				m.grpc = g
			}
		} else if m.grpc.Bind != cfg.GrpcBind || m.grpc.Port != cfg.GrpcPort {
			m.grpc.Stop()
			g, err := StartGRPC(cfg.GrpcBind, cfg.GrpcPort)
			if err != nil {
				log.Printf("[rage] gRPC 重启失败: %v", err)
				m.grpc = nil
			} else {
				m.grpc = g
			}
		}
	} else if m.grpc != nil {
		m.grpc.Stop()
		m.grpc = nil
	}
}

// Config 返回当前生效的狂暴模式配置（公开状态查询用）。
func (m *Manager) Config() siteops.RageConfig {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg
}

// Status 返回传输层运行态（管理后台展示）。
func (m *Manager) Status() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return map[string]any{
		"enabled":      m.cfg.Enabled,
		"ws_enabled":   m.cfg.WsEnabled,
		"mqtt_enabled": m.cfg.MqttEnabled,
		"mqtt_running": m.mqtt != nil,
		"mqtt_bind":    m.cfg.MqttBind,
		"mqtt_port":    m.cfg.MqttPort,
		"grpc_enabled": m.cfg.GrpcEnabled,
		"grpc_running": m.grpc != nil,
		"grpc_bind":    m.cfg.GrpcBind,
		"grpc_port":    m.cfg.GrpcPort,
	}
}
