package realname

// util.go — 包内小工具（保持各提供商文件聚焦签名与协议本身）

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
)

// errLoose 构造「响应无法识别」类错误
func errLoose(msg string) error { return errors.New(msg) }

// itoa int64 → 十进制字符串
func itoa(i int64) string { return strconv.FormatInt(i, 10) }

// formatFloat 定点字符串（两位，去尾零）
func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', 2, 64)
}

// jsonUnmarshalHelper JSON 反序列化简写
func jsonUnmarshalHelper(b []byte, v interface{}) error { return json.Unmarshal(b, v) }

// readLimited 读取响应体（上限 64KB）
func readLimited(resp *http.Response) ([]byte, error) {
	return io.ReadAll(io.LimitReader(resp.Body, 64<<10))
}
