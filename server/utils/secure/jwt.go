package secure

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"nvs-server/config"
	"nvs-server/ipgeo"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID uint   `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	// Binding 令牌绑定签名 = HMAC-SHA256(fingerprint + "\n" + region, JWTSecret)。
	// 把「设备指纹 + 登录地（省级）」加密进令牌，使令牌与设备/IP 强绑定：
	// 窃取到的令牌在异设备 / 异省份使用时，后端重算签名不一致 → 401 拒绝。
	// 为空表示升级前签发的遗留令牌，中间件对此放行（向后兼容）。
	Binding string `json:"binding,omitempty"`
	jwt.RegisteredClaims
}

// ComputeBinding 计算令牌绑定签名。
// fingerprint 为前端采集的设备指纹（空则仅以 region 兜底，降级为纯地理绑定）；
// region 为登录时客户端 IP 的省级归属地（"国家代码|省份"，如 "CN|广东省"），
// 取省级粒度从而容忍省内 IP 变化（家庭宽带重拨、基站切换），跨省份 / 跨国则破坏绑定。
func ComputeBinding(fingerprint, region string) string {
	if fingerprint == "" && region == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(config.JWTSecret))
	mac.Write([]byte(fingerprint + "\n" + region))
	return hex.EncodeToString(mac.Sum(nil))
}

// RegionForBinding 把客户端 IP 解析成「国家代码|省份」粗粒度归属地（供绑定用）。
// 内网 / 保留地址 / 解析失败统一返回空串 —— 此时绑定退化为仅依赖设备指纹（仍可防异设备盗用）。
func RegionForBinding(ip string) string {
	r := ipgeo.Lookup(ip)
	if r.CountryCD == "" && r.Country == "" {
		return "" // 内网 / 保留 / 解析失败：降级为指纹单因子绑定
	}
	prov := r.Province
	if prov != "" {
		prov = ipgeo.NormalizeProvince(prov)
	}
	if r.CountryCD != "" {
		return r.CountryCD + "|" + prov
	}
	return r.Country + "|" + prov
}

// RandomJTI 生成不可预测的刷新令牌族 ID（32 hex = 16 字节随机），用于刷新令牌轮换。
func RandomJTI() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func GenerateToken(userID uint, email, role, fingerprint, region string) (string, error) {
	expireMin, err := strconv.Atoi(config.JWTAccessExpireMinutes)
	if err != nil || expireMin <= 0 {
		expireMin = 15
	}

	claims := Claims{
		UserID:  userID,
		Email:   email,
		Role:    role,
		Binding: ComputeBinding(fingerprint, region),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expireMin) * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "nvs-server",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.JWTSecret))
}

// RefreshClaims 刷新令牌载荷（长期有效，用于无感续期短期访问令牌）。
// JTI 为刷新令牌族 ID：每次刷新都会轮换出新 JTI 并作废旧值，
// 使被盗用的旧刷新令牌立即失效（纵深防御刷新令牌窃取）。
type RefreshClaims struct {
	UserID uint   `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	JTI    string `json:"jti,omitempty"`
	jwt.RegisteredClaims
}

// GenerateRefreshToken 生成长期刷新令牌（默认 720 小时 / 30 天，可由 JWT_REFRESH_EXPIRE_HOURS 配置）。
// 刷新令牌与访问令牌使用相同签名密钥，但有效期更长，且仅在 /api/auth/refresh 端点校验。
// jti 为该刷新令牌的族 ID（刷新轮换时由调用方生成并登记到 RefreshState）。
func GenerateRefreshToken(userID uint, email, role, jti string) (string, error) {
	expireHours, err := strconv.Atoi(config.JWTRefreshExpireHours)
	if err != nil || expireHours <= 0 {
		expireHours = 720
	}

	claims := RefreshClaims{
		UserID: userID,
		Email:  email,
		Role:   role,
		JTI:    jti,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "nvs-server-refresh",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.JWTSecret))
}

// ParseRefreshToken 校验刷新令牌并返回其载荷（签名/过期非法均返回 error）。
func ParseRefreshToken(tokenString string) (*RefreshClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &RefreshClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(config.JWTSecret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*RefreshClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid refresh token")
	}
	return claims, nil
}

func ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(config.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}
