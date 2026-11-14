// Package logarchive 日志分层归档（源目录按类型 log/<type>/<date>/ 分类）：
//
//	① 每天：所有类型的当日日志打在一起，1 个 7z（包内按 类型/日期 组织）；
//	② 每周后：把该周的日 7z 打包为 1 个 tar.gz；
//	③ 每月后：把该月的周 tar.gz 打包为 1 个 tar.gz；
//	④ 每年后：把全年的月 tar.gz 打包为 1 个 tar.xz。
//
// 即归档粒度是「日期」，不再按类型独立成包：请求日志与数据库日志同日同包。
// 幂等：产物比源新则跳过。7z / tar / xz 缺失时对应层级降级跳过。
package logarchive

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Options struct {
	LogDir     string // 默认 "log"
	ArchiveDir string // 默认 "log/archive"
	SevenZip   string // 7z 可执行路径；空则自动探测
}

func (o Options) normalize() Options {
	if strings.TrimSpace(o.LogDir) == "" {
		o.LogDir = "log"
	}
	if strings.TrimSpace(o.ArchiveDir) == "" {
		o.ArchiveDir = filepath.Join(o.LogDir, "archive")
	}
	if strings.TrimSpace(o.SevenZip) == "" {
		o.SevenZip = find7z()
	}
	return o
}

