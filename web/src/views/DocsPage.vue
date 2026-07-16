<template>
  <div class="page-container">
    <div class="page-header">
      <h2>📖 NVS 自定义内容开发文档</h2>
      <p class="subtitle">模板HTML · 扩展HTML · WASM · 安全策略 · 平台API · 用户验证</p>
      <el-alert type="info" :closable="false" show-icon style="margin-bottom:16px">📖 2026-07 更新：新增展开控件 | 虚拟路径沙盒 | HTML广场 | 端口代理 | 作者模板设置</el-alert>
    </div>

    <el-tabs v-model="activeTab" type="border-card">
      <!-- ═══ 快速开始 ═══ -->
      <el-tab-pane label="🚀 快速开始" name="quickstart">
        <el-card class="doc-card"><template #header><h3>概念区分</h3></template>
          <el-table :data="concepts" border size="small">
            <el-table-column prop="name" label="" width="100" />
            <el-table-column prop="desc" label="说明" />
            <el-table-column prop="upload" label="上传方式" width="100" />
            <el-table-column prop="api" label="平台API" width="100" />
          </el-table>
          <el-divider />
          <h4>最小示例：创建一个小说页脚模板</h4>
          <el-steps :active="4" align-center finish-status="success" style="margin:16px 0">
            <el-step title="登录" description="进入平台" />
            <el-step title="自定义→模板" description="新建模板" />
            <el-step title="编写HTML" description="粘贴代码" />
            <el-step title="关联作品" description="选择小说" />
          </el-steps>
          <pre class="code">{{ quickStartTemplate }}</pre>
          <el-divider />
          <h4>📖 展开/收起控件</h4>
          <p>在编辑器中使用 <code>[expand title="标题"]...内容...[/expand]</code> 创建可折叠内容块：</p>
          <pre class="code">[expand title="点击查看详情"]
这里是被折叠的内容，支持 **Markdown**、公式 $E=mc^2$、代码块等。
[/expand]

