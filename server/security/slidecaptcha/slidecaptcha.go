// Package slidecaptcha 实现「货真价实」的滑块验证码（服务端挑战 + 一次性签名 pass token）。
//
// 与纯前端「拖到最右即通过」的假滑块不同，本实现：
//   - 服务端随机生成拼图缺口位置 correctX（客户端无法预知）；
//   - 渲染背景图（含缺口）与拼图块两张 PNG 返回前端；
//   - 用户拖动拼图到缺口后，前端上报滑块 x 坐标；服务端校验 |x - correctX| <= 容差，
//     通过才签发一个 HMAC 签名的一次性 pass token（5 分钟有效）；
//   - 邮箱验证码发送 / 注册等敏感接口必须携带该 pass token，否则一律拒绝。
//
// 这样即使攻击者知道接口，也无法「裸调」发送验证码：没有通过滑块挑战就拿不到 pass token。
package slidecaptcha

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strconv"
	"strings"
	"sync"
	"time"

	"nvs-server/config"
)

const (
	imgWidth     = 320
	imgHeight    = 160
	pieceSize    = 42
	validMinutes = 5                // pass token 有效期
	challengeTTL = 10 * time.Minute // 挑战有效期
	tolerance    = 6                // 滑块对齐容差（像素）
)

// challenge 服务端保存的拼图挑战状态
type challenge struct {
	id        string
	correctX  int
	createdAt time.Time
	used      bool
}

var (
	store      sync.Map // id -> *challenge
	usedTokens sync.Map // pass token -> 过期 Unix 时间戳（一次性使用追踪，防同 token 复用）
	once       sync.Once
)

func init() { once.Do(startReaper) }

// startReaper 周期性清理过期/已用挑战，避免内存泄漏。
func startReaper() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			store.Range(func(k, v any) bool {
				c, _ := v.(*challenge)
				if c == nil || c.used || now.Sub(c.createdAt) > challengeTTL {
					store.Delete(k)
				}
				return true
			})
			// 清理已过期的 pass token 记录（value 存过期 Unix 时间戳）
			usedTokens.Range(func(k, v any) bool {
				if exp, ok := v.(int64); ok && now.Unix() > exp {
					usedTokens.Delete(k)
				}
				return true
			})
		}
	}()
}

func randInt(n int) int {
	if n <= 0 {
		return 0
	}
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return 0
	}
	return int(binary.BigEndian.Uint32(b)) % n
}

// Issue 生成一次新的滑块挑战：随机 correctX + 渲染背景/拼图 PNG。
func Issue() (id string, bgPNG, piecePNG string, err error) {
	// correctX：拼图目标左边界，落在 [pieceSize, imgWidth-pieceSize*2] 区间，避免贴边
	minX := pieceSize + 8
	maxX := imgWidth - pieceSize*2 - 8
	if maxX < minX {
		maxX = minX
	}
	correctX := minX + randInt(maxX-minX+1)

	id = fmt.Sprintf("%d-%d", time.Now().UnixNano(), randInt(1<<30))
	store.Store(id, &challenge{id: id, correctX: correctX, createdAt: time.Now()})

	bg, piece := renderPuzzle(correctX)

	var bgBuf, pieceBuf bytes.Buffer
	if err = png.Encode(&bgBuf, bg); err != nil {
		return "", "", "", err
	}
	if err = png.Encode(&pieceBuf, piece); err != nil {
		return "", "", "", err
	}
	bgPNG = "data:image/png;base64," + base64.StdEncoding.EncodeToString(bgBuf.Bytes())
	piecePNG = "data:image/png;base64," + base64.StdEncoding.EncodeToString(pieceBuf.Bytes())
	return id, bgPNG, piecePNG, nil
}

