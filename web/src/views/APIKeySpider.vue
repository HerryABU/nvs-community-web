<template>
  <div class="page-container">
    <div class="page-header">
      <h2>🕷️ API Key 爬虫 / 客户端接入指南</h2>
      <p class="subtitle">用 API Key 合规、高速地批量读取 NVS 公开数据 · 本页随二进制一同部署，离线可用</p>
      <el-alert type="info" :closable="false" show-icon style="margin-bottom:16px">
        携带 API Key 后身份从「匿名」升级为你的角色（reader/author/admin），配额远高于匿名通道，且不会被误判为爬虫而降级。
        管理员可在 <b>管理后台 → 限流与 API Key</b> 查看全站 Key 的累计调用次数。
      </el-alert>
    </div>

    <el-tabs v-model="activeTab" type="border-card">
      <!-- ═══ 概述 ═══ -->
      <el-tab-pane label="📌 概述" name="overview">
        <el-card class="doc-card">
          <template #header><h3>为什么要用 API Key</h3></template>
          <el-table :border="true" size="small" :data="whyTable">
            <el-table-column prop="way" label="访问方式" width="160" />
            <el-table-column prop="quota" label="配额" width="180" />
            <el-table-column prop="desc" label="说明" />
          </el-table>
          <el-divider />
          <p>本指南面向：批量抓取公开小说/章节的脚本、第三方阅读器、搜索引擎爬虫、数据分析客户端。</p>
          <p>核心原则：<b>带 Key 的自动化请求</b>走的是你账户的配额通道，速度快、不会被降级；<b>不带 Key</b> 的匿名请求配额极低，一旦被识别为爬虫会进一步压低，命中已知爬虫名单会直接 <code>403</code>。</p>
        </el-card>
      </el-tab-pane>

      <!-- ═══ 申请 Key ═══ -->
      <el-tab-pane label="🔑 申请 Key" name="apply">
        <el-card class="doc-card">
          <template #header><h3>三步拿到 API Key</h3></template>
          <el-steps :active="3" align-center finish-status="success" style="margin:16px 0">
            <el-step title="进入设置" description="个人设置 → API Key 管理" />
            <el-step title="创建 Key" description="填备注名与自设配额" />
            <el-step title="保存 Key" description="仅展示一次，立即复制" />
          </el-steps>
          <ol class="steps-list">
            <li>登录站点后进入 <b>个人设置 → API Key 管理</b>。</li>
            <li>填写备注名（如「我的阅读器」）与自设配额，点击「创建 API Key」。</li>
            <li><b>完整 Key 仅展示一次</b>，请立即复制保存；后台只存 SHA-256 哈希，丢失后只能重新生成。</li>
          </ol>
          <el-alert type="warning" :closable="false" show-icon style="margin-top:12px">
            创建时可限制 <b>IP 白名单</b> 与 <b>过期时间</b>，进一步降低被盗用风险。如怀疑泄露，立即在卡片中「停用」或「删除」该 Key。
          </el-alert>
        </el-card>
      </el-tab-pane>

      <!-- ═══ 调用方式 ═══ -->
      <el-tab-pane label="💻 调用方式" name="usage">
        <el-card class="doc-card">
          <template #header><h3>Python · requests 最小示例</h3></template>
          <pre class="code">{{ requestsExample }}</pre>
          <el-divider />
          <h4>返回结构</h4>
          <pre class="code">{{ responseExample }}</pre>
        </el-card>
      </el-tab-pane>

      <!-- ═══ 公开接口 ═══ -->
      <el-tab-pane label="📡 公开接口" name="endpoints">
        <el-card class="doc-card">
          <template #header><h3>常见公开端点</h3></template>
          <el-table :border="true" size="small" :data="endpoints">
            <el-table-column prop="m" label="方法" width="70" />
            <el-table-column prop="p" label="端点" width="280" />
            <el-table-column prop="d" label="说明" />
          </el-table>
          <el-alert type="info" :closable="false" show-icon style="margin-top:12px">
            若站点挂在反向代理前缀下（如 <code>/go-nvs</code>），请把前缀加在 <code>/api</code> 之前，例如
            <code>https://example.com/go-nvs/api/novels</code>。本页「在线测试」会自动带上前缀。
          </el-alert>
        </el-card>
      </el-tab-pane>

      <!-- ═══ 分页遍历 ═══ -->
      <el-tab-pane label="📄 分页遍历" name="paging">
        <el-card class="doc-card">
          <template #header><h3>翻页抓取全部作品</h3></template>
          <pre class="code">{{ paginationExample }}</pre>
        </el-card>
      </el-tab-pane>

      <!-- ═══ 限流重试 ═══ -->
      <el-tab-pane label="⏳ 限流与重试" name="retry">
        <el-card class="doc-card">
          <template #header><h3>指数退避重试（429 / 5xx）</h3></template>
          <p>携带 Key 后按 Key 维度限流；到达上限会收到 <code>429 Too Many Requests</code>。建议实现指数退避重试：</p>
          <pre class="code">{{ retryExample }}</pre>
          <el-alert type="success" :closable="false" show-icon style="margin-top:12px">
            即使在配额内，也建议每次请求间 <code>sleep(0.3)</code> 礼貌访问，避免突发打满。
          </el-alert>
        </el-card>
      </el-tab-pane>

      <!-- ═══ 调用次数 ═══ -->
      <el-tab-pane label="🔢 调用次数" name="count">
        <el-card class="doc-card">
          <template #header><h3>查看累计调用次数</h3></template>
          <el-table :border="true" size="small" :data="countTable">
            <el-table-column prop="side" label="位置" width="120" />
            <el-table-column prop="path" label="路径" width="320" />
            <el-table-column prop="col" label="字段" />
          </el-table>
          <p style="margin-top:12px">调用次数在<b>每次携带有效 Key 的请求被成功处理后自动累计</b>；重新生成 Key 后次数归零。下方「在线测试」每成功调用一次，次数 +1（可在设置页刷新查看）。</p>
        </el-card>
      </el-tab-pane>

      <!-- ═══ 注意事项 ═══ -->
      <el-tab-pane label="⚠️ 注意事项" name="notes">
        <el-card class="doc-card">
          <template #header><h3>安全与合规</h3></template>
          <el-timeline>
            <el-timeline-item v-for="(n, i) in notes" :key="i" :timestamp="n.t" :type="n.type" placement="top">
              <p>{{ n.d }}</p>
            </el-timeline-item>
          </el-timeline>
        </el-card>
      </el-tab-pane>

      <!-- ═══ 在线测试 ═══ -->
      <el-tab-pane label="🧪 在线测试" name="test">
        <el-card class="doc-card">
          <template #header><h3>用你的 Key 实测一个请求</h3></template>
          <p class="hint">粘贴你创建的 API Key（页面不会上传到任何第三方，仅向本站点发起同源请求，浏览器本地执行）。</p>
          <el-form label-width="90px" label-position="top">
            <el-form-item label="API Key">
              <el-input v-model="testKey" type="password" show-password placeholder="nvs_rdr_xxx...（在 个人设置 → API Key 管理 创建）" size="small" style="max-width:520px" />
            </el-form-item>
            <el-form-item label="测试端点">
              <el-select v-model="testEndpoint" size="small" style="max-width:520px;width:100%">
                <el-option v-for="e in endpoints" :key="e.p" :label="`${e.m} ${e.p}`" :value="e.p" />
              </el-select>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" size="small" :loading="testing" @click="runTest">发起测试请求</el-button>
              <el-button size="small" @click="clearTest">清空结果</el-button>
            </el-form-item>
          </el-form>

          <div v-if="testResult" style="margin-top:8px">
            <el-alert
              :type="testResult.ok ? 'success' : 'error'"
              :closable="false"
              show-icon
              :title="testResult.title"
            >
              <template #default>
                <div v-if="testResult.status">HTTP 状态：<code>{{ testResult.status }}</code></div>
                <div v-if="testResult.note">{{ testResult.note }}</div>
              </template>
            </el-alert>
            <h4 style="margin:14px 0 6px">响应预览</h4>
            <pre class="code">{{ testResult.body }}</pre>
          </div>

          <el-alert type="info" :closable="false" show-icon style="margin-top:14px">
            提示：若站点部署在反向代理前缀（如 <code>/go-nvs</code>）下，本测试已自动拼接前缀。看到的响应即你的客户端将拿到的数据形状。
          </el-alert>
        </el-card>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';

