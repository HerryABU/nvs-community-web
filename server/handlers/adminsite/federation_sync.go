package adminsite

// 递归联邦同步：沿友站的友站继续拉取，按站点 URL 去重、限制最大跳数防环。
// 保留旧版 /api/novels 单跳同步作为回退（远程站未升级时自动降级）。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"nvs-server/handlers/admincommon"
	"nvs-server/models"
	apikey "nvs-server/models/apikey"
	platform "nvs-server/models/platform"
	"nvs-server/utils/infra"
	"nvs-server/version"
)

const maxFederationHops = 3

// unsafeFedURLForTest 仅供单测关闭 SSRF 字面量校验（httptest 用回环地址）。
// 生产恒为 false，禁止外部修改。
var unsafeFedURLForTest bool

type fedRemote struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

type fedItemRemote struct {
	ID        uint   `json:"id"`
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	Category  string `json:"category"`
	Summary   string `json:"summary"`
	CoverURL  string `json:"cover_url"`
	Author    string `json:"author"`
	SourceURL string `json:"source_url"`
}

// SyncFederatedSiteRecursive 递归同步入口（替换原单跳版本）。
func SyncFederatedSiteRecursive(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var site platform.FederatedSite
	if err := models.DB.First(&site, id).Error; err != nil {
		infra.NotFound(c, "站点不存在")
		return
	}

	visited := map[string]bool{}
	synced, sites, hops := 0, 0, 0
	key := pickOutboundKey(uint(id))
	chain := fmt.Sprintf("%s(%s)", models.GetSiteName(), selfPublicURL(c))
	stat := syncFedNode(c, site.APIURL, key, visited, 1, maxFederationHops, chain, uint(id), &synced)
	if stat.err != nil {
		// 回退：新接口失败时尝试旧版 /api/novels（兼容未升级的友站）
		if n, e := syncLegacyNovels(c, site, key); e == nil {
			synced += n
			sites = 0
		} else {
			infra.InternalError(c, "同步失败："+stat.err.Error())
			return
		}
	} else {
		sites = stat.sites
		hops = stat.maxHop
	}
	now := time.Now()
	count := int64(0)
	models.DB.Model(&platform.FederatedNovel{}).Where("site_id = ?", id).Count(&count)
	models.DB.Model(&site).Updates(map[string]interface{}{"last_sync_at": now, "novel_count": count})
	infra.Success(c, gin.H{"message": "同步完成", "synced": synced, "sites": sites, "max_hop": hops, "total": count})
}

type syncStat struct {
	sites  int
	maxHop int
	err    error
}

func pickOutboundKey(siteID uint) string {
	for _, k := range apikey.ActiveOutboundKeys(siteID) {
		if plain, err := k.DecryptOutboundKey(); err == nil && plain != "" {
			return plain
		}
	}
	return ""
}