[expand title="参考资料"]
1. 参考链接一
2. 参考链接二
[/expand]</pre>
          <p>渲染后显示为可点击展开的 &lt;details&gt; 元素，读者点击「📋 标题」展开查看。</p>
        </el-card>
      </el-tab-pane>

      <!-- ═══ 模板HTML ═══ -->
      <el-tab-pane label="🎨 模板HTML" name="template">
        <el-card class="doc-card"><template #header><h3>平台 JavaScript API</h3></template>
          <p>模板启用「调用平台API」后，系统自动注入 <code>window.NVS</code> 命名空间。</p>
          <el-table :data="templateAPI" border size="small" style="margin-top:12px">
            <el-table-column prop="method" label="方法" width="280" />
            <el-table-column prop="returns" label="返回值" width="120" />
            <el-table-column prop="desc" label="说明" />
          </el-table>
        </el-card>

        <el-card class="doc-card"><template #header><h3>getNovelData 响应格式</h3></template>
          <pre class="code">{{ novelDataResponse }}</pre>
        </el-card>

        <el-card class="doc-card"><template #header><h3>完整模板示例</h3></template>
          <pre class="code">{{ fullTemplate }}</pre>
        </el-card>

        <el-card class="doc-card"><template #header><h3>postMessage 通信协议</h3></template>
          <el-table :data="postMessageProto" border size="small">
            <el-table-column prop="direction" label="方向" width="100" />
            <el-table-column prop="format" label="消息格式" />
            <el-table-column prop="desc" label="说明" />
          </el-table>
        </el-card>

        <el-card class="doc-card"><template #header><h3>沙盒级别</h3></template>
          <el-table :data="sandboxLevels" border size="small">
            <el-table-column prop="level" label="级别" width="120" />
            <el-table-column prop="sandbox" label="sandbox属性" />
            <el-table-column prop="desc" label="适用场景" />
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- ═══ 扩展HTML ═══ -->
      <el-tab-pane label="🔌 扩展HTML" name="extension">
        <el-card class="doc-card"><template #header><h3>ZIP包规范</h3></template>
          <pre class="code">{{ zipSpec }}</pre>
          <el-alert type="warning" :closable="false" show-icon style="margin-top:12px">
            <template #title>安全限制</template>
            ≤20MB压缩 · ≤50MB解压 · ≤100:1压缩比 · ≤500文件 · 白名单扩展名 · 内容安全扫描
          </el-alert>
        </el-card>

        <el-card class="doc-card"><template #header><h3>扩展HTML网络能力</h3></template>
          <el-table :data="netCaps" border size="small">
            <el-table-column prop="feature" label="API" width="200" />
            <el-table-column prop="support" label="支持" width="80" />
            <el-table-column prop="note" label="备注" />
          </el-table>
        </el-card>

        <el-card class="doc-card"><template #header><h3>端口代理</h3></template>
          <el-steps :active="4" align-center finish-status="success" style="margin:16px 0">
            <el-step title="上传ZIP" />
            <el-step title="命名项目" description="字母/数字/下划线" />
            <el-step title="启动服务" description="在分配端口启动HTTP" />
            <el-step title="代理访问" description="/sandbox/proxy/{uid}/{项目名}/" />
          </el-steps>
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="端口范围">49152-65535（高位优先）</el-descriptions-item>
            <el-descriptions-item label="项目名规则">[a-zA-Z0-9_-]{2,32}</el-descriptions-item>
            <el-descriptions-item label="代理路径">/sandbox/proxy/{uid}/{name}/</el-descriptions-item>
          </el-descriptions>
        </el-card>

        <el-card class="doc-card"><template #header><h3>虚拟文件系统 (VFS)</h3></template>
          <p>扩展应用可通过API在内存中读写数据，完全隔离于真实文件系统。</p>
          <el-table :data="vfsAPI" border size="small" style="margin-top:8px">
            <el-table-column prop="method" label="方法" width="80" />
            <el-table-column prop="path" label="端点" width="260" />
            <el-table-column prop="desc" label="说明" />
          </el-table>
          <pre class="code" style="margin-top:12px">{{ vfsExample }}</pre>
        </el-card>
      </el-tab-pane>

      <!-- ═══ WASM ═══ -->
      <el-tab-pane label="⚙️ WASM" name="wasm">
        <el-card class="doc-card"><template #header><h3>部署 WASM 应用</h3></template>
          <p>WASM 作为扩展HTML ZIP包的一部分上传。系统自动设置必要HTTP头。</p>
          <el-divider />
          <h4>Rust → WASM 示例</h4>
          <pre class="code">{{ rustWasmExample }}</pre>
          <el-divider />
          <h4>JavaScript 胶水代码</h4>
          <pre class="code">{{ jsGlueCode }}</pre>
          <el-divider />
          <h4>系统自动设置的HTTP头</h4>
          <el-table :data="wasmHeaders" border size="small">
            <el-table-column prop="header" label="Header" width="350" />
            <el-table-column prop="purpose" label="作用" />
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- ═══ 安全 ═══ -->
      <el-tab-pane label="🛡️ 安全" name="security">
        <el-card class="doc-card"><template #header><h3>多层安全防线</h3></template>
          <el-timeline>
            <el-timeline-item v-for="(item, i) in securityLayers" :key="i" :timestamp="item.layer" :type="item.type" placement="top">
              <h4>{{ item.title }}</h4>
              <p>{{ item.desc }}</p>
            </el-timeline-item>
          </el-timeline>
        </el-card>
        <el-card class="doc-card"><template #header><h3>内容扫描规则（23条）</h3></template>
          <el-table :data="scanRules" border size="small" max-height="400">
            <el-table-column prop="cat" label="类别" width="70" />
            <el-table-column prop="sev" label="级别" width="80">
              <template #default="{row}"><el-tag :type="row.sev==='CRITICAL'?'danger':'warning'" size="small">{{row.sev}}</el-tag></template>
            </el-table-column>
            <el-table-column prop="rule" label="检测规则" min-width="220" />
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- ═══ API参考 ═══ -->
      <el-tab-pane label="📡 API参考" name="api">
        <el-card class="doc-card"><template #header><h3>全部端点</h3></template>
          <el-table :data="allAPIs" border size="small" max-height="500">
            <el-table-column prop="m" label="" width="50" />
            <el-table-column prop="p" label="端点" width="260" />
            <el-table-column prop="a" label="" width="30" />
            <el-table-column prop="d" label="说明" min-width="200" />
          </el-table>
          <el-alert type="info" :closable="false" show-icon style="margin-top:8px">🔒=需JWT认证 · 🌐=公开</el-alert>
        </el-card>
      </el-tab-pane>

      <!-- ═══ 用户验证 ═══ -->
      <el-tab-pane label="✅ 用户验证" name="verify">
        <el-card class="doc-card"><template #header><h3>章节完整性验证</h3></template>
          <p>每章自动计算 SHA256 + HMAC-SHA256 签名，读者可验证内容未被篡改。</p>
          <el-table :data="verifyAPI" border size="small" style="margin-top:8px">
            <el-table-column prop="m" label="" width="60" />
            <el-table-column prop="p" label="端点" width="300" />
            <el-table-column prop="d" label="说明" />
          </el-table>
          <el-divider />
          <h4>验证流程</h4>
          <pre class="code">{{ verifyFlow }}</pre>
          <el-divider />
          <h4>前端验证示例</h4>
          <pre class="code">{{ verifyFrontend }}</pre>
        </el-card>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