const activeTab = ref('overview');

const whyTable = [
  { way: '匿名（无 Key）', quota: '极低', desc: '被识别为爬虫后进一步降级；命中已知爬虫名单直接 403' },
  { way: '携带 API Key', quota: '按角色配额', desc: 'reader/author/admin 配额远高于匿名，不会被误判，可设更小自配额' },
];

const requestsExample = `import requests

BASE_URL = "https://your-nvs.example.com"  # 反向代理前缀 /go-nvs 则写 https://example.com/go-nvs
API_KEY = "nvs_rdr_3f2a1bxxxxxxxxxxxxxxxxxxxxxxxx"

headers = {
    "X-API-Key": API_KEY,
    "User-Agent": "MyNVSReader/1.0 (contact@example.com)",  # 建议写真实联系方式
    "Accept": "application/json",
}

r = requests.get(f"{BASE_URL}/api/novels?page=1&page_size=20", headers=headers)
print(r.status_code, r.json())`;

const responseExample = `{
  "code": 0,
  "message": "ok",
  "data": {
    "list": [ ... ],
    "total": 128,
    "page": 1,
    "page_size": 20
  }
}`;

const endpoints = [
  { m: 'GET', p: '/api/novels', d: '作品列表 ?page=1&page_size=20&search=关键词&sort_by=featured' },
  { m: 'GET', p: '/api/novels/:id', d: '作品详情，如 /api/novels/42' },
  { m: 'GET', p: '/api/novels/:id/chapters', d: '章节列表' },
  { m: 'GET', p: '/api/novels/:id/chapters/:num', d: '章节正文，如 /api/novels/42/chapters/1' },
  { m: 'GET', p: '/api/categories', d: '分类列表，无参数' },
  { m: 'GET', p: '/api/novels/:id/forum', d: '作品论坛帖子 ?page=1&page_size=20' },
];