func syncFedNode(c *gin.Context, apiBase, key string, visited map[string]bool, hop, maxHop int, chain string, rootSiteID uint, synced *int) syncStat {
	st := syncStat{maxHop: hop}
	if hop > maxHop {
		return st
	}
	base := strings.TrimRight(apiBase, "/")
	norm := strings.ToLower(base)
	if visited[norm] {
		return st
	}
	visited[norm] = true
	query := url.Values{}
	query.Set("page_size", "50")
	query.Set("kind", "")
	query.Set("source_name", models.GetSiteName())
	query.Set("source_url", selfPublicURL(c))
	target := base + "/items?" + query.Encode()
	if !unsafeFedURLForTest && !isSafeFederatedURL(target) {
		st.err = fmt.Errorf("远程站点地址非法")
		return st
	}
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		st.err = err
		return st
	}
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	req.Header.Set("User-Agent", "NVS-Federated/"+version.AppVersion)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		st.err = err
		return st
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		st.err = fmt.Errorf("远程站点返回 %d", resp.StatusCode)
		return st
	}
	var result struct {
		Code int `json:"code"`
		Data struct {
			List  []fedItemRemote `json:"list"`
			Sites []fedRemote     `json:"sites"`
			Site  fedRemote       `json:"site"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		st.err = err
		return st
	}
	if result.Code != 0 {
		st.err = fmt.Errorf("远程站点返回错误")
		return st
	}
	origin := result.Data.Site.Name
	if origin == "" {
		origin = "友站"
	}
	childChain := chain + " → " + origin
	for _, item := range result.Data.List {
		if item.ID == 0 {
			continue
		}
		kind := item.Kind
		if kind == "" {
			kind = "novel"
		}
		siteURL := strings.TrimRight(base, "/api")
		var existing platform.FederatedNovel
		if models.DB.Where("site_id = ? AND remote_id = ? AND kind = ?", rootSiteID, item.ID, kind).First(&existing).Error == nil {
			continue
		}
		source := item.SourceURL
		if source == "" {
			source = siteURL + "/" + kind + "/" + strconv.FormatUint(uint64(item.ID), 10)
		}
		// 本站反代路径：由 proxycore 动态注册 /fed/<siteID> → 源站根。
		proxyPath := "/fed/" + strconv.FormatUint(uint64(rootSiteID), 10)
		if u, e := url.Parse(source); e == nil {
			proxyPath += u.Path
		}
		models.DB.Create(&platform.FederatedNovel{
			SiteID: rootSiteID, RemoteID: item.ID, Kind: kind, Title: item.Title,
			Category: item.Category, Author: item.Author, Summary: item.Summary,
			CoverURL: item.CoverURL, SourceURL: source, ProxyPath: proxyPath,
			OriginSite: origin, SourceChain: childChain, HopCount: hop,
			CachedAt: time.Now(),
		})
		*synced++
	}
	// 递归：继续拉友站的友站（用同一 rootSiteID 归属，来源链记录真实路径）。
	for _, sub := range result.Data.Sites {
		if sub.URL == "" {
			continue
		}
		subAPI := strings.TrimRight(sub.URL, "/") + "/api"
		child := syncFedNode(c, subAPI, "", visited, hop+1, maxHop, childChain, rootSiteID, synced)
		st.sites++
		if child.maxHop > st.maxHop {
			st.maxHop = child.maxHop
		}
	}
	return st
}

// syncLegacyNovels 回退到旧版 /api/novels 单跳同步（兼容未升级的友站）。
func syncLegacyNovels(c *gin.Context, site platform.FederatedSite, key string) (int, error) {
	query := url.Values{}
	query.Set("page_size", "50")
	query.Set("source_name", models.GetSiteName())
	query.Set("source_url", selfPublicURL(c))
	target := strings.TrimRight(site.APIURL, "/") + "/novels?" + query.Encode()
	if !isSafeFederatedURL(target) {
		return 0, fmt.Errorf("远程站点地址非法")
	}
	req, _ := http.NewRequest(http.MethodGet, target, nil)
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	req.Header.Set("User-Agent", "NVS-Federated/"+version.AppVersion)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var result struct {
		Code int `json:"code"`
		Data struct {
			List []struct {
				ID       uint   `json:"id"`
				Title    string `json:"title"`
				Category string `json:"category"`
				Summary  string `json:"summary"`
				CoverURL string `json:"cover_url"`
				Author   struct {
					Nickname string `json:"nickname"`
				} `json:"author"`
			} `json:"list"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}
	if result.Code != 0 {
		return 0, fmt.Errorf("远程站点返回错误")
	}
	n := 0
	for _, item := range result.Data.List {
		if item.ID == 0 {
			continue
		}
		var existing platform.FederatedNovel
		if models.DB.Where("site_id = ? AND remote_id = ? AND kind = ?", site.ID, item.ID, "novel").First(&existing).Error == nil {
			continue
		}
		models.DB.Create(&platform.FederatedNovel{
			SiteID: site.ID, RemoteID: item.ID, Kind: "novel", Title: item.Title,
			Category: item.Category, Author: item.Author.Nickname, Summary: item.Summary,
			CoverURL: item.CoverURL, SourceURL: fmt.Sprintf("%s/novel/%d", site.URL, item.ID),
			ProxyPath:  "/fed/" + strconv.FormatUint(uint64(site.ID), 10) + "/novel/" + strconv.FormatUint(uint64(item.ID), 10),
			OriginSite: site.Name, SourceChain: site.Name, HopCount: 1, CachedAt: time.Now(),
		})
		n++
	}
	return n, nil
}
