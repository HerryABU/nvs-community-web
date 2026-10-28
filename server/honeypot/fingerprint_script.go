package honeypot

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

import "strings"

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