const paginationExample = `import time

PAGE_SIZE = 50
for page in range(1, 1000):
    url = f"{BASE_URL}/api/novels?page={page}&page_size={PAGE_SIZE}&sort_by=updated_at"
    r = requests.get(url, headers=headers)
    r.raise_for_status()
    data = r.json().get("data", {})
    items = data.get("list", [])
    if not items:
        break
    for novel in items:
        process_novel(novel)  # 你的业务逻辑
    if page * PAGE_SIZE >= data.get("total", 0):
        break
    time.sleep(0.3)  # 礼貌访问`;

const retryExample = `import requests
from requests.adapters import HTTPAdapter, Retry

session = requests.Session()
retries = Retry(total=5, backoff_factor=1, status_forcelist=[429, 500, 502, 503, 504])
session.mount("https://", HTTPAdapter(max_retries=retries))
session.mount("http://", HTTPAdapter(max_retries=retries))

# 之后用 session.get(...) 自动重试`;

const countTable = [
  { side: '用户侧', path: '个人设置 → API Key 管理', col: '表格「调用次数」列' },
  { side: '管理侧', path: '管理后台 → 限流与 API Key', col: '全站 Key 一览「调用次数」列' },
];

const notes = [
  { t: '① 防泄露', type: 'danger', d: '不要把 Key 写入前端代码或公开仓库。怀疑泄露立即停用/删除。' },
  { t: '② IP 白名单', type: 'warning', d: '创建 Key 时可限制来源 IP/CIDR，降低被盗用风险。' },
  { t: '③ 过期时间', type: 'warning', d: '可为 Key 设置过期时间，到期自动失效。' },
  { t: '④ User-Agent', type: 'info', d: '建议填写可识别的 User-Agent 与联系方式，避免被管理员误判为恶意爬虫。' },
];

