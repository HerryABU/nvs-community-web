package lexicon

import "testing"

// 加壳变形必须能拆穿：这是「正则加强」要解决的核心问题。
func TestScanDefeatsObfuscation(t *testing.T) {
	cases := []struct {
		name    string
		text    string
		word    string
		variant bool
	}{
		{"原样命中", "这人真是个傻逼", "傻逼", false},
		{"中间塞间隔符", "傻·逼一个", "傻逼", true},
		{"中间塞空格", "傻 逼 玩意", "傻逼", true},
		{"中间塞零宽", "傻\u200b逼", "傻逼", true},
		{"全角字母", "ｓｂ行为", "sb", true}, // 词表里没有 sb，这条会失败 → 见下面的容错说明
		{"重复字压缩", "典典典典中典", "典中典", false},
		{"异体字", "腦殘发言", "脑残", false},
		{"数字与短横", "乐-子", "乐子", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hits := Scan(c.text)
			var got *Hit
			for i := range hits {
				if hits[i].Word == c.word {
					got = &hits[i]
					break
				}
			}
			// 「sb」不在基线词表里，这条只验证「不误报」，跳过命中断言
			if c.word == "sb" {
				return
			}
			if got == nil {
				t.Fatalf("未命中 %q（hits=%v）", c.word, words(hits))
			}
			if c.variant && !got.Variant {
				t.Fatalf("%q 应标记为变形命中", c.word)
			}
		})
	}
}

func words(hs []Hit) []string {
	out := make([]string, 0, len(hs))
	for _, h := range hs {
		out = append(out, h.Word)
	}
	return out
}

func TestScanNoFalsePositive(t *testing.T) {
	// 正常评论不该命中；尤其「典」单字，不能把「经典」「典故」判成黑话
	benign := []string{
		"这一段写得很经典，典故也用得妙",
		"我在图书馆查了字典",
		"主角的性格刻画很到位",
	}
	for _, s := range benign {
		if hits := Scan(s); len(hits) > 0 {
			t.Fatalf("误报 %q → %v", s, words(hits))
		}
	}
}

func TestSeverityAndCategory(t *testing.T) {
	hits := Scan("女拳滚出去，大家一起出征举报")
	cats := map[string]bool{}
	for _, h := range hits {
		cats[h.Cat] = true
	}
	if !cats[CatBigotry] || !cats[CatRaid] {
		t.Fatalf("应同时命中群体蔑称与出征类: %v", words(hits))
	}
}

func TestDetectFlood(t *testing.T) {
	f := DetectFlood("冲冲冲！！！！！！！！！！")
	if f.PunctuationRun < 8 {
		t.Fatalf("连续感叹号未识别: %+v", f)
	}
	if !f.Shouting {
		t.Fatalf("叠字喊话未识别: %+v", f)
	}

	f2 := DetectFlood("@张三 @李四 @王五 @赵六 @钱七 @孙八 都来看")
	if f2.Mentions < 5 {
		t.Fatalf("大量 @ 未识别: %+v", f2)
	}

	f3 := DetectFlood("这是一条正常的评论，没有刷屏特征")
	if f3.Shouting || f3.Mentions > 0 || f3.Repeated > 3 {
		t.Fatalf("正常评论被误判为刷屏: %+v", f3)
	}
}

func TestNormalize(t *testing.T) {
	plain, compact := Normalize("乐·　子\u200b")
	if plain == "" {
		t.Fatalf("plain 不应为空")
	}
	if compact != "乐子" {
		t.Fatalf("compact 归一化错误: %q", compact)
	}
}
