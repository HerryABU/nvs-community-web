# 国密能力 × 密评指标映射（非权威，仅供站长备查）

> **免责声明**：本表**不构成合规建议**，也不替代密评。密评结论以测评机构正式报告为准；
> 等保定级备案、密评、密码模块（GM/T 0028）合规均由站长自行完成。
> 框架只提供 Go 层可观测能力与配置开关，**默认不启用国密**。

## 1. 能力对照（GB/T 39786-2021 · 网络和通信安全，参考项）

| 指标（参考） | NVS 框架提供的 Go 层能力 | 落点 | 站长需自行完成 |
|---|---|---|---|
| 8.2 a) 通信实体身份鉴别 | SM2 双证书（签名 + 加密）加载与 OID 识别 | `server/cert.LoadDualCertificate`（`//go:build gm`） | 签发 SM2 双证书、私钥保管、测评举证 |
| 通信数据机密性 | TLCP 套件 `0xE011`(SM4-CBC) / `0xE051`(SM4-GCM) | `server/crypto`（gm 轨按 `GMProvider` 分发） | 确认所有对外通道启用、反向代理不降级 |
| 通信数据完整性 | SM3（国密 TLS 内建） | 同上 | 确认覆盖关键字段与关键路径 |
| 密码算法合规性 | SM2 / SM3 / SM4 全链路（纯 Go 或 Tongsuo CGO） | `crypto/gm_purego.go`、`crypto/gm_tongsuo.go` | 密码模块需满足 GM/T 0028；框架不替代硬件/软件密码模块认证 |
| 协商可观测 | 握手记录 JSONL（协议 / 套件 / SNI / 时间 / 错误） | `server/tlsconfig` | 保存日志、按密评要求留存 |
| 会话复用控制 | `SessionTicketsDisabled` 配置字段 | `config.CryptoConfig` | 决定是否开启会话复用 |

## 2. 双轨与构建

| 轨道 | 构建 | 依赖 | 说明 |
|---|---|---|---|
| 国际轨（默认） | `make build` | 零国密模块 | `go.mod` 不含国密依赖，`go version -m` 可验证 |
| 纯 Go 国密轨 | `make build-gm` | `go.gm.mod`（gmsm + zhenyi-base/zgmtls） | 无 CGO，适合交叉编译 |
| CGO 国密轨 | `make build-gm-cgo` | 另需 Tongsuo（`--enable-ntls`）与 `TONGSUO_HOME` | 与 C 层 TLS 栈行为一致 |

配置（`.env`）：

```
NVS_CRYPTO_TRACK=intl        # intl | gm | dual
NVS_CRYPTO_GM_PROVIDER=      # purego | tongsuo（track=gm/dual 时必填，否则启动即报错）
NVS_CRYPTO_INTL_CERT_FILE=
NVS_CRYPTO_INTL_KEY_FILE=
NVS_CRYPTO_SM_SIGN_CERT_FILE=  NVS_CRYPTO_SM_SIGN_KEY_FILE=
NVS_CRYPTO_SM_ENC_CERT_FILE=   NVS_CRYPTO_SM_ENC_KEY_FILE=
NVS_CRYPTO_SESSION_TICKETS_DISABLED=false
NVS_CRYPTO_HANDSHAKE_LOG=      # 握手 JSONL 路径，留空不落盘
```

## 3. 自检与审计

| 命令 | 产出 |
|---|---|
| `make gm-audit`（= `go run . gm-audit`） | Markdown 报告：轨道 / 实现路径 / 证书存在性 / 证书有效期 / 握手统计 / 会话复用状态 |
| `make credential-age` | 凭据文件年龄报表（修改时间 + 距今天数） |
| `make scan-secrets` | 扫描前端产物中的疑似密钥，命中即非零退出 |
| `make test-gm` | 国密轨单测（需先 `make tidy-gm`） |

## 4. 已知边界

1. **本机未做 gm 轨端到端握手验证**：Windows 开发机无 Tongsuo，且 `-tags gm` 需先下载 gmsm/zgmtls 依赖（`make tidy-gm`）。
   代码路径、接口签名、构建标签均已就位，站长在 Linux + Tongsuo 环境执行验收 2/3/4 即可。
2. **`dual` 模式的回退隔离**（N-02）依赖 gmtls 的 `GetCertificate` 分支；国际轨 panic 隔离由该分支独立函数保证，
   但**尚缺故障注入测试**，建议在站长环境补一次「故意配错 SM2 证书 → Chrome 仍可访问」的验证。
3. 框架不实现实名认证与内容过滤（N-05），NVS 的应用层实现见 `server/security`、`server/handlers`。
