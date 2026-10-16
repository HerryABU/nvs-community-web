// idcard.go — 中国大陆居民身份证校验（GB 11643-1999）与脱敏。
// 仅做格式/校验位/出生日期合法性校验；不做公安网真实性核验（真实性由国家实名 API 侧完成）。
package utils

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

// 身份证前两位省级行政区划代码（简化校验：覆盖 31 个省/自治区/直辖市）
var idProvinceCodes = map[string]bool{
	"11": true, "12": true, "13": true, "14": true, "15": true,
	"21": true, "22": true, "23": true,
	"31": true, "32": true, "33": true, "34": true, "35": true, "36": true, "37": true,
	"41": true, "42": true, "43": true,
	"44": true, "45": true, "46": true,
	"50": true, "51": true, "52": true, "53": true, "54": true,
	"61": true, "62": true, "63": true, "64": true, "65": true,
	"71": true, "81": true, "82": true,
	"91": true,
}

// 身份证校验位权重（GB 11643-1999）
var idWeights = [17]int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
var idCheckCodes = "10X98765432"

var (
	// ErrIDCardFormat 身份证号格式非法
	ErrIDCardFormat = errors.New("身份证号格式不正确")
	// ErrIDCardProvince 省份代码非法
	ErrIDCardProvince = errors.New("身份证号省份代码不正确")
	// ErrIDCardBirth 出生日期非法
	ErrIDCardBirth = errors.New("身份证号出生日期不正确")
	// ErrIDCardChecksum 校验位错误
	ErrIDCardChecksum = errors.New("身份证号校验位错误")
)

// ValidateIDCard 校验 18 位身份证号，返回出生日期。
func ValidateIDCard(id string) (time.Time, error) {
	id = strings.ToUpper(strings.TrimSpace(id))
	if len(id) != 18 {
		return time.Time{}, ErrIDCardFormat
	}
	// 前 17 位必须是数字
	body := id[:17]
	for i := 0; i < 17; i++ {
		if body[i] < '0' || body[i] > '9' {
			return time.Time{}, ErrIDCardFormat
		}
	}
	// 最后一位是数字或 X
	last := id[17]
	if last != 'X' && (last < '0' || last > '9') {
		return time.Time{}, ErrIDCardFormat
	}
	// 省份
	if !idProvinceCodes[id[:2]] {
		return time.Time{}, ErrIDCardProvince
	}
	// 出生日期合法性
	birth, err := time.Parse("20060102", id[6:14])
	if err != nil {
		return time.Time{}, ErrIDCardBirth
	}
	if birth.After(time.Now()) {
		return time.Time{}, ErrIDCardBirth
	}
	if birth.Year() < 1900 {
		return time.Time{}, ErrIDCardBirth
	}
	// 校验位
	sum := 0
	for i := 0; i < 17; i++ {
		d, _ := strconv.Atoi(string(body[i]))
		sum += d * idWeights[i]
	}
	expect := idCheckCodes[sum%11]
	if expect != last {
		return time.Time{}, ErrIDCardChecksum
	}
	return birth, nil
}

// MaskIDCard 脱敏：保留前 3 后 4，中间以 * 代替（110***********1234）。
func MaskIDCard(id string) string {
	id = strings.ToUpper(strings.TrimSpace(id))
	if len(id) != 18 {
		if len(id) <= 4 {
			return "****"
		}
		return id[:2] + strings.Repeat("*", len(id)-4) + id[len(id)-2:]
	}
	return id[:3] + strings.Repeat("*", 11) + id[14:]
}

// AgeAt 计算截至 when 的周岁年龄。
func AgeAt(birth time.Time, when time.Time) int {
	age := when.Year() - birth.Year()
	// 未过生日则减一
	if when.Month() < birth.Month() || (when.Month() == birth.Month() && when.Day() < birth.Day()) {
		age--
	}
	if age < 0 {
		age = 0
	}
	return age
}

// AgeFromIDCard 由身份证号推算当前周岁。
func AgeFromIDCard(id string) (int, error) {
	birth, err := ValidateIDCard(id)
	if err != nil {
		return 0, err
	}
	return AgeAt(birth, time.Now()), nil
}
