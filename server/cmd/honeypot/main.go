// nvs-honeypot：独立蜜罐进程（M4 隔离形态）
//
// 为什么要独立：蜜罐是**故意暴露**的面，一旦被写入/打崩，不能连累主站。
// 因此它用独立端口、独立数据库，只挂诱饵/影子认证/验证页，不加载任何业务路由。
//
// 启动：
//
//	go build -o nvs-honeypot.exe ./cmd/honeypot
//	HONEYPOT_PORT=18081 NVS_HONEYPOT_DB_PATH=data/honeypot/honeypot.db ./nvs-honeypot.exe
//
// 反向代理：把诱饵路径（/.env、/.git、/admin.php、/hp/*、/api/v2/*）指到本进程；
// 主站仍需 NVS_HONEYPOT_DB_PATH 指向**同一个库**，后台才能看到这里的命中记录。
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"nvs-server/honeypot"

	"github.com/gin-gonic/gin"
)

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func main() {
	port := env("HONEYPOT_PORT", "18081")
	dbPath := env("NVS_HONEYPOT_DB_PATH", filepath.Join("data", "honeypot", "honeypot.db"))

	// 隔离断言：蜜罐库不得与主库同路径
	if mainDB := strings.TrimSpace(os.Getenv("DB_PATH")); mainDB != "" {
		mAbs, _ := filepath.Abs(mainDB)
		hAbs, _ := filepath.Abs(dbPath)
		if mAbs == hAbs {
			log.Fatalf("[honeypot] 蜜罐库不能与主库同路径（%s），隔离失效", dbPath)
		}
	}

	if err := honeypot.OpenIsolated(dbPath); err != nil {
		log.Fatalf("[honeypot] 打开蜜罐库失败：%v", err)
	}
	log.Printf("[honeypot] 独立库：%s", dbPath)

	if env("GIN_MODE", "") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())

	// 诱饵 + 影子认证 + 验证页（只有这三类，不挂任何业务接口）
	honeypot.Register(r)
	honeypot.RegisterShadow(r)
	honeypot.RegisterVerify(r)

	r.GET("/__hp/health", func(c *gin.Context) {
		set := honeypot.Current()
		c.JSON(http.StatusOK, gin.H{
			"ok": true, "level": set.Level, "enabled": set.Enabled,
			"db": dbPath, "isolated": honeypot.Isolated(),
		})
	})

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Printf("[honeypot] 监听 :%s（等级由主站后台控制，改完 15s 内生效）", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[honeypot] 启动失败：%v", err)
		}
	}()

	// 优雅停机
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Printf("[honeypot] 已停止（端口 %s）", strconv.Quote(port))
}
