import{E as q}from"./el-alert-w-VaMHUw.js";import{k,b7 as $,g as N,i as K,l as Y,w as X,aT as Q,E as ee,al as te,ap as ae,s as se,_ as le}from"./index-Cfq3ZYNp.js";import{E as oe,a as ne}from"./el-descriptions-Doasv75c.js";import{E as re}from"./el-divider-IQAobFM0.js";import{E as de,a as ie}from"./el-step-CVMcAh4x.js";import{E as pe,a as me}from"./el-tab-pane-Dk-NLAiy.js";import{L as _,ak as ue,d as A,H as ce,f as fe,M as b,O as g,P as s,T as f,B as u,S as be,Y as P,Z as a,a8 as ve,_ as x,Q as L,a5 as d,X as e,r as he,ac as p,J as ye,ah as ge}from"./vendor-echarts-Cclku186.js";import"./vendor-mermaid-C-Lzzts6.js";import"./vendor-graph-up_Vg_9Q.js";import"./index-BYaiasTt.js";const B="timeline",we=N({mode:{type:String,values:["start","alternate","alternate-reverse","end"],default:"start"},reverse:Boolean}),Te=_({name:"ElTimeline",props:we,setup(n,{slots:i}){const w=k("timeline");ce(B,{props:n,slots:i});const o=A(()=>[w.b(),w.is(n.mode)]);return()=>{var I;const T=$(((I=i.default)==null?void 0:I.call(i))??[]);return ue("ul",{class:o.value},n.reverse?T.reverse():T)}}}),Ie=N({timestamp:{type:String,default:""},hideTimestamp:Boolean,center:Boolean,placement:{type:String,values:["top","bottom"],default:"bottom"},type:{type:String,values:["primary","success","warning","danger","info"],default:""},color:{type:String,default:""},size:{type:String,values:["normal","large"],default:"normal"},icon:{type:K},hollow:Boolean});var Se=_({name:"ElTimelineItem",__name:"timeline-item",props:Ie,setup(n){const i=n,{props:w}=fe(B),o=k("timeline-item"),T=A(()=>[o.e("node"),o.em("node",i.size||""),o.em("node",i.type||""),o.is("hollow",i.hollow)]),I=A(()=>[o.b(),{[o.e("center")]:i.center},o.is(w.mode)]);return(S,M)=>(b(),g("li",{class:f(I.value)},[s("div",{class:f(u(o).e("tail"))},null,2),S.$slots.dot?x("v-if",!0):(b(),g("div",{key:0,class:f(T.value),style:be({backgroundColor:n.color})},[n.icon?(b(),P(u(Y),{key:0,class:f(u(o).e("icon"))},{default:a(()=>[(b(),P(ve(n.icon)))]),_:1},8,["class"])):x("v-if",!0)],6)),S.$slots.dot?(b(),g("div",{key:1,class:f(u(o).e("dot"))},[L(S.$slots,"dot")],2)):x("v-if",!0),s("div",{class:f(u(o).e("wrapper"))},[!n.hideTimestamp&&n.placement==="top"?(b(),g("div",{key:0,class:f([u(o).e("timestamp"),u(o).is("top")])},d(n.timestamp),3)):x("v-if",!0),s("div",{class:f(u(o).e("content"))},[L(S.$slots,"default")],2),!n.hideTimestamp&&n.placement==="bottom"?(b(),g("div",{key:1,class:f([u(o).e("timestamp"),u(o).is("bottom")])},d(n.timestamp),3)):x("v-if",!0)],2)],2))}}),G=Se;const xe=X(Te,{TimelineItem:G}),Ee=Q(G),Ce={class:"page-container"},Ae={class:"page-header"},_e=`<!-- 1. 进入「自定义→UI模板管理」→ 新建模板 -->
<!-- 2. 选择「交互模式」，勾选「调用平台小说API」 -->
<!-- 3. 粘贴以下代码 -->

<div id="footer" style="background:#1e1e2e;color:#fff;padding:24px;border-radius:12px;
  font-family:-apple-system,sans-serif;text-align:center;margin:20px 0">
  <h3 id="title">加载中...</h3>
  <p id="meta"></p>
  <button onclick="refresh()" style="background:#667eea;color:#fff;border:none;
    padding:8px 20px;border-radius:20px;cursor:pointer;margin:8px">
    🔄 刷新
  </button>
</div>
<script>
async function refresh() {
  const d = await window.NVS.getNovelData();
  document.getElementById('title').textContent = d.novel.title;
  document.getElementById('meta').textContent =
    d.novel.author + ' · ' + d.novel.total_chapters + '章 · ' +
    (d.novel.total_words/10000).toFixed(1)+'万字';
}
refresh();
<\/script>`,Me=`// GET /api/template/novel/:id 响应
{
  "code": 0,
  "data": {
    "novel": {
      "id": 1, "title": "三体", "author": "刘慈欣",
      "author_id": 2, "category": "硬科幻",
      "summary": "文化大革命如火如荼进行的同时...",
      "total_chapters": 36, "total_words": 205000,
      "view_count": 15800, "status": "published",
      "created_at": "2026-01-15T08:00:00Z",
      "updated_at": "2026-07-10T14:30:00Z"
    },
    "chapters": [
      {"number":"1","title":"疯狂年代","url":"..."},
      {"number":"2","title":"寂静的春天","url":"..."}
    ]
  }
}`,He=`<!DOCTYPE html>
<html><head><meta charset="UTF-8">
<style>
  #panel { background:linear-gradient(135deg,#667eea,#764ba2);
    color:#fff;padding:24px;border-radius:12px;margin:20px 0;
    font-family:-apple-system,sans-serif; }
  #panel h3{margin:0 0 8px} #panel p{margin:4px 0;opacity:.9}
  #panel button{background:rgba(255,255,255,.2);color:#fff;
    border:1px solid rgba(255,255,255,.3);padding:8px 20px;
    border-radius:20px;cursor:pointer;font-size:14px;margin:4px}
  #err{color:#ff6b6b;display:none}
</style></head><body>
<div id="panel">
  <h3 id="title">加载中...</h3>
  <p id="meta"></p>
  <button onclick="refresh()">🔄 刷新</button>
  <button onclick="window.NVS.sendMessage('open-toc',{})">📑 目录</button>
  <p id="err"></p>
</div>
<script>
async function refresh() {
  try {
    const d = await window.NVS.getNovelData();
    if (!d || !d.novel) throw new Error('无数据');
    document.getElementById('title').textContent = d.novel.title;
    document.getElementById('meta').innerHTML =
      '✍️ ' + d.novel.author + ' · 📚 ' + d.novel.total_chapters +
      '章 · 📝 ' + (d.novel.total_words/10000).toFixed(1) +
      '万字 · 👁️ ' + d.novel.view_count;
    document.getElementById('err').style.display='none';
  } catch(e) {
    document.getElementById('err').style.display='block';
    document.getElementById('err').textContent='⚠️ 加载失败: '+e.message;
  }
}
refresh();
<\/script></body></html>`,Pe=`my-app.zip
├── index.html       ← 入口文件（默认，可手动选择其他.html）
├── style.css
├── app.js
├── module.wasm      ← WASM模块（需勾选「允许WASM」）
├── data.json
└── assets/
    ├── logo.png
    └── font.woff2

⚠️ 禁止: .exe .dll .so .sh .bat .ps1 .py .php .asp .jsp`,Le=`// 扩展HTML中使用VFS存储数据
async function saveSettings(theme) {
  await fetch('/api/sandbox-vfs/1/write', {
    method:'POST', headers:{'Content-Type':'application/json'},
    body: JSON.stringify({key:'settings.json', value:JSON.stringify({theme})})
  });
}
async function loadSettings() {
  const r = await fetch('/api/sandbox-vfs/1/read?key=settings.json');
  return (await r.json()).data.value;
}`,ke=`// Rust (lib.rs) → 编译为 WASM
use wasm_bindgen::prelude::*;
#[wasm_bindgen]
pub fn fibonacci(n: u32) -> u32 {
    match n { 0 => 0, 1 => 1, _ => fibonacci(n-1)+fibonacci(n-2) }
}
// 编译: wasm-pack build --target web`,Ne=`// index.html — 加载WASM模块
<script type="module">
import init, { fibonacci } from './pkg/my_wasm.js';
async function run() {
  await init();
  console.log('fib(10)=', fibonacci(10)); // 55
  document.getElementById('result').textContent = fibonacci(40);
}
run();
<\/script>`,Be=`// 验证流程
// 1. 获取章节 → 响应含 content_hash + content_signature
// 2. 本地计算 SHA256(content) → 对比 content_hash
// 3. 用作者公钥验证 HMAC-SHA256(content, signing_key) → 对比 content_signature
// 4. 前端显示验证结果 ✓/✗`,Ge=`// 前端章节完整性验证
async function verifyChapter(novelId, chapterNum) {
  const r = await fetch('/api/novels/'+novelId+'/chapters/'+chapterNum+'/verify');
  const d = await r.json();
  if (d.code!==0) return {ok:false, error:'获取验证数据失败'};

  // 1. 本地计算SHA256
  const content = d.data.content || '';
  const hashBuffer = await crypto.subtle.digest('SHA-256',
    new TextEncoder().encode(content));
  const localHash = Array.from(new Uint8Array(hashBuffer))
    .map(b=>b.toString(16).padStart(2,'0')).join('');

  // 2. 对比哈希
  const hashMatch = localHash === d.data.content_hash;

  // 3. HMAC签名验证（需作者公钥，此处简化展示）
  // const sigVerified = await verifyHMAC(content, d.data.content_signature, publicKey);

  return { ok:hashMatch, hash:localHash, stored:d.data.content_hash, match:hashMatch };
}`,Ve=_({__name:"DocsPage",setup(n){const i=he("quickstart"),w=[{name:"模板HTML",desc:"美化小说阅读页面的HTML片段，可调用平台API获取小说数据，支持按钮等交互控件",upload:"文本粘贴",api:"✅ NVS API"},{name:"扩展HTML",desc:"独立沙盒应用，ZIP包部署。HTML+CSS+JS+WASM完整前端，支持端口代理和虚拟文件系统",upload:"ZIP包",api:"❌ 完全隔离"}],o=[{method:"window.NVS.getNovelData(novelId?)",returns:"Promise&lt;Object&gt;",desc:"获取小说元数据+章节列表。novelId可选，为空则自动从URL参数?novel=提取"},{method:"window.NVS.getCurrentChapter()",returns:"Number",desc:"从iframe URL ?chapter=参数解析当前章节号"},{method:"window.NVS.sendMessage(type, data)",returns:"void",desc:"向父页面发送postMessage，type为消息类型，data为JSON对象"},{method:"window.addEventListener('nvs-message',fn)",returns:"void",desc:"监听父页面通过CustomEvent发来的消息"}],T=[{direction:"模板→父页面",format:'{source:"nvs-frame", type:"...", data:{...}}',desc:"通过window.NVS.sendMessage()发送，父页面监听message事件"},{direction:"父页面→模板",format:'{source:"nvs-parent", ...}',desc:"模板内监听nvs-message CustomEvent接收"}],I=[{level:"strict",sandbox:"allow-scripts allow-same-origin",desc:"纯展示（页脚/版权声明），不可交互"},{level:"interactive",sandbox:"allow-scripts allow-same-origin allow-forms allow-popups",desc:"含按钮/表单/弹窗（导航栏/工具栏）"}],S=[{feature:"fetch() / XMLHttpRequest",support:"✅",note:"connect-src: self http: https: ws: wss: data: blob:"},{feature:"WebSocket",support:"✅",note:"iframe sandbox不限制WebSocket"},{feature:"Server-Sent Events",support:"✅",note:"connect-src未限制EventSource"},{feature:"WebRTC",support:"⚠️",note:"Permissions-Policy禁用camera/microphone"},{feature:"外部脚本",support:"❌",note:"CSP script-src: self 仅同源"},{feature:"localStorage/Cookie",support:"✅",note:"allow-same-origin允许同源存储"}],M=[{method:"POST",path:"/api/sandbox-vfs/:htmlId/write",desc:"写入 {key, value}，单值≤1MB，总量≤10MB/100键"},{method:"GET",path:"/api/sandbox-vfs/:htmlId/read?key=...",desc:"读取键值"},{method:"DELETE",path:"/api/sandbox-vfs/:htmlId/delete?key=...",desc:"删除键"},{method:"GET",path:"/api/sandbox-vfs/:htmlId/list",desc:"列出所有键及大小"}],V=[{header:"Cross-Origin-Embedder-Policy: require-corp",purpose:"启用SharedArrayBuffer（WASM多线程必需）"},{header:"Cross-Origin-Opener-Policy: same-origin",purpose:"隔离浏览器上下文"},{header:"Content-Security-Policy: ... 'wasm-unsafe-eval'",purpose:"允许WebAssembly编译"},{header:"Content-Type: application/wasm",purpose:"正确的WASM MIME类型"}],R=[{layer:"①上传前",type:"primary",title:"内容安全扫描",desc:"23条规则检测提权/Shell注入/木马/挖矿/僵尸网络/数据窃取/钓鱼"},{layer:"②解压中",type:"warning",title:"ZIP炸弹检测",desc:"压缩比≤100:1 · 总大小≤50MB · 单文件≤20MB · 文件数≤500"},{layer:"③解压后",type:"danger",title:"目录锁定",desc:"文件→0444只读 · 目录→0755可写（允许创建数据文件）· 标记.nvs_sandbox_lock"},{layer:"④运行时",type:"danger",title:"写入拦截中间件",desc:"POST/PUT到沙盒路径时检查扩展名，拦截.js/.wasm/.html/.sh等33种脚本扩展名"},{layer:"⑤通信",type:"info",title:"CSP + iframe sandbox",desc:"sandbox属性限制iframe能力，CSP限制资源加载和网络连接"}],z=[{cat:"提权",sev:"CRITICAL",rule:'child_process / require("child_process") / execSync'},{cat:"提权",sev:"CRITICAL",rule:"sudo / su / setuid / chmod 777 / chown root"},{cat:"提权",sev:"CRITICAL",rule:"/etc/passwd / /etc/shadow / C:\\Windows\\System32"},{cat:"提权",sev:"CRITICAL",rule:"insmod / modprobe / LoadLibrary / dlopen"},{cat:"Shell",sev:"CRITICAL",rule:'exec("/bin/...") / cmd.exe / powershell.exe'},{cat:"Shell",sev:"CRITICAL",rule:'nc -e / bash -i >& / python -c "import socket"'},{cat:"Shell",sev:"HIGH",rule:"wget ... | bash / certutil -urlcache"},{cat:"木马",sev:"CRITICAL",rule:'eval(atob(...)) / Function("...")()'},{cat:"混淆",sev:"HIGH",rule:"\\xHH 十六进制转义 ≥10次连续"},{cat:"木马",sev:"HIGH",rule:"隐藏iframe (hidden/0px/opacity:0)"},{cat:"木马",sev:"HIGH",rule:"document.write(atob(...))"},{cat:"木马",sev:"MEDIUM",rule:"外部脚本 .ru/.cn/.top/.xyz/.tk等可疑TLD"},{cat:"僵尸",sev:"CRITICAL",rule:"WebSocket + setInterval 组合通信"},{cat:"僵尸",sev:"HIGH",rule:"定时fetch/XHR心跳轮询"},{cat:"窃取",sev:"CRITICAL",rule:"document.cookie + sendBeacon/fetch"},{cat:"窃取",sev:"HIGH",rule:"localStorage + sendBeacon"},{cat:"窃取",sev:"CRITICAL",rule:"addEventListener keydown + sendBeacon (键盘记录)"},{cat:"窃取",sev:"HIGH",rule:"navigator.clipboard.read + sendBeacon"},{cat:"挖矿",sev:"CRITICAL",rule:"CoinHive / cryptonight / new Miner("},{cat:"钓鱼",sev:"CRITICAL",rule:'<form action="https://..." ><input password>'}],W=[{m:"GET",p:"/api/userframes",a:"🔒",d:"我的模板列表"},{m:"POST",p:"/api/userframes",a:"🔒",d:"创建模板"},{m:"GET",p:"/api/userframes/:id",a:"🔒",d:"模板详情+内容"},{m:"PUT",p:"/api/userframes/:id",a:"🔒",d:"更新模板"},{m:"DELETE",p:"/api/userframes/:id",a:"🔒",d:"删除模板"},{m:"GET",p:"/api/userframes/public",a:"🌐",d:"公开模板列表"},{m:"GET",p:"/api/userframes/:id/preview",a:"🌐",d:"模板沙盒预览(HTML)"},{m:"GET",p:"/api/userhtmls",a:"🔒",d:"我的扩展列表"},{m:"POST",p:"/api/userhtmls/upload",a:"🔒",d:"上传ZIP (multipart)"},{m:"GET",p:"/api/userhtmls/:id",a:"🔒",d:"扩展详情+文件列表"},{m:"PUT",p:"/api/userhtmls/:id",a:"🔒",d:"更新扩展元数据"},{m:"DELETE",p:"/api/userhtmls/:id",a:"🔒",d:"删除扩展"},{m:"GET",p:"/api/userhtmls/:id/preview",a:"🌐",d:"扩展沙盒预览"},{m:"GET",p:"/api/novels/:id/frames",a:"🌐",d:"作品关联模板"},{m:"GET",p:"/api/novels/:id/htmls",a:"🌐",d:"作品关联扩展HTML"},{m:"GET",p:"/api/template/novel/:id",a:"🌐",d:"小说数据API(模板调用)"},{m:"POST",p:"/api/port/allocate",a:"🔒",d:"分配代理端口"},{m:"POST",p:"/api/port/release",a:"🔒",d:"释放端口"},{m:"GET",p:"/api/port/list",a:"🔒",d:"已分配端口列表"},{m:"GET",p:"/api/sandbox/info",a:"🌐",d:"沙盒系统信息"},{m:"ANY",p:"/sandbox/proxy/:uid/:name/*",a:"🌐",d:"命名端口代理"},{m:"POST",p:"/api/sandbox-vfs/:id/write",a:"🌐",d:"VFS写入"},{m:"GET",p:"/api/sandbox-vfs/:id/read",a:"🌐",d:"VFS读取"},{m:"DELETE",p:"/api/sandbox-vfs/:id/delete",a:"🌐",d:"VFS删除"},{m:"GET",p:"/api/sandbox-vfs/:id/list",a:"🌐",d:"VFS列表"}],D=[{m:"GET",p:"/api/novels/:id/chapters/:num/verify",d:"返回章节实时SHA256+HMAC签名验证信息"},{m:"GET",p:"/api/novels/:id/chapters/:num",d:"获取章节内容(含hash_match/signature_verified字段)"}];return(Re,t)=>{const E=q,l=ae,m=te,h=re,v=ie,H=de,r=ee,y=me,C=ne,O=oe,j=Ee,F=xe,Z=se,U=pe;return b(),g("div",Ce,[s("div",Ae,[t[2]||(t[2]=s("h2",null,"📖 NVS 自定义内容开发文档",-1)),t[3]||(t[3]=s("p",{class:"subtitle"},"模板HTML · 扩展HTML · WASM · 安全策略 · 平台API · 用户验证",-1)),e(E,{type:"info",closable:!1,"show-icon":"",style:{"margin-bottom":"16px"}},{default:a(()=>[...t[1]||(t[1]=[p("📖 2026-07 更新：新增展开控件 | 虚拟路径沙盒 | HTML广场 | 端口代理 | 作者模板设置",-1)])]),_:1})]),e(U,{modelValue:i.value,"onUpdate:modelValue":t[0]||(t[0]=c=>i.value=c),type:"border-card"},{default:a(()=>[e(y,{label:"🚀 快速开始",name:"quickstart"},{default:a(()=>[e(r,{class:"doc-card"},{header:a(()=>[...t[4]||(t[4]=[s("h3",null,"概念区分",-1)])]),default:a(()=>[e(m,{data:w,border:"",size:"small"},{default:a(()=>[e(l,{prop:"name",label:"",width:"100"}),e(l,{prop:"desc",label:"说明"}),e(l,{prop:"upload",label:"上传方式",width:"100"}),e(l,{prop:"api",label:"平台API",width:"100"})]),_:1}),e(h),t[5]||(t[5]=s("h4",null,"最小示例：创建一个小说页脚模板",-1)),e(H,{active:4,"align-center":"","finish-status":"success",style:{margin:"16px 0"}},{default:a(()=>[e(v,{title:"登录",description:"进入平台"}),e(v,{title:"自定义→模板",description:"新建模板"}),e(v,{title:"编写HTML",description:"粘贴代码"}),e(v,{title:"关联作品",description:"选择小说"})]),_:1}),s("pre",{class:"code"},d(_e)),e(h),t[6]||(t[6]=s("h4",null,"📖 展开/收起控件",-1)),t[7]||(t[7]=s("p",null,[p("在编辑器中使用 "),s("code",null,'[expand title="标题"]...内容...[/expand]'),p(" 创建可折叠内容块：")],-1)),t[8]||(t[8]=s("pre",{class:"code"},`[expand title="点击查看详情"]
这里是被折叠的内容，支持 **Markdown**、公式 $E=mc^2$、代码块等。
[/expand]

[expand title="参考资料"]
1. 参考链接一
2. 参考链接二
[/expand]`,-1)),t[9]||(t[9]=s("p",null,"渲染后显示为可点击展开的 <details> 元素，读者点击「📋 标题」展开查看。",-1))]),_:1})]),_:1}),e(y,{label:"🎨 模板HTML",name:"template"},{default:a(()=>[e(r,{class:"doc-card"},{header:a(()=>[...t[10]||(t[10]=[s("h3",null,"平台 JavaScript API",-1)])]),default:a(()=>[t[11]||(t[11]=s("p",null,[p("模板启用「调用平台API」后，系统自动注入 "),s("code",null,"window.NVS"),p(" 命名空间。")],-1)),e(m,{data:o,border:"",size:"small",style:{"margin-top":"12px"}},{default:a(()=>[e(l,{prop:"method",label:"方法",width:"280"}),e(l,{prop:"returns",label:"返回值",width:"120"}),e(l,{prop:"desc",label:"说明"})]),_:1})]),_:1}),e(r,{class:"doc-card"},{header:a(()=>[...t[12]||(t[12]=[s("h3",null,"getNovelData 响应格式",-1)])]),default:a(()=>[s("pre",{class:"code"},d(Me))]),_:1}),e(r,{class:"doc-card"},{header:a(()=>[...t[13]||(t[13]=[s("h3",null,"完整模板示例",-1)])]),default:a(()=>[s("pre",{class:"code"},d(He))]),_:1}),e(r,{class:"doc-card"},{header:a(()=>[...t[14]||(t[14]=[s("h3",null,"postMessage 通信协议",-1)])]),default:a(()=>[e(m,{data:T,border:"",size:"small"},{default:a(()=>[e(l,{prop:"direction",label:"方向",width:"100"}),e(l,{prop:"format",label:"消息格式"}),e(l,{prop:"desc",label:"说明"})]),_:1})]),_:1}),e(r,{class:"doc-card"},{header:a(()=>[...t[15]||(t[15]=[s("h3",null,"沙盒级别",-1)])]),default:a(()=>[e(m,{data:I,border:"",size:"small"},{default:a(()=>[e(l,{prop:"level",label:"级别",width:"120"}),e(l,{prop:"sandbox",label:"sandbox属性"}),e(l,{prop:"desc",label:"适用场景"})]),_:1})]),_:1})]),_:1}),e(y,{label:"🔌 扩展HTML",name:"extension"},{default:a(()=>[e(r,{class:"doc-card"},{header:a(()=>[...t[16]||(t[16]=[s("h3",null,"ZIP包规范",-1)])]),default:a(()=>[s("pre",{class:"code"},d(Pe)),e(E,{type:"warning",closable:!1,"show-icon":"",style:{"margin-top":"12px"}},{title:a(()=>[...t[17]||(t[17]=[p("安全限制",-1)])]),default:a(()=>[t[18]||(t[18]=p(" ≤20MB压缩 · ≤50MB解压 · ≤100:1压缩比 · ≤500文件 · 白名单扩展名 · 内容安全扫描 ",-1))]),_:1})]),_:1}),e(r,{class:"doc-card"},{header:a(()=>[...t[19]||(t[19]=[s("h3",null,"扩展HTML网络能力",-1)])]),default:a(()=>[e(m,{data:S,border:"",size:"small"},{default:a(()=>[e(l,{prop:"feature",label:"API",width:"200"}),e(l,{prop:"support",label:"支持",width:"80"}),e(l,{prop:"note",label:"备注"})]),_:1})]),_:1}),e(r,{class:"doc-card"},{header:a(()=>[...t[20]||(t[20]=[s("h3",null,"端口代理",-1)])]),default:a(()=>[e(H,{active:4,"align-center":"","finish-status":"success",style:{margin:"16px 0"}},{default:a(()=>[e(v,{title:"上传ZIP"}),e(v,{title:"命名项目",description:"字母/数字/下划线"}),e(v,{title:"启动服务",description:"在分配端口启动HTTP"}),e(v,{title:"代理访问",description:"/sandbox/proxy/{uid}/{项目名}/"})]),_:1}),e(O,{column:2,border:"",size:"small"},{default:a(()=>[e(C,{label:"端口范围"},{default:a(()=>[...t[21]||(t[21]=[p("49152-65535（高位优先）",-1)])]),_:1}),e(C,{label:"项目名规则"},{default:a(()=>[...t[22]||(t[22]=[p("[a-zA-Z0-9_-]{2,32}",-1)])]),_:1}),e(C,{label:"代理路径"},{default:a(()=>[...t[23]||(t[23]=[p("/sandbox/proxy/{uid}/{name}/",-1)])]),_:1})]),_:1})]),_:1}),e(r,{class:"doc-card"},{header:a(()=>[...t[24]||(t[24]=[s("h3",null,"虚拟文件系统 (VFS)",-1)])]),default:a(()=>[t[25]||(t[25]=s("p",null,"扩展应用可通过API在内存中读写数据，完全隔离于真实文件系统。",-1)),e(m,{data:M,border:"",size:"small",style:{"margin-top":"8px"}},{default:a(()=>[e(l,{prop:"method",label:"方法",width:"80"}),e(l,{prop:"path",label:"端点",width:"260"}),e(l,{prop:"desc",label:"说明"})]),_:1}),s("pre",{class:"code",style:{"margin-top":"12px"}},d(Le))]),_:1})]),_:1}),e(y,{label:"⚙️ WASM",name:"wasm"},{default:a(()=>[e(r,{class:"doc-card"},{header:a(()=>[...t[26]||(t[26]=[s("h3",null,"部署 WASM 应用",-1)])]),default:a(()=>[t[27]||(t[27]=s("p",null,"WASM 作为扩展HTML ZIP包的一部分上传。系统自动设置必要HTTP头。",-1)),e(h),t[28]||(t[28]=s("h4",null,"Rust → WASM 示例",-1)),s("pre",{class:"code"},d(ke)),e(h),t[29]||(t[29]=s("h4",null,"JavaScript 胶水代码",-1)),s("pre",{class:"code"},d(Ne)),e(h),t[30]||(t[30]=s("h4",null,"系统自动设置的HTTP头",-1)),e(m,{data:V,border:"",size:"small"},{default:a(()=>[e(l,{prop:"header",label:"Header",width:"350"}),e(l,{prop:"purpose",label:"作用"})]),_:1})]),_:1})]),_:1}),e(y,{label:"🛡️ 安全",name:"security"},{default:a(()=>[e(r,{class:"doc-card"},{header:a(()=>[...t[31]||(t[31]=[s("h3",null,"多层安全防线",-1)])]),default:a(()=>[e(F,null,{default:a(()=>[(b(),g(ye,null,ge(R,(c,J)=>e(j,{key:J,timestamp:c.layer,type:c.type,placement:"top"},{default:a(()=>[s("h4",null,d(c.title),1),s("p",null,d(c.desc),1)]),_:2},1032,["timestamp","type"])),64))]),_:1})]),_:1}),e(r,{class:"doc-card"},{header:a(()=>[...t[32]||(t[32]=[s("h3",null,"内容扫描规则（23条）",-1)])]),default:a(()=>[e(m,{data:z,border:"",size:"small","max-height":"400"},{default:a(()=>[e(l,{prop:"cat",label:"类别",width:"70"}),e(l,{prop:"sev",label:"级别",width:"80"},{default:a(({row:c})=>[e(Z,{type:c.sev==="CRITICAL"?"danger":"warning",size:"small"},{default:a(()=>[p(d(c.sev),1)]),_:2},1032,["type"])]),_:1}),e(l,{prop:"rule",label:"检测规则","min-width":"220"})]),_:1})]),_:1})]),_:1}),e(y,{label:"📡 API参考",name:"api"},{default:a(()=>[e(r,{class:"doc-card"},{header:a(()=>[...t[33]||(t[33]=[s("h3",null,"全部端点",-1)])]),default:a(()=>[e(m,{data:W,border:"",size:"small","max-height":"500"},{default:a(()=>[e(l,{prop:"m",label:"",width:"50"}),e(l,{prop:"p",label:"端点",width:"260"}),e(l,{prop:"a",label:"",width:"30"}),e(l,{prop:"d",label:"说明","min-width":"200"})]),_:1}),e(E,{type:"info",closable:!1,"show-icon":"",style:{"margin-top":"8px"}},{default:a(()=>[...t[34]||(t[34]=[p("🔒=需JWT认证 · 🌐=公开",-1)])]),_:1})]),_:1})]),_:1}),e(y,{label:"✅ 用户验证",name:"verify"},{default:a(()=>[e(r,{class:"doc-card"},{header:a(()=>[...t[35]||(t[35]=[s("h3",null,"章节完整性验证",-1)])]),default:a(()=>[t[36]||(t[36]=s("p",null,"每章自动计算 SHA256 + HMAC-SHA256 签名，读者可验证内容未被篡改。",-1)),e(m,{data:D,border:"",size:"small",style:{"margin-top":"8px"}},{default:a(()=>[e(l,{prop:"m",label:"",width:"60"}),e(l,{prop:"p",label:"端点",width:"300"}),e(l,{prop:"d",label:"说明"})]),_:1}),e(h),t[37]||(t[37]=s("h4",null,"验证流程",-1)),s("pre",{class:"code"},d(Be)),e(h),t[38]||(t[38]=s("h4",null,"前端验证示例",-1)),s("pre",{class:"code"},d(Ge))]),_:1})]),_:1})]),_:1},8,["modelValue"])])}}}),$e=le(Ve,[["__scopeId","data-v-fe2f14da"]]);export{$e as default};
