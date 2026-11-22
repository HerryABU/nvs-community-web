package selfhost

import (
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"nvs-server/selfhost/portmap"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

// TestDirectRouteRules 管理员直派路由的保留段与端口校验。
func TestDirectRouteRules(t *testing.T) {
	setupDB(t)
	if _, err := SaveDirectRoute(DirectRoute{Prefix: "/api/x", Port: 19099}); err == nil {
		t.Error("保留段 api 不应可作直派前缀")
	}
	if _, err := SaveDirectRoute(DirectRoute{Prefix: "/direct-demo", Port: 0}); err == nil {
		t.Error("非法端口应被拒绝")
	}
	r, err := SaveDirectRoute(DirectRoute{Prefix: "/direct-demo", Port: 19099, RootPath: "api/", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.RootPath != "/api" {
		t.Errorf("根路径归一化错误: %q", r.RootPath)
	}
	list, _ := ListEnabledDirectRoutes()
	if len(list) != 1 || list[0].Prefix != "/direct-demo" {
		t.Fatalf("直派路由列表错误: %+v", list)
	}
	if err := ToggleDirectRoute(r.ID, false); err != nil {
		t.Fatal(err)
	}
	list, _ = ListEnabledDirectRoutes()
	if len(list) != 0 {
		t.Error("停用后不应再出现在代理加载表")
	}
}

// TestPortMapValidation 自环与非法端口校验。
func TestPortMapValidation(t *testing.T) {
	setupDB(t)
	if err := validatePortMap(&PortMap{ListenPort: 19000, TargetHost: "127.0.0.1", TargetPort: 19000}); err == nil {
		t.Error("同端口自环应被拒绝")
	}
	if err := validatePortMap(&PortMap{ListenPort: 70000, TargetHost: "127.0.0.1", TargetPort: 19000}); err == nil {
		t.Error("超范围端口应被拒绝")
	}
}

// TestPortMapEcho 真起一个 TCP echo 服务，验证监听端口→目标端口的双向转发。
func TestPortMapEcho(t *testing.T) {
	targetPort := freePort(t)
	listenPort := freePort(t)
	if targetPort == listenPort {
		t.Skip("端口碰撞，跳过")
	}
	echo, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(targetPort))
	if err != nil {
		t.Fatal(err)
	}
	defer echo.Close()
	go func() {
		for {
			c, err := echo.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 128)
				n, _ := c.Read(buf)
				_, _ = c.Write(buf[:n])
			}(c)
		}
	}()

	m := portmap.New()
	m.Reload([]portmap.Endpoint{{ID: 1, ListenPort: listenPort, Target: "127.0.0.1:" + strconv.Itoa(targetPort)}}, nil)
	defer m.Reload(nil, nil)
	if !m.Running(1) {
		t.Fatal("映射未启动")
	}
	c, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(listenPort), 2*time.Second)
	if err != nil {
		t.Fatalf("连接映射端口失败: %v", err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(2 * time.Second))
	_, _ = c.Write([]byte("ping"))
	buf := make([]byte, 128)
	n, err := c.Read(buf)
	if err != nil {
		t.Fatalf("读取回显失败: %v", err)
	}
	if got := strings.TrimSpace(string(buf[:n])); got != "ping" {
		t.Fatalf("回显错误: %q", got)
	}
}
