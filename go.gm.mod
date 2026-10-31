// 国密轨独立模块清单：默认 go.mod 保持零国密依赖（N-01）。
// 使用：make build-gm / build-gm-cgo（内部走 -modfile=../go.gm.mod -tags gm）
// 首次使用先 make tidy-gm 生成 go.gm.sum。
module nvs-server

go 1.26.0

require (
	github.com/aiyang-zh/zhenyi-base v1.1.5
	github.com/disintegration/imaging v1.6.2
	github.com/emmansun/gmsm v0.45.1
	github.com/fogleman/gg v1.3.0
	github.com/gin-contrib/gzip v1.2.8
	github.com/gin-gonic/gin v1.12.0
	github.com/glebarez/sqlite v1.11.0
	github.com/go-git/go-git/v5 v5.19.3
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0
	github.com/microcosm-cc/bluemonday v1.0.27
	github.com/redis/go-redis/v9 v9.23.0
	github.com/tongsuo-project/tongsuo-go-sdk v1.0.0
	github.com/yuin/goldmark v1.8.6
	golang.org/x/crypto v0.58.0
	golang.org/x/image v0.47.0
	gorm.io/driver/mysql v1.6.0
	gorm.io/gorm v1.31.2
)

require (
	dario.cat/mergo v1.0.0 // indirect
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/ProtonMail/go-crypto v1.1.6 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/bytedance/gopkg v0.1.4 // indirect
	github.com/bytedance/sonic v1.15.2 // indirect
	github.com/bytedance/sonic/loader v0.5.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudflare/circl v1.6.3 // indirect
	github.com/cloudwego/base64x v0.1.7 // indirect
	github.com/cyphar/filepath-securejoin v0.6.1 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/gabriel-vasile/mimetype v1.4.13 // indirect
	github.com/gin-contrib/sse v1.1.1 // indirect
	github.com/glebarez/go-sqlite v1.21.2 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.9.2 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.30.3 // indirect
	github.com/go-sql-driver/mysql v1.8.1 // indirect
	github.com/goccy/go-json v0.10.6 // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/css v1.0.1 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/klauspost/cpuid/v2 v2.4.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-isatty v0.0.23 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.4.3 // indirect
	github.com/pjbgf/sha1cd v0.7.0 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/quic-go/quic-go v0.60.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/sergi/go-diff v1.3.2-0.20230802210424-5b0b94c5c0d3 // indirect
	github.com/skeema/knownhosts v1.3.1 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.3.1 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	go.mongodb.org/mongo-driver/v2 v2.8.0 // indirect
	go.uber.org/atomic v1.12.0 // indirect
	golang.org/x/arch v0.29.0 // indirect
	golang.org/x/net v0.60.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	modernc.org/libc v1.22.5 // indirect
	modernc.org/mathutil v1.5.0 // indirect
	modernc.org/memory v1.5.0 // indirect
	modernc.org/sqlite v1.23.1 // indirect
)