// ====== 在线测试 ======
const testKey = ref('');
const testEndpoint = ref('/api/novels');
const testing = ref(false);
const testResult = ref<{ ok: boolean; title: string; status?: number; body: string; note?: string } | null>(null);

function prefix(): string {
  return (window as any).__NVS_PREFIX__ || '';
}

async function runTest() {
  if (!testKey.value.trim()) {
    testResult.value = { ok: false, title: '请先粘贴 API Key', body: '' };
    return;
  }
  testing.value = true;
  testResult.value = null;
  const url = prefix() + testEndpoint.value;
  try {
    const res = await fetch(url, {
      headers: {
        'X-API-Key': testKey.value.trim(),
        Accept: 'application/json',
      },
    });
    const text = await res.text();
    let body = text;
    let note = '';
    try {
      body = JSON.stringify(JSON.parse(text), null, 2);
    } catch {
      /* 非 JSON，原样展示 */
    }
    if (res.status === 200) {
      note = '请求成功，本次调用已计入该 Key 的「调用次数」（可到 个人设置 → API Key 管理 刷新查看）。';
    } else if (res.status === 401 || res.status === 403) {
      note = 'Key 无效或未携带正确头；请确认 Key 完整且仍处启用状态。';
    } else if (res.status === 429) {
      note = '触发限流，请稍后重试（建议实现指数退避）。';
    }
    testResult.value = {
      ok: res.status === 200,
      title: res.status === 200 ? `调用成功（${res.status}）` : `调用返回 ${res.status}`,
      status: res.status,
      body,
      note,
    };
  } catch (e: any) {
    testResult.value = {
      ok: false,
      title: '请求失败',
      body: e?.message || String(e),
      note: '请确认站点可访问；若部署在反向代理前缀下，本测试已自动拼接前缀。',
    };
  } finally {
    testing.value = false;
  }
}

function clearTest() {
  testResult.value = null;
}
</script>

<style scoped>
.page-container { max-width: 1020px; margin: 0 auto; padding: 24px 20px 60px }
.page-header { margin-bottom: 24px }
.page-header h2 { margin: 0; font-size: 1.5rem }
.subtitle { color: var(--text-secondary); margin: 6px 0 0; font-size: .9rem }
.doc-card { margin-bottom: 20px }
.doc-card :deep(h3) { margin: 0; font-size: 1.1rem }
.doc-card :deep(h4) { margin: 12px 0 6px; font-size: 1rem; color: var(--primary-color) }
.code { background:#1e1e2e; color:#cdd6f4; border:1px solid #313244; border-radius:8px; padding:14px 16px;
  font-family:'SF Mono',Fira Code,Cascadia Code,monospace; font-size:.78rem; line-height:1.6;
  overflow-x:auto; white-space:pre; max-height:420px; overflow-y:auto }
.steps-list { padding-left: 20px; line-height: 2 }
.steps-list li { margin: 4px 0 }
.hint { color: var(--text-light, #666); font-size: .85rem; line-height: 1.6; margin: 0 0 12px }
:deep(.el-timeline__item) p { margin: 0; color: var(--text-secondary); font-size:.88rem }

/* ====== 移动端适配 ====== */
@media (max-width: 768px) {
  .page-container { padding: 16px 12px }
  .code { font-size: .72rem; padding: 10px 12px; max-height: 320px }
  .page-container :deep(.el-table) { min-width: 560px }
  .page-container :deep(.el-table__wrapper) { overflow-x: auto }
}
</style>