func find7z() string {
	for _, p := range []string{"7z", "7za", `C:\Program Files\7-Zip\7z.exe`, `C:\Program Files (x86)\7-Zip\7z.exe`} {
		if lp, err := exec.LookPath(p); err == nil {
			return lp
		}
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// Run 执行分层归档，返回本次产物列表。单个失败不中断整体，错误汇总返回。
func Run(opt Options) ([]string, error) {
	opt = opt.normalize()
	var outputs []string
	var errs []string
	addErr := func(format string, a ...interface{}) { errs = append(errs, fmt.Sprintf(format, a...)) }

	// 收集所有「日期 → 当日有日志的类型目录」
	dateTypes, dates, err := collectDates(opt.LogDir)
	if err != nil {
		return outputs, err
	}
	if len(dates) == 0 {
		return outputs, nil
	}

	// ① 日 7z：同一天的所有类型打在一个包里
	dailyOut := map[string]string{}
	for _, d := range dates {
		if opt.SevenZip == "" {
			break
		}
		srcDirs := make([]string, 0, len(dateTypes[d]))
		for _, typ := range dateTypes[d] {
			srcDirs = append(srcDirs, filepath.Join(opt.LogDir, typ, d))
		}
		out := filepath.Join(opt.ArchiveDir, "daily", d+".7z")
		built := false
		if staleAny(srcDirs, out) {
			if err := pack7zDirs(opt.SevenZip, out, srcDirs); err != nil {
				addErr("daily %s: %v", out, err)
			} else {
				built = true
			}
		}
		if fileExists(out) {
			dailyOut[d] = out
			if built {
				outputs = append(outputs, out)
			}
		}
	}

	// ② 周 tar.gz（内容 = 该周各日的 7z）
	weeklyOut := map[string]string{}
	for _, w := range groupWeek(dates) {
		srcs := pickExisting(dailyOut, w.dates)
		if len(srcs) == 0 {
			continue
		}
		out := filepath.Join(opt.ArchiveDir, "weekly", w.key+".tar.gz")
		built := false
		if staleAny(srcs, out) {
			if err := packFilesTarGz(out, srcs); err != nil {
				addErr("weekly %s: %v", out, err)
			} else {
				built = true
			}
		}
		if fileExists(out) {
			weeklyOut[w.key] = out
			if built {
				outputs = append(outputs, out)
			}
		}
	}

	// ③ 月 tar.gz（内容 = 该月各周的 tar.gz）
	monthlyOut := map[string]string{}
	for _, m := range groupMonth(dates) {
		var srcs []string
		for _, wk := range weeklyOut {
			if weekInMonth(filepath.Base(wk), m.key) {
				srcs = append(srcs, wk)
			}
		}
		if len(srcs) == 0 {
			continue
		}
		out := filepath.Join(opt.ArchiveDir, "monthly", m.key+".tar.gz")
		built := false
		if staleAny(srcs, out) {
			if err := packFilesTarGz(out, srcs); err != nil {
				addErr("monthly %s: %v", out, err)
			} else {
				built = true
			}
		}
		if fileExists(out) {
			monthlyOut[m.key] = out
			if built {
				outputs = append(outputs, out)
			}
		}
	}

	// ④ 年 tar.xz（内容 = 全年各月的 tar.gz）
	for _, y := range groupYear(dates) {
		var srcs []string
		for _, mk := range monthlyOut {
			if strings.HasPrefix(filepath.Base(mk), y.key+"-") {
				srcs = append(srcs, mk)
			}
		}
		if len(srcs) == 0 {
			continue
		}
		out := filepath.Join(opt.ArchiveDir, "yearly", y.key+".tar.xz")
		built := false
		if staleAny(srcs, out) {
			if err := packFilesTarXz(out, srcs); err != nil {
				addErr("yearly %s: %v", out, err)
			} else {
				built = true
			}
		}
		if built {
			outputs = append(outputs, out)
		}
	}

	if len(errs) > 0 {
		return outputs, fmt.Errorf("%d 个归档失败: %s", len(errs), strings.Join(errs, "; "))
	}
	return outputs, nil
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func pickExisting(m map[string]string, keys []string) []string {
	var out []string
	for _, k := range keys {
		if v, ok := m[k]; ok && fileExists(v) {
			out = append(out, v)
		}
	}
	return out
}

// collectDates 返回 日期 → 当日有日志的类型列表，以及排序后的日期列表
func collectDates(logDir string) (map[string][]string, []string, error) {
	types, err := listTypes(logDir)
	if err != nil {
		return nil, nil, err
	}
	dateTypes := map[string][]string{}
	for _, typ := range types {
		dates, err := listDates(filepath.Join(logDir, typ))
		if err != nil {
			continue
		}
		for _, d := range dates {
			dateTypes[d] = append(dateTypes[d], typ)
		}
	}
	dates := make([]string, 0, len(dateTypes))
	for d := range dateTypes {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	return dateTypes, dates, nil
}

// weekRe 从归档名（如 2026W45.tar.gz）提取年/周
var weekRe = regexp.MustCompile(`(\d{4})W(\d{1,2})`)

// weekInMonth 判断周归档的归属月（按该周周一所在月）
func weekInMonth(base, monthKey string) bool {
	m := weekRe.FindStringSubmatch(base)
	if len(m) != 3 {
		return false
	}
	t, err := weekStart(m[1], m[2])
	if err != nil {
		return false
	}
	return t.Format("2006-01") == monthKey
}

func weekStart(year, week string) (time.Time, error) {
	y, err := parseInt(year)
	if err != nil {
		return time.Time{}, err
	}
	w, err := parseInt(week)
	if err != nil {
		return time.Time{}, err
	}
	jan4 := time.Date(y, 1, 4, 0, 0, 0, 0, time.Local)
	wd := int(jan4.Weekday())
	if wd == 0 {
		wd = 7
	}
	monday := jan4.AddDate(0, 0, -(wd - 1))
	return monday.AddDate(0, 0, (w-1)*7), nil
}

func parseInt(s string) (int, error) {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("bad number")
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}

func listTypes(logDir string) ([]string, error) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && e.Name() != "archive" {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

func listDates(typeDir string) ([]string, error) {
	entries, err := os.ReadDir(typeDir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			if _, err := time.Parse("2006-01-02", e.Name()); err == nil {
				out = append(out, e.Name())
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

type weekGroup struct {
	key   string
	dates []string
}

func groupWeek(dates []string) []weekGroup {
	m := map[string][]string{}
	var order []string
	for _, d := range dates {
		t, err := time.Parse("2006-01-02", d)
		if err != nil {
			continue
		}
		y, w := t.ISOWeek()
		key := fmt.Sprintf("%dW%02d", y, w)
		if _, ok := m[key]; !ok {
			order = append(order, key)
		}
		m[key] = append(m[key], d)
	}
	sort.Strings(order)
	out := make([]weekGroup, 0, len(order))
	for _, k := range order {
		out = append(out, weekGroup{key: k, dates: m[k]})
	}
	return out
}

type monthGroup struct {
	key   string
	dates []string
}

func groupMonth(dates []string) []monthGroup {
	m := map[string][]string{}
	var order []string
	for _, d := range dates {
		key := d[:7]
		if _, ok := m[key]; !ok {
			order = append(order, key)
		}
		m[key] = append(m[key], d)
	}
	sort.Strings(order)
	out := make([]monthGroup, 0, len(order))
	for _, k := range order {
		out = append(out, monthGroup{key: k, dates: m[k]})
	}
	return out
}

type yearGroup struct {
	key   string
	dates []string
}

func groupYear(dates []string) []yearGroup {
	m := map[string][]string{}
	var order []string
	for _, d := range dates {
		key := d[:4]
		if _, ok := m[key]; !ok {
			order = append(order, key)
		}
		m[key] = append(m[key], d)
	}
	sort.Strings(order)
	out := make([]yearGroup, 0, len(order))
	for _, k := range order {
		out = append(out, yearGroup{key: k, dates: m[k]})
	}
	return out
}

// staleAny 任一源目录比产物新 → 需要重建
func staleAny(srcDirs []string, out string) bool {
	oi, err := os.Stat(out)
	if err != nil {
		return true
	}
	for _, d := range srcDirs {
		if si, err := os.Stat(d); err == nil && si.ModTime().After(oi.ModTime()) {
			return true
		}
	}
	return false
}

// pack7zDirs 把多个「<type>/<date>」目录打进同一个 7z：
// 包内结构为 <type>/<date>/...（类型/日期 两级分类）。
func pack7zDirs(sevenZip, out string, srcDirs []string) error {
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "nvs-log7z-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	for _, src := range srcDirs {
		typ := filepath.Base(filepath.Dir(src)) // 类型 = 父目录名
		dst := filepath.Join(tmp, typ, filepath.Base(src))
		if err := copyTree(src, dst); err != nil {
			return err
		}
	}
	absOut, err := filepath.Abs(out)
	if err != nil {
		absOut = out
	}
	cmd := exec.Command(sevenZip, "a", "-t7z", "-mx=5", "-y", absOut, ".")
	cmd.Dir = tmp
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("7z: %v: %s", err, string(b))
	}
	return nil
}

// packFilesTarGz 把若干「上一级归档文件」打包为 tar.gz（保留文件名，含日期）。
func packFilesTarGz(out string, files []string) error {
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return err
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			continue
		}
		hdr.Name = filepath.Base(file)
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		in, err := os.Open(file)
		if err != nil {
			return err
		}
		_, _ = io.Copy(tw, in)
		in.Close()
	}
	return nil
}

// packFilesTarXz 把若干「上一级归档文件」打包为 tar.xz（tar -cJf，依赖系统 tar/xz）。
func packFilesTarXz(out string, files []string) error {
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return err
	}
	absOut, err := filepath.Abs(out)
	if err != nil {
		absOut = out
	}
	tmp, err := os.MkdirTemp("", "nvs-logxz-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	var names []string
	for _, f := range files {
		dst := filepath.Join(tmp, filepath.Base(f))
		if err := copyFile(f, dst); err != nil {
			return err
		}
		names = append(names, filepath.Base(f))
	}
	args := append([]string{"-cJf", toPosixPath(absOut), "-C", toPosixPath(tmp)}, names...)
	cmd := exec.Command("tar", args...)
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tar.xz: %v: %s", err, string(b))
	}
	return nil
}

// toPosixPath 把 Windows 路径转成 MSYS/GNU tar 可识别的 POSIX 形式（C:\x → /c/x），
// 否则 tar 会把 "C:" 当作远程主机名。
func toPosixPath(p string) string {
	if len(p) >= 2 && p[1] == ':' {
		return "/" + strings.ToLower(p[:1]) + strings.ReplaceAll(p[2:], "\\", "/")
	}
	return strings.ReplaceAll(p, "\\", "/")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	outFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer outFile.Close()
	_, err = io.Copy(outFile, in)
	return err
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return copyFile(path, target)
	})
}