const activeTab = ref('quickstart');

const concepts = [
  { name:'模板HTML', desc:'美化小说阅读页面的HTML片段，可调用平台API获取小说数据，支持按钮等交互控件', upload:'文本粘贴', api:'✅ NVS API' },
  { name:'扩展HTML', desc:'独立沙盒应用，ZIP包部署。HTML+CSS+JS+WASM完整前端，支持端口代理和虚拟文件系统', upload:'ZIP包', api:'❌ 完全隔离' },
];

const quickStartTemplate = `<!-- 1. 进入「自定义→UI模板管理」→ 新建模板 -->
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
<\/script>`;

const templateAPI = [
  { method:'window.NVS.getNovelData(novelId?)', returns:'Promise&lt;Object&gt;', desc:'获取小说元数据+章节列表。novelId可选，为空则自动从URL参数?novel=提取' },
  { method:'window.NVS.getCurrentChapter()', returns:'Number', desc:'从iframe URL ?chapter=参数解析当前章节号' },
  { method:'window.NVS.sendMessage(type, data)', returns:'void', desc:'向父页面发送postMessage，type为消息类型，data为JSON对象' },
  { method:"window.addEventListener('nvs-message',fn)", returns:'void', desc:'监听父页面通过CustomEvent发来的消息' },
];

const novelDataResponse = `// GET /api/template/novel/:id 响应
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
}`;

const fullTemplate = `<!DOCTYPE html>
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
<\/script></body></html>`;

const postMessageProto = [
  { direction:'模板→父页面', format:'{source:"nvs-frame", type:"...", data:{...}}', desc:'通过window.NVS.sendMessage()发送，父页面监听message事件' },
  { direction:'父页面→模板', format:'{source:"nvs-parent", ...}', desc:'模板内监听nvs-message CustomEvent接收' },
];

const sandboxLevels = [
  { level:'strict', sandbox:'allow-scripts allow-same-origin', desc:'纯展示（页脚/版权声明），不可交互' },
  { level:'interactive', sandbox:'allow-scripts allow-same-origin allow-forms allow-popups', desc:'含按钮/表单/弹窗（导航栏/工具栏）' },
];

const zipSpec = `my-app.zip
├── index.html       ← 入口文件（默认，可手动选择其他.html）
├── style.css
├── app.js
├── module.wasm      ← WASM模块（需勾选「允许WASM」）
├── data.json
└── assets/
    ├── logo.png
    └── font.woff2

⚠️ 禁止: .exe .dll .so .sh .bat .ps1 .py .php .asp .jsp`;

const netCaps = [
  { feature:'fetch() / XMLHttpRequest', support:'✅', note:'connect-src: self http: https: ws: wss: data: blob:' },
  { feature:'WebSocket', support:'✅', note:'iframe sandbox不限制WebSocket' },
  { feature:'Server-Sent Events', support:'✅', note:'connect-src未限制EventSource' },
  { feature:'WebRTC', support:'⚠️', note:'Permissions-Policy禁用camera/microphone' },
  { feature:'外部脚本', support:'❌', note:'CSP script-src: self 仅同源' },
  { feature:'localStorage/Cookie', support:'✅', note:'allow-same-origin允许同源存储' },
];

const vfsAPI = [
  { method:'POST', path:'/api/sandbox-vfs/:htmlId/write', desc:'写入 {key, value}，单值≤1MB，总量≤10MB/100键' },
  { method:'GET', path:'/api/sandbox-vfs/:htmlId/read?key=...', desc:'读取键值' },
  { method:'DELETE', path:'/api/sandbox-vfs/:htmlId/delete?key=...', desc:'删除键' },
  { method:'GET', path:'/api/sandbox-vfs/:htmlId/list', desc:'列出所有键及大小' },
];

const vfsExample = `// 扩展HTML中使用VFS存储数据
async function saveSettings(theme) {
  await fetch('/api/sandbox-vfs/1/write', {
    method:'POST', headers:{'Content-Type':'application/json'},
    body: JSON.stringify({key:'settings.json', value:JSON.stringify({theme})})
  });
}
async function loadSettings() {
  const r = await fetch('/api/sandbox-vfs/1/read?key=settings.json');
  return (await r.json()).data.value;
}`;

