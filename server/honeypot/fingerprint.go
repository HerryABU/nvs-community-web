package honeypot

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// ===== (原 fingerprint.go) =====

// ==================== 指纹采集（M3） ====================
// 验证页 DOMContentLoaded 即采集（不等提交）：攻击者关掉页面也已经拿到数据。
// 采集项与真实风控靠拢但**只做标记不硬拒**（硬拒会立刻暴露这是蜜罐）。
// 合规：仅管理员可读；随事件按留存天数一起清理；不做跨站追踪。

// Fingerprint 浏览器/环境指纹
type Fingerprint struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	SessionID uint   `gorm:"not null;index" json:"session_id"`
	Canvas    string `gorm:"size:64;default:''" json:"canvas"`
	WebGL     string `gorm:"size:64;default:''" json:"webgl"`
	Fonts     string `gorm:"size:64;default:''" json:"fonts"`
	TZ        string `gorm:"size:64;default:''" json:"tz"`
	Lang      string `gorm:"size:64;default:''" json:"lang"`
	Screen    string `gorm:"size:32;default:''" json:"screen"`
	DPR       string `gorm:"size:16;default:''" json:"dpr"`
	Cores     string `gorm:"size:16;default:''" json:"cores"`
	Memory    string `gorm:"size:16;default:''" json:"memory"`
	Hints     string `gorm:"size:256;default:''" json:"client_hints"`
	Headless  string `gorm:"size:256;default:''" json:"headless_flags"`
	// TLS 指纹（反代透传）：缺失记 missing，不伪造；来源记 source
	JA3       string `gorm:"size:64;default:'';index" json:"ja3"`
	JA4       string `gorm:"size:64;default:''" json:"ja4"`
	TLSSource string `gorm:"size:24;default:''" json:"tls_source"`
	TLSVer    string `gorm:"size:16;default:''" json:"tls_version"`
	// WebRTC ICE 候选里的 host 地址（内网 IP / mDNS 名），仅作关联线索
	WebRTCIPs string    `gorm:"size:256;default:''" json:"webrtc_ips"`
	Raw       string    `gorm:"type:text" json:"raw"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

func (Fingerprint) TableName() string { return "honeypot_fingerprints" }

// FingerprintInput 上报载荷（前端 JS 采集）
type FingerprintInput struct {
	Canvas   string `json:"canvas"`
	WebGL    string `json:"webgl"`
	Fonts    string `json:"fonts"`
	TZ       string `json:"tz"`
	Lang     string `json:"lang"`
	Screen   string `json:"screen"`
	DPR      string `json:"dpr"`
	Cores    string `json:"cores"`
	Memory   string `json:"memory"`
	Hints    string `json:"hints"`
	Headless string `json:"headless"`
	Raw      string `json:"raw"`
	WebRTC   string `json:"webrtc"` // 前端 RTCPeerConnection 采集（逗号分隔）
	// TLS 由服务端注入（不信任前端上报）
	JA3       string `json:"-"`
	JA4       string `json:"-"`
	TLSSource string `json:"-"`
	TLSVer    string `json:"-"`
}

// SaveFingerprint 落库（同会话同指纹去重）
func SaveFingerprint(sessionID uint, in FingerprintInput) error {
	var n int64
	db().Model(&Fingerprint{}).
		Where("session_id = ? AND canvas = ? AND webgl = ?", sessionID, in.Canvas, in.WebGL).
		Count(&n)
	if n > 0 {
		return nil
	}
	if len(in.Raw) > 4000 {
		in.Raw = in.Raw[:4000]
	}
	return db().Create(&Fingerprint{
		SessionID: sessionID,
		Canvas:    truncate(in.Canvas, 64), WebGL: truncate(in.WebGL, 64),
		Fonts: truncate(in.Fonts, 64), TZ: truncate(in.TZ, 64), Lang: truncate(in.Lang, 64),
		Screen: truncate(in.Screen, 32), DPR: truncate(in.DPR, 16),
		Cores: truncate(in.Cores, 16), Memory: truncate(in.Memory, 16),
		Hints: truncate(in.Hints, 256), Headless: truncate(in.Headless, 256),
		JA3: truncate(in.JA3, 64), JA4: truncate(in.JA4, 64),
		TLSSource: truncate(in.TLSSource, 24), TLSVer: truncate(in.TLSVer, 16),
		WebRTCIPs: truncate(in.WebRTC, 256),
		Raw:       in.Raw, CreatedAt: time.Now(),
	}).Error
}

// ListFingerprints 会话的指纹
func ListFingerprints(sessionID uint) ([]Fingerprint, error) {
	var list []Fingerprint
	if err := db().Where("session_id = ?", sessionID).
		Order("id DESC").Limit(20).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// fingerprintJSON 生成可导出的指纹摘要（取证包用）
func fingerprintJSON(fp Fingerprint) string {
	b, err := json.Marshal(fp)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// ===== 合并自 fingerprint_script.go =====

// ==================== 指纹采集脚本（内联，混编进页面） ====================
// 设计约束（docs/honeypot-design.md §1）：
//   - 不出现独立的 fingerprint.js，采集脚本与页面混编；
//   - DOMContentLoaded 即采，不等提交（攻击者关掉页面也已拿到数据）；
//   - 只在标记/记录层面做事，不做「检测到 headless 就拒绝」这种暴露蜜罐的行为。
//
// 为什么抽成共用块：
// 原本采集脚本只内联在验证页（/hp/verify）上，只有走到风控卡口的攻击者才有指纹；
// 而「浏览型攻击者」（直接访问 /admin.php、/wp-login.php 等 HTML 诱饵，
// 这是最常见的路径枚举/扫描行为）完全采不到任何浏览器指纹。
// 现把同一段采集代码复用到 HTML 诱饵页，两者上报到同一端点 /hp/fp，
// 指纹按「IP + UA 归一」的会话聚合，天然并入同一个攻击会话。

// fingerprintCollectorJS 采集脚本主体（不含 <script> 标签）。
// 采集项：Canvas / WebGL / Fonts / 时区 / 语言 / 屏幕 / DPR / CPU 核数 /
// 内存 / Client Hints / Headless 痕迹 / WebRTC 内网候选 / UA 原文。
func fingerprintCollectorJS() string {
	return `(function(){
  function fnv(s){var h=0x811c9dc5;for(var i=0;i<s.length;i++){h^=s.charCodeAt(i);h=(h*0x01000193)>>>0;}return h.toString(16);}
  function canvasHash(){
    try{
      var c=document.createElement('canvas');c.width=200;c.height=40;
      var x=c.getContext('2d');x.textBaseline='top';x.font='14px Arial';
      x.fillStyle='#f60';x.fillRect(0,0,100,20);x.fillStyle='#069';x.fillText('nvs\u9a8c\u8bc1',2,2);
      return fnv(c.toDataURL());
    }catch(e){return 'na';}
  }
  function webglHash(){
    try{
      var c=document.createElement('canvas');var g=c.getContext('webgl')||c.getContext('experimental-webgl');
      if(!g) return 'na';
      var d=g.getExtension('WEBGL_debug_renderer_info');
      return fnv(String(g.getParameter(d?d.UNMASKED_RENDERER_WEBGL:g.RENDERER))+'|'+g.getParameter(g.VENDOR));
    }catch(e){return 'na';}
  }
  function fontsHash(){
    try{
      var list=['Arial','Courier New','Georgia','Times New Roman','Verdana','Microsoft YaHei','SimSun','PingFang SC'];
      var s='';for(var i=0;i<list.length;i++){s+=document.fonts&&document.fonts.check?document.fonts.check('12px "'+list[i]+'"')?'1':'0':'0';}
      return fnv(s);
    }catch(e){return 'na';}
  }
  // WebRTC 内网 IP：ICE host 候选里可能带内网地址（现代浏览器多为 mDNS .local）
  var webrtcIPs=[];
  function grabWebRTC(){
    return new Promise(function(resolve){
      try{
        var PC=window.RTCPeerConnection||window.webkitRTCPeerConnection||window.mozRTCPeerConnection;
        if(!PC){return resolve();}
        var pc=new PC({iceServers:[]});
        var done=false;
        function finish(){if(done)return;done=true;try{pc.close();}catch(e){}resolve();}
        pc.onicecandidate=function(e){
          if(!e||!e.candidate){return finish();}
          var m=/([0-9]{1,3}(?:\.[0-9]{1,3}){3}|[a-f0-9]{4}:[a-f0-9:]+|[a-z0-9-]+\.local)/i.exec(e.candidate.candidate||'');
          if(m&&webrtcIPs.indexOf(m[1])<0&&webrtcIPs.length<4){webrtcIPs.push(m[1]);}
        };
        pc.createDataChannel('x');
        pc.createOffer().then(function(o){return pc.setLocalDescription(o);}).catch(function(){finish();});
        setTimeout(finish,1200);
      }catch(e){resolve();}
    });
  }
  function collect(){
    var hd=[];
    if(navigator.webdriver) hd.push('webdriver');
    if(!navigator.languages||navigator.languages.length===0) hd.push('no_languages');
    if(!window.chrome) hd.push('no_chrome_obj');
    if(navigator.plugins&&navigator.plugins.length===0) hd.push('no_plugins');
    var data={
      canvas:canvasHash(),webgl:webglHash(),fonts:fontsHash(),
      tz:(Intl.DateTimeFormat().resolvedOptions().timeZone||''),
      lang:navigator.language||'',
      screen:(screen.width+'x'+screen.height),
      dpr:String(window.devicePixelRatio||1),
      cores:String(navigator.hardwareConcurrency||0),
      memory:String(navigator.deviceMemory||0),
      hints:(navigator.userAgentData&&navigator.userAgentData.platform?navigator.userAgentData.platform:''),
      headless:hd.join(','),
      raw:JSON.stringify({ua:navigator.userAgent,platform:navigator.platform,vendor:navigator.vendor,
        webglVendor:webglHash(),langs:navigator.languages,tzOffset:new Date().getTimezoneOffset()})
    };
    function send(){
      data.webrtc=webrtcIPs.join(',');
      try{
        var body=JSON.stringify(data);
        if(navigator.sendBeacon){navigator.sendBeacon('/hp/fp',new Blob([body],{type:'application/json'}));}
        else{fetch('/hp/fp',{method:'POST',headers:{'Content-Type':'application/json'},body:body,keepalive:true});}
      }catch(e){}
    }
    grabWebRTC().then(send,send);
  }
  if(document.readyState==='loading'){document.addEventListener('DOMContentLoaded',collect);}else{collect();}
})();`
}

// fingerprintScriptTag 采集脚本标签（可直接内联进 HTML）
func fingerprintScriptTag() string {
	return "<script>" + fingerprintCollectorJS() + "</script>"
}

// injectFingerprintScript 把采集脚本注入 HTML（幂等：已注入则不重复）
func injectFingerprintScript(html string) string {
	if html == "" {
		return html
	}
	if strings.Contains(html, "/hp/fp") {
		return html // 页面自带采集脚本，避免重复注入
	}
	tag := fingerprintScriptTag()
	for _, anchor := range []string{"</body>", "</html>"} {
		if i := lastIndexFold(html, anchor); i >= 0 {
			return html[:i] + tag + html[i:]
		}
	}
	return html + tag
}

// lastIndexFold 大小写不敏感地找子串最后出现的位置（按字节比较，避免
// strings.ToLower 改变多字节字符长度导致索引错位）
func lastIndexFold(s, sub string) int {
	idx := -1
	for i := 0; i+len(sub) <= len(s); i++ {
		if strings.EqualFold(s[i:i+len(sub)], sub) {
			idx = i
		}
	}
	return idx
}

// looksLikeHTML 粗判内容是否为 HTML（用于未声明 Content-Type 的自定义诱饵）
func looksLikeHTML(body string) bool {
	low := strings.ToLower(strings.TrimSpace(body))
	if strings.HasPrefix(low, "<!doctype html") || strings.HasPrefix(low, "<html") {
		return true
	}
	return strings.Contains(low, "<body") || strings.Contains(low, "<head")
}

// isHTMLContentType 判断 Content-Type 是否为 HTML
func isHTMLContentType(ct string) bool {
	return strings.Contains(strings.ToLower(ct), "text/html")
}

// withFingerprintInjection 给 HTML 类诱饵内容注入指纹采集脚本。
// 返回副本，不修改注册时共享的 Decoy/Headers map（避免跨请求污染）。
// 非 HTML 内容（text/plain、application/json）不注入——纯文本不执行 JS。
func withFingerprintInjection(d Decoy) Decoy {
	ct := ""
	if d.Headers != nil {
		ct = d.Headers["Content-Type"]
	}
	switch {
	case isHTMLContentType(ct):
		d.Content = injectFingerprintScript(d.Content)
	case ct == "" && looksLikeHTML(d.Content):
		// 自定义诱饵未声明 Content-Type 但内容是 HTML：补类型后再注入
		headers := make(map[string]string, len(d.Headers)+1)
		for k, v := range d.Headers {
			headers[k] = v
		}
		headers["Content-Type"] = "text/html; charset=utf-8"
		d.Headers = headers
		d.Content = injectFingerprintScript(d.Content)
	}
	return d
}

// ===== 合并自 tlsfp.go =====

// ==================== TLS 指纹（JA3 / JA4）与 WebRTC 内网 IP（gap-audit #4） ====================
// 现实约束（必须先说清楚，否则就是骗人）：
//   - JA3/JA4 来自 TLS ClientHello，**只有终止 TLS 的那一层算得出来**。
//     本进程通常跑在反代/网关之后，看到的是明文 HTTP，拿不到 ClientHello。
//   - 所以这里提供两条采集路径，按部署形态选：
//       A. 反代透传（推荐、零成本）：nginx / Caddy / 网关算出 JA3/JA4 后放头
//          `X-JA3-Hash` / `X-JA4`（或 `X-TLS-Fingerprint`），本包直接采信并落库；
//       B. 本进程直接终止 TLS（不走反代）时，`c.Request.TLS` 可用，
//          但 Go 标准库不暴露 ClientHello 原始字节，故只记「TLS 版本 + 套件」，
//          JA3 标记为 `direct-no-ja3`（明确「没算」而不是「算不出来乱写」）。
//   - 反代没配时不静默：落 `ja3_missing`，并在后台给出配置模板，避免「以为在采其实没采」。
//
// WebRTC 内网 IP 走验证页 JS（RTCPeerConnection 收集 ICE candidate 里的 host 候选）：
// Chrome/Safari 现代版本默认 mDNS 混淆（xxx.local），能拿到真内网 IP 的场景在减少，
// 所以这里只做「有就记、没有就空」，不作为判定依据，只作为关联线索。

// TLSInfo 一次请求的 TLS 层信息
type TLSInfo struct {
	JA3       string `json:"ja3"`
	JA4       string `json:"ja4"`
	Version   string `json:"tls_version"`
	Source    string `json:"source"` // proxy_header / direct_tls / missing
	ViaProxy  bool   `json:"via_proxy"`
	TermLocal bool   `json:"terminated_locally"`
}

// tlsHeaderCandidates 反代常用头（按优先级）
var ja3Headers = []string{"X-JA3-Hash", "X-JA3", "X-TLS-JA3", "X-SSL-JA3"}
var ja4Headers = []string{"X-JA4", "X-JA4-Hash", "X-TLS-JA4"}

// CaptureTLS 采集 TLS 指纹（缺失时明确标 missing，不伪造）
func CaptureTLS(c *http.Request) TLSInfo {
	info := TLSInfo{Source: "missing"}
	for _, h := range ja3Headers {
		if v := strings.TrimSpace(c.Header.Get(h)); v != "" {
			info.JA3 = truncate(v, 64)
			info.Source = "proxy_header"
			info.ViaProxy = true
			break
		}
	}
	for _, h := range ja4Headers {
		if v := strings.TrimSpace(c.Header.Get(h)); v != "" {
			info.JA4 = truncate(v, 64)
			info.Source = "proxy_header"
			info.ViaProxy = true
			break
		}
	}
	// 兼容通用头（部分网关只给一个合并指纹）
	if info.JA3 == "" && info.JA4 == "" {
		if v := strings.TrimSpace(c.Header.Get("X-TLS-Fingerprint")); v != "" {
			info.JA3 = truncate(v, 64)
			info.Source = "proxy_header"
			info.ViaProxy = true
		}
	}
	if c.TLS != nil {
		info.TermLocal = true
		info.Version = tlsVersionName(c.TLS.Version)
		if info.Source == "missing" {
			// 本进程终止 TLS 但无法算 JA3：明确记录，不冒充
			info.Source = "direct_no_ja3"
		}
	}
	return info
}

func tlsVersionName(v uint16) string {
	switch v {
	case 0x0304:
		return "TLS1.3"
	case 0x0303:
		return "TLS1.2"
	case 0x0302:
		return "TLS1.1"
	case 0x0301:
		return "TLS1.0"
	}
	return "unknown"
}

// tlsHint 反代配置模板（后台直接展示，省得用户去翻文档）
func tlsHint() map[string]string {
	return map[string]string{
		"nginx": `# 需要 ngx_http_ssl_fingerprint 或 lua-resty-ja3 模块算出指纹后放头
