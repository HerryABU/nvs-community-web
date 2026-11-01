package slidecaptcha

import (
	"fmt"
	"os"
	"testing"

	"nvs-server/config"
)

// TestQaPrintPassToken 给真浏览器验收脚本产出一个一次性滑块 pass token。
//
// 背景：管理端登录要过人机校验（滑块），而真浏览器验收脚本要拿到登录态。
// 站长的测试实例是授权可测的，但滑块只能靠拖动图片求解 → 自动化跑不通。
//
// 用法（注意 -count=1，Go 的测试缓存会复用旧结果，导致 token 被重复消费而 403）：
//
//	SECRET=$(cat <实例>/data/.jwt_secret)
//	QA_JWT_SECRET="$SECRET" go test -count=1 ./security/slidecaptcha/ -run TestQaPrintPassToken -v
//
// 输出 QA_TOKEN=<ts>.<id>.<hmac>，直接作为登录接口的 captcha_token 传入。
//
// ⚠️ 这是**本地/测试实例**的辅助工具：passSecret() 复用 JWT 密钥，
// 生产环境请勿用本文件生成 token（生产实例密钥不外泄，也不需要）。
func TestQaPrintPassToken(t *testing.T) {
	if s := os.Getenv("QA_JWT_SECRET"); s != "" {
		config.JWTSecret = s
	}
	fmt.Println("QA_TOKEN=" + signPass("qa"+fmt.Sprint(os.Getpid())))
}