const rustWasmExample = `// Rust (lib.rs) → 编译为 WASM
use wasm_bindgen::prelude::*;
#[wasm_bindgen]
pub fn fibonacci(n: u32) -> u32 {
    match n { 0 => 0, 1 => 1, _ => fibonacci(n-1)+fibonacci(n-2) }
}
// 编译: wasm-pack build --target web`;

const jsGlueCode = `// index.html — 加载WASM模块
<script type="module">
import init, { fibonacci } from './pkg/my_wasm.js';
async function run() {
  await init();
  console.log('fib(10)=', fibonacci(10)); // 55
  document.getElementById('result').textContent = fibonacci(40);
}
run();
<\/script>`;

const wasmHeaders = [
  { header:"Cross-Origin-Embedder-Policy: require-corp", purpose:"启用SharedArrayBuffer（WASM多线程必需）" },
  { header:"Cross-Origin-Opener-Policy: same-origin", purpose:"隔离浏览器上下文" },
  { header:"Content-Security-Policy: ... 'wasm-unsafe-eval'", purpose:"允许WebAssembly编译" },
  { header:"Content-Type: application/wasm", purpose:"正确的WASM MIME类型" },
];

const securityLayers = [
  { layer:'①上传前', type:'primary', title:'内容安全扫描', desc:'23条规则检测提权/Shell注入/木马/挖矿/僵尸网络/数据窃取/钓鱼' },
  { layer:'②解压中', type:'warning', title:'ZIP炸弹检测', desc:'压缩比≤100:1 · 总大小≤50MB · 单文件≤20MB · 文件数≤500' },
  { layer:'③解压后', type:'danger', title:'目录锁定', desc:'文件→0444只读 · 目录→0755可写（允许创建数据文件）· 标记.nvs_sandbox_lock' },
  { layer:'④运行时', type:'danger', title:'写入拦截中间件', desc:'POST/PUT到沙盒路径时检查扩展名，拦截.js/.wasm/.html/.sh等33种脚本扩展名' },
  { layer:'⑤通信', type:'info', title:'CSP + iframe sandbox', desc:'sandbox属性限制iframe能力，CSP限制资源加载和网络连接' },
];

const scanRules = [
  { cat:'提权', sev:'CRITICAL', rule:'child_process / require("child_process") / execSync' },
  { cat:'提权', sev:'CRITICAL', rule:'sudo / su / setuid / chmod 777 / chown root' },
  { cat:'提权', sev:'CRITICAL', rule:'/etc/passwd / /etc/shadow / C:\\Windows\\System32' },
  { cat:'提权', sev:'CRITICAL', rule:'insmod / modprobe / LoadLibrary / dlopen' },
  { cat:'Shell', sev:'CRITICAL', rule:'exec("/bin/...") / cmd.exe / powershell.exe' },
  { cat:'Shell', sev:'CRITICAL', rule:'nc -e / bash -i >& / python -c "import socket"' },
  { cat:'Shell', sev:'HIGH', rule:'wget ... | bash / certutil -urlcache' },
  { cat:'木马', sev:'CRITICAL', rule:'eval(atob(...)) / Function("...")()' },
  { cat:'混淆', sev:'HIGH', rule:'\\xHH 十六进制转义 ≥10次连续' },
  { cat:'木马', sev:'HIGH', rule:'隐藏iframe (hidden/0px/opacity:0)' },
  { cat:'木马', sev:'HIGH', rule:'document.write(atob(...))' },
  { cat:'木马', sev:'MEDIUM', rule:'外部脚本 .ru/.cn/.top/.xyz/.tk等可疑TLD' },
  { cat:'僵尸', sev:'CRITICAL', rule:'WebSocket + setInterval 组合通信' },
  { cat:'僵尸', sev:'HIGH', rule:'定时fetch/XHR心跳轮询' },
  { cat:'窃取', sev:'CRITICAL', rule:'document.cookie + sendBeacon/fetch' },
  { cat:'窃取', sev:'HIGH', rule:'localStorage + sendBeacon' },
  { cat:'窃取', sev:'CRITICAL', rule:'addEventListener keydown + sendBeacon (键盘记录)' },
  { cat:'窃取', sev:'HIGH', rule:'navigator.clipboard.read + sendBeacon' },
  { cat:'挖矿', sev:'CRITICAL', rule:'CoinHive / cryptonight / new Miner(' },
  { cat:'钓鱼', sev:'CRITICAL', rule:'<form action="https://..." ><input password>' },
];