map $ssl_ja3_hash $ja3_out { default $ssl_ja3_hash; "" "missing"; }
add_header X-JA3-Hash $ja3_out always;
proxy_set_header X-JA3-Hash $ja3_out;`,
		"caddy": `# Caddy 需插件（caddy-ja3 / caddy-tlsfingerprint）
header_up X-JA3-Hash {http.request.tls.ja3_hash}`,
		"note": "反代未透传时，蜜罐指纹里 ja3 会记为 missing（不会伪造）。本进程直接终止 TLS 时只记版本，不记 JA3。",
	}
}

// markTLSMissingStat 统计用（轻量：只统计会话里 ja3 缺失比例，供后台提示）
func TLSStats() map[string]interface{} {
	var total, withJA3, withJA4, withWebRTC int64
	db().Model(&Fingerprint{}).Count(&total)
	db().Model(&Fingerprint{}).Where("ja3 <> '' AND ja3 <> 'missing'").Count(&withJA3)
	db().Model(&Fingerprint{}).Where("ja4 <> ''").Count(&withJA4)
	db().Model(&Fingerprint{}).Where("webrtc_ips <> ''").Count(&withWebRTC)
	ratio := 0.0
	if total > 0 {
		ratio = float64(withJA3) / float64(total) * 100
	}
	return map[string]interface{}{
		"fingerprints": total,
		"with_ja3":     withJA3,
		"with_ja4":     withJA4,
		"with_webrtc":  withWebRTC,
		"ja3_ratio":    ratio,
		"hint":         tlsHint(),
		// 采集率低不是 bug，是反代没配；这里明说，避免被当成功能失效
		"diagnosis": diagnoseJA3(total, withJA3),
	}
}

func diagnoseJA3(total, withJA3 int64) string {
	switch {
	case total == 0:
		return "还没有指纹数据（蜜罐未被访问或等级为 off）"
	case withJA3 == 0:
		return "反代未透传 JA3/JA4：请按 hint 配置，或在蜜罐独立进程直接终止 TLS"
	case float64(withJA3)/float64(total) < 0.5:
		return "部分请求缺 JA3：可能有多个入口（只有经反代的那条会带指纹）"
	}
	return "JA3 采集正常"
}
