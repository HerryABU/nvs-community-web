package honeypot

// ==================== 验证页（M3：恒失败 + 指纹前置采集） ====================
// 设计约束（docs/honeypot-design.md §1）：
//   1. 验证永不放行：没有 success 分支，也不落可重放的凭据；
//   2. 指纹在 DOMContentLoaded 即采，不等提交；
//   3. 采集脚本与页面混编，不出现独立的 fingerprint.js；
//   4. 只在标记/记录层面做事，不做「检测到 headless 就拒绝」这种暴露蜜罐的行为。

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// RegisterVerify 注册验证页与采集接口
func RegisterVerify(r *gin.Engine) {
	r.GET("/hp/verify", verifyPage)
	r.POST("/hp/fp", collectFingerprint)
	r.POST("/hp/verify", verifySubmit)
}

// verifyPage 风控验证页（外观仿主站，采集脚本内联）
func verifyPage(c *gin.Context) {
	set := Current()
	if !set.Enabled || set.Level == LevelOff {
		c.String(http.StatusNotFound, "页面不存在")
		return
	}
	next := c.Query("next")
	if next == "" {
		next = "/admin"
	}
	// 记录「到达验证页」——这一步本身就说明前面已触发卡口
	recordShadow(c, EventVerifyPage, "verify_page", 70, "到达验证页 next="+truncate(next, 120))

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("X-Robots-Tag", "noindex")
	c.String(http.StatusOK, verifyHTML(next))
}

// collectFingerprint 指纹上报（恒返回风控失败，不给任何「通过」语义）
func collectFingerprint(c *gin.Context) {
	set := Current()
	if !set.Enabled || set.Level == LevelOff {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "reason": "not_found"})
		return
	}
	var in FingerprintInput
	_ = c.ShouldBindJSON(&in)

	ip := c.ClientIP()
	fields := BuildLogFields(set, c)
	now := time.Now()
	sess, err := ensureSession(ip, fields.UA, fields.Referer, fields.XFF, now)
	if err == nil {
		if in.Headless == "" {
			in.Headless = detectHeadless(c)
		}
		_ = SaveFingerprint(sess.ID, in)
		_ = updateSessionStats(sess, 55, now)
		ev := &Event{
			SessionID: sess.ID, Kind: EventVerifyPage, Decoy: "fingerprint",
			Method: "POST", Path: "/hp/fp",
			Query: fields.Query, UA: fields.UA, Referer: fields.Referer, XFF: fields.XFF,
			BodySnippet: truncate("指纹上报 canvas="+in.Canvas+" webgl="+in.WebGL+" headless="+in.Headless, 600),
			Risk:        55, Action: "record", CreatedAt: now,
		}
		_ = db().Create(ev)
	}
	// 恒失败：与真实风控失败一致
	c.JSON(http.StatusOK, gin.H{"ok": false, "reason": "risk_control"})
}

// verifySubmit 验证提交：**没有任何 success 分支**
func verifySubmit(c *gin.Context) {
	set := Current()
	if !set.Enabled || set.Level == LevelOff {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "reason": "not_found"})
		return
	}
	ip := c.ClientIP()
	if IsBanned(ip) {
		c.JSON(http.StatusTooManyRequests, gin.H{"ok": false, "reason": "too_many_requests"})
		return
	}
	fields := BuildLogFields(set, c)
	now := time.Now()
	if sess, err := ensureSession(ip, fields.UA, fields.Referer, fields.XFF, now); err == nil {
		ev := &Event{
			SessionID: sess.ID, Kind: EventVerifyTry, Decoy: "verify_submit",
			Method: "POST", Path: "/hp/verify",
			Query: fields.Query, UA: fields.UA, Referer: fields.Referer, XFF: fields.XFF,
			BodySnippet: truncate("验证提交（恒失败）；body="+fields.BodySnippet, 600),
			Risk:        75, Action: "record", CreatedAt: now,
		}
		_ = db().Create(ev)
		_ = updateSessionStats(sess, 75, now)
	}
	// 每次都「差一点」：给一个看似可重试的失败原因，诱导继续消耗
	c.JSON(http.StatusOK, gin.H{
		"ok":     false,
		"reason": "human_verification_failed",
		"retry":  true,
	})
}