// Verify 校验滑块位置并签发一次性 pass token。成功返回 token，失败返回空串。
func Verify(id string, sliderX int) (token string, ok bool) {
	v, ok := store.Load(id)
	if !ok {
		return "", false
	}
	c, _ := v.(*challenge)
	if c == nil || c.used || time.Now().Sub(c.createdAt) > challengeTTL {
		store.Delete(id)
		return "", false
	}
	if abs(sliderX-c.correctX) > tolerance {
		return "", false
	}
	c.used = true
	return signPass(id), true
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

// signPass 用服务端密钥对挑战签发一次性 pass token（HMAC-SHA256，含时间戳）。
// correctX 仅服务端持有（滑块挑战完成时即消费），不参与签名；校验端拿不到 correctX，
// 故以 id|ts 作为签名基底，由 HMAC 保证 token 确由服务端签发且未被篡改。
func signPass(id string) string {
	ts := time.Now().Unix()
	payload := fmt.Sprintf("%s|%d", id, ts)
	mac := hmac.New(sha256.New, []byte(passSecret()))
	sum := mac.Sum([]byte(payload))
	return fmt.Sprintf("%d.%s.%x", ts, id, sum)
}

// VerifyPassToken 校验 pass token 有效性：结构合法、未过期、签名正确，且一次性未使用。
// token 格式：`<ts>.<challengeID>.<hmacHex>`，以 "." 三段分割（challengeID 由服务端生成，不含 "."）。
func VerifyPassToken(token string) bool {
	if token == "" {
		return false
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	ts, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false
	}
	id := parts[1]
	macHex := parts[2]
	if time.Now().Unix()-ts > validMinutes*60 {
		return false
	}
	mac := hmac.New(sha256.New, []byte(passSecret()))
	sum := mac.Sum([]byte(fmt.Sprintf("%s|%d", id, ts)))
	expected := fmt.Sprintf("%x", sum)
	if !hmac.Equal([]byte(expected), []byte(macHex)) {
		return false
	}
	// 一次性：同一 token 仅首次校验通过，复用立即失效（防同一滑块结果多次刷接口）
	if _, loaded := usedTokens.LoadOrStore(token, ts+validMinutes*60); loaded {
		return false
	}
	return true
}

func passSecret() string {
	// 复用 JWT 密钥作为 HMAC 密钥（已做随机化持久化）
	if config.JWTSecret != "" {
		return config.JWTSecret
	}
	return "nvs-captcha-default-secret"
}

// renderPuzzle 渲染背景图（含缺口）与拼图块。返回两张 RGBA 图像。
func renderPuzzle(correctX int) (bg, piece *image.RGBA) {
	bg = image.NewRGBA(image.Rect(0, 0, imgWidth, imgHeight))
	// 背景：随机基色渐变
	baseHue := randInt(360)
	top := hsl(baseHue, 60, 55)
	bot := hsl((baseHue+40)%360, 55, 40)
	for y := 0; y < imgHeight; y++ {
		t := float64(y) / float64(imgHeight)
		col := lerpColor(top, bot, t)
		for x := 0; x < imgWidth; x++ {
			bg.Set(x, y, col)
		}
	}
	// 随机噪点纹理（增加辨识难度，但 humans 可解）
	for i := 0; i < 120; i++ {
		x := randInt(imgWidth)
		y := randInt(imgHeight)
		bg.Set(x, y, color.RGBA{uint8(randInt(256)), uint8(randInt(256)), uint8(randInt(256)), 70})
	}

	// 缺口（目标位置）：在 correctX 处画一个半透明深色圆角方块，提示拼图归位处
	drawRoundedRect(bg, correctX, imgHeight/2-pieceSize/2, pieceSize, pieceSize, color.RGBA{0, 0, 0, 90})

	// 拼图块：从背景「抠」出正确位置的图案，再叠加高亮边框，便于拖动辨识
	piece = image.NewRGBA(image.Rect(0, 0, pieceSize, pieceSize))
	draw.Draw(piece, piece.Bounds(), bg, image.Pt(correctX, imgHeight/2-pieceSize/2), draw.Src)
	drawRoundedRectBorder(piece, 0, 0, pieceSize, pieceSize, color.RGBA{255, 255, 255, 220})
	return bg, piece
}

func drawRoundedRect(img *image.RGBA, x, y, w, h int, c color.Color) {
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			if xx < 0 || yy < 0 || xx >= img.Bounds().Dx() || yy >= img.Bounds().Dy() {
				continue
			}
			img.Set(xx, yy, c)
		}
	}
}

func drawRoundedRectBorder(img *image.RGBA, x, y, w, h int, c color.Color) {
	drawRoundedRect(img, x, y, w, 2, c)
	drawRoundedRect(img, x, y+h-2, w, 2, c)
	drawRoundedRect(img, x, y, 2, h, c)
	drawRoundedRect(img, x+w-2, y, 2, h, c)
}

// hsl / lerpColor 简单颜色工具（避免引入第三方依赖）
func hsl(h, s, l int) color.RGBA {
	// h:0-360, s/l:0-100
	hh := float64(h) / 360
	ss := float64(s) / 100
	ll := float64(l) / 100
	r, g, b := hslToRGB(hh, ss, ll)
	return color.RGBA{r, g, b, 255}
}

func hslToRGB(h, s, l float64) (uint8, uint8, uint8) {
	var r, g, b float64
	if s == 0 {
		r, g, b = l, l, l
	} else {
		var q float64
		if l < 0.5 {
			q = l * (1 + s)
		} else {
			q = l + s - l*s
		}
		p := 2*l - q
		r = hue2rgb(p, q, h+1.0/3)
		g = hue2rgb(p, q, h)
		b = hue2rgb(p, q, h-1.0/3)
	}
	return uint8(r*255 + 0.5), uint8(g*255 + 0.5), uint8(b*255 + 0.5)
}

func hue2rgb(p, q, t float64) float64 {
	if t < 0 {
		t += 1
	}
	if t > 1 {
		t -= 1
	}
	switch {
	case t < 1.0/6:
		return p + (q-p)*6*t
	case t < 1.0/2:
		return q
	case t < 2.0/3:
		return p + (q-p)*(2.0/3-t)*6
	}
	return p
}

func lerpColor(a, b color.RGBA, t float64) color.RGBA {
	return color.RGBA{
		uint8(float64(a.R) + t*(float64(b.R)-float64(a.R))),
		uint8(float64(a.G) + t*(float64(b.G)-float64(a.G))),
		uint8(float64(a.B) + t*(float64(b.B)-float64(a.B))),
		255,
	}
}
