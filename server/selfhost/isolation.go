package selfhost

// 空间隔离检测：项目只能使用自己的上游空间，不能借代理指向平台自身或另一个项目的空间。
// 由于平台不执行用户代码，这里在「代理转发边界」做检测与阻断。

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"

	"nvs-server/config"
	"nvs-server/models"
)

type IsolationIssue struct {
	ProjectID uint   `json:"project_id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Upstream  string `json:"upstream"`
	Reason    string `json:"reason"`
	Level     string `json:"level"` // block | warn
}

type upstreamInfo struct {
	host string
	port int
	root string
}

func parseUpstream(raw string) (upstreamInfo, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return upstreamInfo{}, false
	}
	port := 80
	if u.Scheme == "https" {
		port = 443
	}
	if p := u.Port(); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil {
			return upstreamInfo{}, false
		}
		port = n
	}
	root := strings.Trim(u.Path, "/")
	return upstreamInfo{host: strings.ToLower(u.Hostname()), port: port, root: root}, true
}

func localHost(host string) bool {
	if strings.EqualFold(host, "localhost") || host == "::1" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
	}
	return false
}

// CheckIsolation 全站自检。block 级问题会阻止提交审核与路由启用。
func CheckIsolation() []IsolationIssue {
	var projects []Project
	models.DB.Select("id", "name", "slug", "upstream").Where("upstream <> ?", "").Find(&projects)
	selfPort, _ := strconv.Atoi(config.ServerPort)

	type seenKey struct {
		host string
		port int
		root string
	}
	seen := map[seenKey]uint{}
	var routes []Route
	models.DB.Find(&routes)
	routePrefixes := map[string]uint{}
	for _, r := range routes {
		routePrefixes[strings.Trim(strings.TrimPrefix(r.Prefix, "/"), "/")] = r.ProjectID
	}

	issues := make([]IsolationIssue, 0)
	for _, p := range projects {
		info, ok := parseUpstream(p.Upstream)
		if !ok {
			issues = append(issues, IsolationIssue{p.ID, p.Name, p.Slug, p.Upstream, "上游地址无法解析", "block"})
			continue
		}
		if localHost(info.host) {
			issues = append(issues, IsolationIssue{p.ID, p.Name, p.Slug, p.Upstream, "上游指向本机回环地址，可能访问平台自身空间", "block"})
		}
		if selfPort > 0 && info.port == selfPort && localHost(info.host) {
			issues = append(issues, IsolationIssue{p.ID, p.Name, p.Slug, p.Upstream, "上游端口等于平台监听端口", "block"})
		}
		key := seenKey{info.host, info.port, info.root}
		if owner, exists := seen[key]; exists && owner != p.ID {
			// 共享同一后端可能是合法的多前缀别名，降为提醒；真正阻断只看本机/平台端口/他人代理空间。
			issues = append(issues, IsolationIssue{p.ID, p.Name, p.Slug, p.Upstream, "与其他项目使用完全相同的上游空间（如非预期请更换端口或根路径）", "warn"})
		}
		seen[key] = p.ID
		if other, exists := routePrefixes[info.root]; exists && other != p.ID {
			issues = append(issues, IsolationIssue{p.ID, p.Name, p.Slug, p.Upstream, "上游根路径指向另一个项目的代理空间", "block"})
		}
	}
	return issues
}

func isolationBlocks(projectID uint) error {
	for _, issue := range CheckIsolation() {
		if issue.Level == "block" && issue.ProjectID == projectID {
			return errors.New("空间隔离检测未通过：" + issue.Reason)
		}
	}
	return nil
}