// detectHeadless 服务端可见的自动化痕迹（只标记，不拒绝）
func detectHeadless(c *gin.Context) string {
	ua := strings.ToLower(c.Request.UserAgent())
	flags := make([]string, 0, 4)
	for _, kw := range []string{"headless", "puppeteer", "playwright", "phantomjs", "selenium", "curl", "python", "go-http", "scrapy"} {
		if strings.Contains(ua, kw) {
			flags = append(flags, "ua:"+kw)
		}
	}
	if c.Request.Header.Get("Sec-Fetch-Site") == "" && c.Request.Header.Get("Sec-Fetch-Mode") == "" {
		flags = append(flags, "no_sec_fetch")
	}
	if c.Request.Header.Get("Accept-Language") == "" {
		flags = append(flags, "no_accept_language")
	}
	if c.Request.Header.Get("Referer") == "" {
		flags = append(flags, "no_referer")
	}
	return strings.Join(flags, ",")
}

// verifyHTML 验证页（内联采集，样式与诱饵后台一致）
func verifyHTML(next string) string {
	next = strings.ReplaceAll(next, `"`, "")
	next = strings.ReplaceAll(next, "<", "")
	return `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>安全验证</title>
<style>
body{font-family:system-ui,-apple-system,"Segoe UI",sans-serif;background:#0f172a;color:#e2e8f0;
display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0}
.card{background:#1e293b;border:1px solid #334155;border-radius:14px;padding:28px 32px;width:360px;box-shadow:0 18px 40px rgba(0,0,0,.35)}
h3{margin:0 0 6px;font-size:18px}
p.sub{margin:0 0 18px;color:#94a3b8;font-size:13px;line-height:1.6}
.slider{height:44px;border-radius:8px;background:#0f172a;border:1px solid #334155;display:flex;align-items:center;
padding:0 14px;color:#64748b;font-size:13px;cursor:grab;user-select:none;position:relative;overflow:hidden}
.slider .fill{position:absolute;left:0;top:0;bottom:0;background:rgba(59,130,246,.25);width:0}
.slider span{position:relative}
button{width:100%;margin-top:16px;padding:11px;border:0;border-radius:8px;background:#3b82f6;color:#fff;font-size:15px}
.hint{margin-top:12px;font-size:12px;color:#64748b}
.err{margin-top:10px;font-size:12.5px;color:#f87171;min-height:16px}
</style></head>
<body><div class="card">
<h3>安全验证</h3>
<p class="sub">检测到当前登录环境存在异常，请完成验证后继续操作。</p>
<div class="slider" id="slider"><div class="fill" id="fill"></div><span id="label">按住滑块，拖动到最右侧</span></div>
<button id="submit">验证</button>
<div class="err" id="err"></div>
<div class="hint">若多次验证失败，请稍后重试或联系管理员。</div>
</div>
<script>
(function(){
  var next = "` + next + `";
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
    try{
      var body=JSON.stringify(data);
      if(navigator.sendBeacon){navigator.sendBeacon('/hp/fp',new Blob([body],{type:'application/json'}));}
      else{fetch('/hp/fp',{method:'POST',headers:{'Content-Type':'application/json'},body:body,keepalive:true});}
    }catch(e){}
  }
  if(document.readyState==='loading'){document.addEventListener('DOMContentLoaded',collect);}else{collect();}

  var dragging=false,startX=0;
  var slider=document.getElementById('slider'),fill=document.getElementById('fill'),label=document.getElementById('label');
  slider.addEventListener('mousedown',function(e){dragging=true;startX=e.clientX;});
  document.addEventListener('mouseup',function(){dragging=false;});
  document.addEventListener('mousemove',function(e){
    if(!dragging)return;
    var w=Math.min(Math.max(e.clientX-startX,0),slider.clientWidth-60);
    fill.style.width=w+'px';
    if(w>=slider.clientWidth-70){label.textContent='已拖动，请点击验证';}
    else{label.textContent='继续拖动…';}
  });
  document.getElementById('submit').addEventListener('click',function(){
    var err=document.getElementById('err');
    err.textContent='';
    fetch('/hp/verify',{method:'POST',headers:{'Content-Type':'application/json'},
      body:JSON.stringify({next:next,t:Date.now()})}).then(function(r){return r.json();}).then(function(d){
        err.textContent='验证未通过，请重试（'+(d.reason||'risk_control')+'）';
      }).catch(function(){err.textContent='网络异常，请重试';});
  });
})();
</script></body></html>`
}

// VerifyPageHTMLForTest 供单测断言（页面必须不含任何 success 分支）
func VerifyPageHTMLForTest(next string) string { return verifyHTML(next) }
