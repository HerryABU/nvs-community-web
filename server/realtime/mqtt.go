package realtime

// mqtt.go —— 狂暴模式可选传输层之一：内嵌 MQTT Broker（独立端口）。
//
// 设计要点：
//   - 复用 mochi-mqtt 的纯 Go Broker，无需外部中间件；仅当站长在狂暴模式配置中
//     显式开启 mqtt_enabled 时，由 realtime.Manager 拉起（独立 TCP 端口，默认 1883）。
//   - 转发出站：把 Hub 的每条原始评论事件，经 Hub 泛播 Sink 推到 MQTT 主题
//     nvs/comment/<type>/<id>（如 nvs/comment/novel/123）。MQTT 客户端（自建物联/
//     第三方桥接/任意 MQTT 5.0 客户端）订阅该主题或通配 nvs/comment/+/+ 即可实时收评。
//   - 入站（客户端→服务端）不在本特性范围：评论仍以平台原生接口提交，MQTT 仅作
//     单向实时广播渠道，避免把写路径暴露给未鉴权的外部 broker 订阅者。
//   - Sink 以 id 注册（MqttSinkID），关闭时精准注销，保证重启/关停不残留钩子。
//   - 鉴权：默认匿名可读（只读广播，无写路径），满足「友链/联邦」轻量互通诉求。

import (
	"encoding/json"
	"log"
	"net"
	"strconv"
	"strings"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
)

// MqttSinkID Hub 泛播 Sink 的注册 ID（MQTT 专线）
const MqttSinkID = "mqtt"

// MQTTBroker 内嵌 MQTT Broker 包装。
type MQTTBroker struct {
	Bind   string
	Port   int
	server *mqtt.Server
}

// channelToMqttTopic 把 Hub 频道 key（comment:novel:123）映射为 MQTT 主题 nvs/comment/novel/123。
func channelToMqttTopic(channel string) string {
	parts := strings.SplitN(channel, ":", 3)
	if len(parts) != 3 {
		return "nvs/comment/unknown"
	}
	return "nvs/" + parts[0] + "/" + parts[1] + "/" + parts[2]
}

// sink Hub 泛播回调：把事件以 JSON 线格式发布到对应 MQTT 主题。
// 必须非阻塞、快速返回——broker 正停时发布失败直接忽略，不影响 Hub 主投递。
func (b *MQTTBroker) sink(channel string, ev Event) {
	if b == nil || b.server == nil {
		return
	}
	payload, err := json.Marshal(ev)
	if err != nil {
		return
	}
	topic := channelToMqttTopic(channel)
	// 签名：Publish(topic string, payload []byte, retain bool, qos byte)
	_ = b.server.Publish(topic, payload, false, 0)
}

// StartMQTT 拉起内嵌 Broker 并注册 Hub 泛播 Sink。失败返回 error（调用方记录日志，不阻断主服务）。
func StartMQTT(bind string, port int) (*MQTTBroker, error) {
	b := &MQTTBroker{Bind: bind, Port: port}
	srv := mqtt.New(nil) // 默认匿名可读（只读广播，无写路径）
	addr := net.JoinHostPort(bind, strconv.Itoa(port))
	tcp := listeners.NewTCP(listeners.Config{Type: "tcp", ID: "nvs-rage-mqtt", Address: addr})
	if err := srv.AddListener(tcp); err != nil {
		return nil, err
	}
	b.server = srv
	go func() {
		if err := srv.Serve(); err != nil {
			log.Printf("[mqtt] broker 已停止: %v", err)
		}
	}()
	DefaultHub.AddSink(MqttSinkID, b.sink)
	log.Printf("[mqtt] Broker 已监听 %s（主题前缀 nvs/comment/...）", addr)
	return b, nil
}

// Stop 关停 Broker 并注销泛播 Sink（保存关闭 / 进程退出时调用）。
func (b *MQTTBroker) Stop() {
	DefaultHub.RemoveSink(MqttSinkID)
	if b.server != nil {
		_ = b.server.Close()
	}
	b.server = nil
}
