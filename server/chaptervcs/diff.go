package chaptervcs

// ==================== 行级差异（LCS） ====================
// 供「版本对比 / 冲突并排展示 / 三方合并」共用。中文长段落按行比较足够直观，
// 不做字符级 diff（噪声大、对中文不友好）。

import "strings"

// DiffType 行类型
type DiffType string

const (
	DiffEqual DiffType = "equal"
	DiffAdd   DiffType = "add"
	DiffDel   DiffType = "del"
)

// DiffLine 一行差异（并排展示：add 只有右侧，del 只有左侧）
type DiffLine struct {
	Type    DiffType `json:"type"`
	LeftNo  int      `json:"left_no"`  // 0 = 该侧无对应行
	RightNo int      `json:"right_no"` // 0 = 该侧无对应行
	Text    string   `json:"text"`
}

// DiffResult 差异结果
type DiffResult struct {
	Lines   []DiffLine `json:"lines"`
	Added   int        `json:"added"`
	Removed int        `json:"removed"`
	// Truncated 差异过大时降级为整块替换（避免 DP 内存爆炸）
	Truncated bool `json:"truncated"`
}

// 超过该行数差异（两侧剩余行数）直接降级为整块替换
const maxDiffMatrix = 1200

// SplitLines 统一换行后切行
func SplitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Split(s, "\n")
}

// DiffLines 计算行级差异
func DiffLines(a, b string) DiffResult {
	left := SplitLines(a)
	right := SplitLines(b)
	return diffSlices(left, right)
}

func diffSlices(left, right []string) DiffResult {
	// 1) 去掉公共前后缀，缩小 DP 规模
	start := 0
	for start < len(left) && start < len(right) && left[start] == right[start] {
		start++
	}
	endL, endR := len(left), len(right)
	for endL > start && endR > start && left[endL-1] == right[endR-1] {
		endL--
		endR--
	}
	midL, midR := left[start:endL], right[start:endR]

	res := DiffResult{Lines: make([]DiffLine, 0, len(left)+len(right))}
	// 公共前缀
	for i := 0; i < start; i++ {
		res.Lines = append(res.Lines, DiffLine{Type: DiffEqual, LeftNo: i + 1, RightNo: i + 1, Text: left[i]})
	}
	// 公共后缀（中间部分算完后追加）
	appendSuffix := func() {
		for i := endL; i < len(left); i++ {
			j := endR + (i - endL)
			res.Lines = append(res.Lines, DiffLine{Type: DiffEqual, LeftNo: i + 1, RightNo: j + 1, Text: left[i]})
		}
	}

	if len(midL) == 0 && len(midR) == 0 {
		appendSuffix()
		return res
	}

	// 2) 规模过大 → 整块替换（保留可对比性，避免 O(n*m) 内存）
	if len(midL) > maxDiffMatrix || len(midR) > maxDiffMatrix {
		res.Truncated = true
		for i, l := range midL {
			res.Lines = append(res.Lines, DiffLine{Type: DiffDel, LeftNo: start + i + 1, Text: l})
			res.Removed++
		}
		for i, r := range midR {
			res.Lines = append(res.Lines, DiffLine{Type: DiffAdd, RightNo: start + i + 1, Text: r})
			res.Added++
		}
		appendSuffix()
		return res
	}

	// 3) LCS DP
	lcs := lcsTable(midL, midR)
	i, j := len(midL), len(midR)
	// 逆序收集后翻转
	var mid []DiffLine
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && midL[i-1] == midR[j-1]:
			mid = append(mid, DiffLine{Type: DiffEqual, LeftNo: start + i, RightNo: start + j, Text: midL[i-1]})
			i--
			j--
		case j > 0 && (i == 0 || lcs[i][j-1] >= lcs[i-1][j]):
			mid = append(mid, DiffLine{Type: DiffAdd, RightNo: start + j, Text: midR[j-1]})
			res.Added++
			j--
		default:
			mid = append(mid, DiffLine{Type: DiffDel, LeftNo: start + i, Text: midL[i-1]})
			res.Removed++
			i--
		}
	}
	for k := len(mid) - 1; k >= 0; k-- {
		res.Lines = append(res.Lines, mid[k])
	}
	appendSuffix()
	return res
}

// lcsTable LCS 长度表（行数为 0 的边界安全）
func lcsTable(a, b []string) [][]int {
	n, m := len(a), len(b)
	t := make([][]int, n+1)
	for i := range t {
		t[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if a[i-1] == b[j-1] {
				t[i][j] = t[i-1][j-1] + 1
			} else if t[i-1][j] >= t[i][j-1] {
				t[i][j] = t[i-1][j]
			} else {
				t[i][j] = t[i][j-1]
			}
		}
	}
	return t
}

// DiffStat 仅返回增删行数（列表页用，省流量）
func DiffStat(a, b string) (added, removed int) {
	r := DiffLines(a, b)
	return r.Added, r.Removed
}
