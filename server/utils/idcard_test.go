package utils

import (
	"strings"
	"testing"
	"time"
)

type timeType = time.Time

// 公安部公示的样例号段 + 自校验位构造的合法号码
func TestValidateIDCard(t *testing.T) {
	// 11010519491231002X 是广泛使用的 GB11643 校验样例（1949-12-31，男）
	valid := "11010519491231002X"
	birth, err := ValidateIDCard(valid)
	if err != nil {
		t.Fatalf("合法身份证应通过: %v", err)
	}
	if birth.Year() != 1949 || birth.Month() != 12 || birth.Day() != 31 {
		t.Errorf("出生日期解析错误: %v", birth)
	}

	// 小写 x 应被接受
	if _, err := ValidateIDCard(strings.ToLower(valid)); err != nil {
		t.Errorf("小写 x 应被接受: %v", err)
	}

	cases := []struct {
		name string
		id   string
	}{
		{"长度不足", "1101051949123100"},
		{"含非法字符", "1101051949123100YZ"},
		{"省份代码非法", "99" + "110519491231002X"},
		{"出生日期非法(月)", "110105" + "19991331" + "002X"},
		{"校验位错误", "110105194912310021"},
		{"未来日期", "11010520991231002X"},
	}
	for _, c := range cases {
		if _, err := ValidateIDCard(c.id); err == nil {
			t.Errorf("%s: 应被拒绝 (%s)", c.name, c.id)
		}
	}
}

func mustDate(s string) (t timeType) {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return d
}

func TestMaskIDCard(t *testing.T) {
	m := MaskIDCard("11010519491231002X")
	if m != "110***********002X" {
		t.Errorf("脱敏格式错误: %s", m)
	}
	if !strings.Contains(m, "*") || strings.Contains(m, "19491231") {
		t.Errorf("脱敏不应泄露出生日期: %s", m)
	}
}

func TestAgeAt(t *testing.T) {
	// 1990-06-15 出生
	cases := []struct {
		when  string
		want  int
	}{
		{"2020-06-14", 29}, // 生日前一天
		{"2020-06-15", 30}, // 生日当天
		{"2020-06-16", 30},
		{"2021-01-01", 30},
	}
	for _, c := range cases {
		birth := mustDate("1990-06-15")
		when := mustDate(c.when)
		if got := AgeAt(birth, when); got != c.want {
			t.Errorf("AgeAt(%s) = %d, want %d", c.when, got, c.want)
		}
	}
}