const allAPIs = [
  { m:'GET', p:'/api/userframes', a:'🔒', d:'我的模板列表' },
  { m:'POST', p:'/api/userframes', a:'🔒', d:'创建模板' },
  { m:'GET', p:'/api/userframes/:id', a:'🔒', d:'模板详情+内容' },
  { m:'PUT', p:'/api/userframes/:id', a:'🔒', d:'更新模板' },
  { m:'DELETE', p:'/api/userframes/:id', a:'🔒', d:'删除模板' },
  { m:'GET', p:'/api/userframes/public', a:'🌐', d:'公开模板列表' },
  { m:'GET', p:'/api/userframes/:id/preview', a:'🌐', d:'模板沙盒预览(HTML)' },
  { m:'GET', p:'/api/userhtmls', a:'🔒', d:'我的扩展列表' },
  { m:'POST', p:'/api/userhtmls/upload', a:'🔒', d:'上传ZIP (multipart)' },
  { m:'GET', p:'/api/userhtmls/:id', a:'🔒', d:'扩展详情+文件列表' },
  { m:'PUT', p:'/api/userhtmls/:id', a:'🔒', d:'更新扩展元数据' },
  { m:'DELETE', p:'/api/userhtmls/:id', a:'🔒', d:'删除扩展' },
  { m:'GET', p:'/api/userhtmls/:id/preview', a:'🌐', d:'扩展沙盒预览' },
  { m:'GET', p:'/api/novels/:id/frames', a:'🌐', d:'作品关联模板' },
  { m:'GET', p:'/api/novels/:id/htmls', a:'🌐', d:'作品关联扩展HTML' },
  { m:'GET', p:'/api/template/novel/:id', a:'🌐', d:'小说数据API(模板调用)' },
  { m:'POST', p:'/api/port/allocate', a:'🔒', d:'分配代理端口' },
  { m:'POST', p:'/api/port/release', a:'🔒', d:'释放端口' },
  { m:'GET', p:'/api/port/list', a:'🔒', d:'已分配端口列表' },
  { m:'GET', p:'/api/sandbox/info', a:'🌐', d:'沙盒系统信息' },
  { m:'ANY', p:'/sandbox/proxy/:uid/:name/*', a:'🌐', d:'命名端口代理' },
  { m:'POST', p:'/api/sandbox-vfs/:id/write', a:'🌐', d:'VFS写入' },
  { m:'GET', p:'/api/sandbox-vfs/:id/read', a:'🌐', d:'VFS读取' },
  { m:'DELETE', p:'/api/sandbox-vfs/:id/delete', a:'🌐', d:'VFS删除' },
  { m:'GET', p:'/api/sandbox-vfs/:id/list', a:'🌐', d:'VFS列表' },
];

const verifyAPI = [
  { m:'GET', p:'/api/novels/:id/chapters/:num/verify', d:'返回章节实时SHA256+HMAC签名验证信息' },
  { m:'GET', p:'/api/novels/:id/chapters/:num', d:'获取章节内容(含hash_match/signature_verified字段)' },
];

const verifyFlow = `// 验证流程
// 1. 获取章节 → 响应含 content_hash + content_signature
// 2. 本地计算 SHA256(content) → 对比 content_hash
// 3. 用作者公钥验证 HMAC-SHA256(content, signing_key) → 对比 content_signature
// 4. 前端显示验证结果 ✓/✗`;

const verifyFrontend = `// 前端章节完整性验证
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
}`;
</script>

<style scoped>
.page-container{max-width:1020px;margin:0 auto;padding:24px 20px 60px}
.page-header{margin-bottom:24px}
.page-header h2{margin:0;font-size:1.5rem}
.subtitle{color:var(--text-secondary);margin:6px 0 0;font-size:.9rem}
.doc-card{margin-bottom:20px}
.doc-card :deep(h3){margin:0;font-size:1.1rem}
.doc-card :deep(h4){margin:12px 0 6px;font-size:1rem;color:var(--primary-color)}
.code{background:#1e1e2e;color:#cdd6f4;border:1px solid #313244;border-radius:8px;padding:14px 16px;font-family:'SF Mono',Fira Code,Cascadia Code,monospace;font-size:.78rem;line-height:1.6;overflow-x:auto;white-space:pre;max-height:420px;overflow-y:auto}
:deep(.el-timeline__item) h4{margin:0 0 4px}
:deep(.el-timeline__item) p{margin:0;color:var(--text-secondary);font-size:.88rem}
</style>