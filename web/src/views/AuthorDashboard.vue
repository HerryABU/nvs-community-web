<template>
  <div class="dashboard-bg">
    <div class="dashboard-container">
      <!-- 顶部标题 -->
      <div class="dashboard-hero">
        <h1 class="dashboard-main-title">
          <span class="title-glow">创作数据大屏</span>
        </h1>
        <p class="dashboard-subtitle">跟踪创作数据 · 管理作品</p>
        <div class="hero-buttons">
          <el-button class="create-btn" size="large" @click="$router.push('/author/editor')">
            <el-icon><EditPen /></el-icon>新建作品
          </el-button>
          <el-button size="large" @click="$router.push('/author/blog/new')">
            <el-icon><Edit /></el-icon>写博客
          </el-button>
          <el-button size="large" @click="goMyBlogs">
            <el-icon><Notebook /></el-icon>我的博客
          </el-button>
          <el-button size="large" @click="$router.push('/collab')">
            <el-icon><UserFilled /></el-icon>协作
          </el-button>
        </div>
      </div>

      <!-- 作者名片：头像 + 签名状态 -->
      <AuthorCard />

      <!-- 第一行：统计卡片 -->
      <el-row :gutter="20" class="stats-row">
        <el-col :xs="24" :sm="12" :md="6" v-for="(card, idx) in statCardDefs" :key="idx">
          <div class="glass-stat-card">
            <div class="glass-stat-icon" :style="{ background: card.gradient }">
              <el-icon :size="24"><component :is="card.icon" /></el-icon>
            </div>
            <div class="glass-stat-body">
              <AnimatedNumber
                :target="card.value"
                :duration="1400"
                class="glass-stat-value"
              />
              <div class="glass-stat-label">{{ card.label }}</div>
            </div>
          </div>
        </el-col>
      </el-row>

      <!-- 第二行：趋势图（多指标切换）+ 柱状图（多指标切换）-->
      <el-row :gutter="20" class="chart-row">
        <el-col :xs="24" :md="12">
          <DashboardCharts :metrics="authorTrendMetrics" />
        </el-col>
        <el-col :xs="24" :md="12">
          <DashboardCharts :metrics="novelBarMetrics" />
        </el-col>
      </el-row>

      <!-- 第三行：收藏/阅读（合并切换）+ 评分仪表盘 -->
      <el-row :gutter="20" class="chart-row">
        <el-col :xs="24" :md="12">
          <DashboardCharts :metrics="statBarMetrics" />
        </el-col>
        <el-col :xs="24" :md="12">
          <DashboardCharts :rating-gauge="avgRating" />
        </el-col>
      </el-row>

      <!-- 博客管理 -->
      <div class="novels-section">
        <h2 class="section-title-bar">
          <span class="title-bar-text">我的博客</span>
          <span class="title-bar-count">{{ myBlogs.length }} 篇</span>
        </h2>
        <div v-loading="loadingBlogs" style="min-height:60px">
          <el-empty v-if="!loadingBlogs && myBlogs.length === 0" description="还没有博客文章，点击上方「写博客」开始吧！" />
          <div v-else class="blog-list-dash">
            <div v-for="blog in myBlogs" :key="blog.id" class="blog-item-dash">
              <div class="blog-item-main">
                <el-tag v-if="blog.is_pinned" size="small" type="danger" style="margin-right:6px">置顶</el-tag>
                <el-tag v-if="blog.category" size="small" style="margin-right:6px">{{ blog.category }}</el-tag>
                <span class="blog-item-title-dash">{{ blog.title }}</span>
                <span class="blog-item-meta">{{ formatDate(blog.created_at) }} · {{ blog.view_count }} 阅读<template v-if="blog.like_count"> · 👍 {{ blog.like_count }}</template></span>
              </div>
              <div class="blog-item-actions">
                <el-button size="small" text type="primary" @click="$router.push(`/author/${authStore.user?.username || authStore.user?.id}/blogs/${blog.id}`)">查看</el-button>
                <el-button size="small" text type="primary" @click="$router.push(`/author/blog/edit/${blog.id}`)">编辑</el-button>
                <el-popconfirm title="确定删除？" @confirm="deleteMyBlog(blog.id)">
                  <template #reference><el-button size="small" text type="danger">删除</el-button></template>
                </el-popconfirm>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 自定义 HTML 管理（统一收纳） -->
      <div class="novels-section">
        <h2 class="section-title-bar">
          <span class="title-bar-text">🎨 自定义 HTML 管理</span>
          <span class="title-bar-count">{{ htmlStats.frames }} 模板 · {{ htmlStats.htmls }} 扩展</span>
        </h2>
        <div class="html-mgr-grid">
          <div class="html-mgr-card" @click="$router.push('/author/templates')">
            <div class="hmc-icon">🎨</div>
            <div class="hmc-body">
              <div class="hmc-title">主题模板（UI 美化）</div>
              <div class="hmc-desc">阅读页 / 作者主页模板，支持沙盒与非沙盒注入</div>
              <div class="hmc-meta">
                <el-tag size="small" type="info">{{ htmlStats.frames }} 个</el-tag>
                <el-tag v-if="htmlStats.framePending > 0" size="small" type="warning">{{ htmlStats.framePending }} 待复核</el-tag>
              </div>
            </div>
          </div>
          <div class="html-mgr-card" @click="$router.push('/htmls')">
            <div class="hmc-icon">🔌</div>
            <div class="hmc-body">
              <div class="hmc-title">扩展应用（HTML 托管）</div>
              <div class="hmc-desc">ZIP 上传完整 HTML 应用，沙盒运行 / 广场发布</div>
              <div class="hmc-meta">
                <el-tag size="small" type="info">{{ htmlStats.htmls }} 个</el-tag>
                <el-tag v-if="htmlStats.htmlPending > 0" size="small" type="warning">{{ htmlStats.htmlPending }} 待复核</el-tag>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- API Key 管理（作者可在工作台直接创建 / 管理，用于爬虫 / 客户端接入） -->
      <div class="novels-section">
        <h2 class="section-title-bar">
          <span class="title-bar-text">🔑 API Key 管理</span>
          <span class="title-bar-count">{{ myKeys.length }} 个</span>
        </h2>
        <div class="apikey-panel">
          <el-alert type="info" :closable="false" show-icon style="margin-bottom: 14px"
            :title="`当前身份：${roleLabel} · 本角色 Key 配额上限 ${roleQuota.req} 次 / ${roleQuota.window} 秒`">
            <template #default>
              <div class="apikey-hint">
                创建 API Key 后，调用本站 API 时携带 <code>X-API-Key</code> 请求头即可获得远高于匿名访问的配额。
                不带 Key 的请求会被压到很低的匿名配额，被识别为爬虫则更低。批量抓取 / 自动化脚本想提速，请带上你的 Key。
                详细接入示例见站内页面 <router-link to="/api-spider">「🕷️ API Key 爬虫指南」</router-link>。
              </div>
            </template>
          </el-alert>

          <el-form label-width="90px" label-position="top" style="margin-bottom: 16px">
            <el-row :gutter="12">
              <el-col :xs="24" :sm="8">
                <el-form-item label="备注名">
                  <el-input v-model="createForm.name" placeholder="如：我的阅读器" size="small" />
                </el-form-item>
              </el-col>
              <el-col :xs="12" :sm="5">
                <el-form-item label="配额（次/分）">
                  <el-input-number v-model="createForm.quota" :min="1" :max="roleQuota.req" size="small" style="width: 100%" />
                </el-form-item>
              </el-col>
              <el-col :xs="24" :sm="11">
                <el-form-item label="过期时间（可选）">
                  <el-date-picker
                    v-model="createForm.expiresAt"
                    type="date"
                    placeholder="永久有效"
                    size="small"
                    style="width: 100%"
                    value-format="YYYY-MM-DD"
                  />
                </el-form-item>
              </el-col>
            </el-row>
            <el-form-item label="IP 白名单（可选）">
              <el-input v-model="createForm.allowedIps" placeholder="如 1.2.3.4, 10.0.0.0/8（留空 = 不限 IP）" size="small" />
            </el-form-item>
            <el-button type="primary" size="small" @click="createKey" :loading="creatingKey">创建 API Key</el-button>
          </el-form>

          <el-divider style="margin: 8px 0 14px" />
          <el-table :data="myKeys" size="small" v-loading="loadingKeys" style="width: 100%">
            <el-table-column prop="name" label="备注" min-width="100">
              <template #default="{ row }">{{ row.name || '—' }}</template>
            </el-table-column>
            <el-table-column label="Key 前缀" width="160">
              <template #default="{ row }"><code>{{ row.key_prefix }}</code></template>
            </el-table-column>
            <el-table-column label="配额" width="100">
              <template #default="{ row }">{{ row.quota_per_min }} 次/分</template>
            </el-table-column>
            <el-table-column label="状态" width="80">
              <template #default="{ row }">
                <el-tag :type="row.is_active ? 'success' : 'danger'" size="small">{{ row.is_active ? '启用' : '停用' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="过期" width="100">
              <template #default="{ row }">{{ row.expires_at ? row.expires_at.slice(0, 10) : '永久' }}</template>
            </el-table-column>
            <el-table-column label="最近使用" width="150">
              <template #default="{ row }">{{ formatTime(row.last_used_at) }}</template>
            </el-table-column>
            <el-table-column label="调用次数" width="90">
              <template #default="{ row }">{{ row.total_calls ?? 0 }}</template>
            </el-table-column>
            <el-table-column label="操作" width="200" fixed="right">
              <template #default="{ row }">
                <el-button size="small" text @click="toggleKey(row)">{{ row.is_active ? '停用' : '启用' }}</el-button>
                <el-button size="small" text type="warning" @click="regenerateKey(row)">重生成</el-button>
                <el-button size="small" text type="danger" @click="deleteKey(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
          <p v-if="!myKeys.length && !loadingKeys" class="apikey-empty">尚未创建 API Key</p>
        </div>
      </div>

      <!-- 完整 Key 一次性展示 -->
      <el-dialog v-model="showNewKey" title="API Key 创建成功" width="540px">
        <el-alert type="warning" :closable="false" show-icon title="完整 Key 仅此一次展示，请立即复制保存" style="margin-bottom: 12px" />
        <div class="newkey-box">
          <code class="newkey-code">{{ newKeyValue }}</code>
          <el-button type="primary" @click="copyKey">复制</el-button>
        </div>
        <p class="apikey-hint">
          请求时携带 <code>X-API-Key: {{ newKeyPrefix }}…</code>（完整 Key）。服务端仅存储哈希，遗失后只能重新生成。
        </p>
      </el-dialog>

      <!-- 底部：作品管理卡片列表 -->
      <div class="novels-section">
        <h2 class="section-title-bar">
          <span class="title-bar-text">我的作品</span>
          <span class="title-bar-count">{{ novels.length }} 部</span>
        </h2>
        <div v-loading="loadingNovels">
          <el-empty v-if="!loadingNovels && novels.length === 0" description="还没有作品，快去创作吧！">
            <el-button type="primary" @click="$router.push('/author/editor')">新建作品</el-button>
          </el-empty>

          <div class="novel-card-grid" v-else>
            <div v-for="novel in novels" :key="novel.id" class="novel-card glass-novel-card">
              <div class="novel-card-cover">
                <el-image v-if="novel.cover_url" :src="novel.cover_url" fit="cover" class="cover-img">
                  <template #error>
                    <div class="cover-placeholder"><el-icon :size="32"><Document /></el-icon></div>
                  </template>
                </el-image>
                <div v-else class="cover-placeholder"><el-icon :size="32"><Document /></el-icon></div>
                <div class="novel-status-badge" :class="novel.status">
                  {{ novel.status === 'published' ? '已发布' : novel.status === 'draft' ? '草稿' : '已隐藏' }}
                </div>
              </div>
              <div class="novel-card-body">
                <h3 class="novel-card-title">{{ novel.title }}</h3>
                <div class="novel-card-tags">
                  <template v-if="novel.categories && novel.categories.length > 0">
                    <el-tag size="small" v-for="cat in novel.categories" :key="cat" class="cat-tag">{{ cat }}</el-tag>
                  </template>
                  <el-tag size="small" v-else class="cat-tag">{{ novel.category }}</el-tag>
                </div>
                <!-- 隔离墙开关（仅敏感分类作品显示） -->
                <div v-if="isSensitiveCategory(novel)" class="wall-toggle-row">
                  <span style="font-size:0.8rem;color:#999">隔离确认</span>
                  <el-switch
                    :model-value="novel.wall_enabled !== false"
                    size="small"
                    :loading="wallToggling === novel.id"
                    @change="(val: boolean) => toggleWall(novel, val)"
                  />
                  <span style="font-size:0.75rem" :style="{color: novel.wall_enabled !== false ? '#22c55e' : '#999'}">
                    {{ novel.wall_enabled !== false ? '已启用' : '已关闭' }}
                  </span>
                </div>
                <div class="novel-card-meta">
                  <span><el-icon><Collection /></el-icon>{{ novel.total_chapters || 0 }} 章</span>
                  <span><el-icon><MagicStick /></el-icon>{{ (novel.total_words || 0).toLocaleString() }} 字</span>
                </div>
              </div>
              <div class="novel-card-actions">
                <el-button size="small" type="primary" plain @click="$router.push(`/author/editor/${novel.id}`)">
                  <el-icon><Edit /></el-icon>编辑
                </el-button>
                <el-button size="small" plain @click="$router.push(`/novel/${novel.id}`)">
                  <el-icon><View /></el-icon>查看
                </el-button>
                <el-button size="small" plain @click="openCollab(novel.id)">
                  <el-icon><UserFilled /></el-icon>协作
                </el-button>
                <el-dropdown trigger="click" @command="(fmt: string) => handleExport(novel.id, fmt)">
                  <el-button size="small" type="warning" plain :loading="exportingId === novel.id">
                    <el-icon><Download /></el-icon>导出<el-icon class="el-icon--right"><ArrowDown /></el-icon>
                  </el-button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item command="zip">ZIP 打包</el-dropdown-item>
                      <el-dropdown-item command="epub">EPUB 电子书</el-dropdown-item>
                      <el-dropdown-item command="docx">Word 文档</el-dropdown-item>
                      <el-dropdown-item command="md">合并 Markdown</el-dropdown-item>
                      <el-dropdown-item command="txt">纯文本 TXT</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
                <el-button size="small" type="danger" plain @click="handleDelete(novel)">
                  <el-icon><Delete /></el-icon>删除
                </el-button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 我创建的论坛（可管理：禁言/解禁，管理员封禁的仅管理员可解） -->
      <div class="novels-section" v-if="myForums.length > 0">
        <h2 class="section-title-bar">
          <span class="title-bar-text">我创建的论坛</span>
          <span class="title-bar-count">{{ myForums.length }} 个</span>
        </h2>
        <div class="forum-admin-list">
          <div v-for="f in myForums" :key="f.id" class="forum-admin-item">
            <div class="forum-admin-main">
              <span class="forum-admin-name">{{ f.name }}</span>
              <el-tag v-if="f.room_code" size="small" style="cursor:pointer" title="点击复制房间号" @click="copyRoomCode(f.room_code)">🚪 {{ f.room_code }}</el-tag>
              <el-tag v-if="f.has_password" size="small" type="warning">🔒 密码</el-tag>
              <el-tag v-if="f.review_status === 'pending'" size="small" type="info">待审查</el-tag>
              <el-tag v-if="f.review_status === 'rejected'" size="small" type="danger">已驳回</el-tag>
              <span class="forum-admin-meta">{{ f.thread_count }} 个帖子</span>
            </div>
            <el-button size="small" type="primary" plain @click="$router.push(`/forum/${f.id}`)">管理</el-button>
          </div>
        </div>
      </div>

      <!-- 回收站（用户自删内容，可恢复；管理员封禁的不可自行恢复） -->
      <div class="novels-section" v-if="recycle.novels.length > 0 || recycle.blogs.length > 0">
        <h2 class="section-title-bar">
          <span class="title-bar-text">🗑 回收站</span>
          <span class="title-bar-count">{{ recycle.novels.length + recycle.blogs.length }} 项</span>
        </h2>
        <el-alert type="info" :closable="false" show-icon style="margin-bottom: 12px"
          title="以下内容由您自己删除，可随时恢复；被管理员封禁的内容不在此处，请联系平台申诉。" />
        <div class="recycle-list">
          <div v-for="n in recycle.novels" :key="'rn' + n.id" class="recycle-item">
            <el-tag type="warning" size="small">小说</el-tag>
            <span class="recycle-title">《{{ n.title }}》</span>
            <span class="recycle-meta">{{ n.total_chapters || 0 }} 章 · {{ (n.total_words || 0).toLocaleString() }} 字</span>
            <el-button size="small" type="success" plain @click="restoreRecycle('novel', n.id)">恢复</el-button>
          </div>
          <div v-for="b in recycle.blogs" :key="'rb' + b.id" class="recycle-item">
            <el-tag type="warning" size="small">博客</el-tag>
            <span class="recycle-title">{{ b.title }}</span>
            <span class="recycle-meta">{{ formatDate(b.created_at) }}</span>
            <el-button size="small" type="success" plain @click="restoreRecycle('blog', b.id)">恢复</el-button>
          </div>
        </div>
      </div>

      <!-- 封禁内容（176号令合规告知） -->
      <div class="novels-section banned-section" v-if="bannedContent.novels.length > 0 || bannedContent.chapters.length > 0">
        <h2 class="section-title-bar">
          <span class="title-bar-text">已封禁内容</span>
          <span class="title-bar-count">{{ bannedContent.novels.length + bannedContent.chapters.length }} 项</span>
        </h2>
        <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 12px"
          title="以下内容因违反平台合规规范（176号令）被封禁，如有异议请联系平台申诉。" />
        <div class="banned-list">
          <div v-for="n in bannedContent.novels" :key="'n' + n.id" class="banned-item">
            <div class="banned-head">
              <el-tag type="danger" size="small">小说</el-tag>
              <span class="banned-title">《{{ n.title }}》</span>
            </div>
            <div class="banned-reason"><span class="banned-label">封禁原因：</span>{{ n.ban_reason || '未填写' }}</div>
            <div class="banned-time">封禁时间：{{ formatDate(n.banned_at) }}</div>
          </div>
          <div v-for="c in bannedContent.chapters" :key="'c' + c.id" class="banned-item">
            <div class="banned-head">
              <el-tag type="danger" size="small">章节</el-tag>
              <span class="banned-title">《{{ c.novel_title }}》第{{ c.chapter_number }}章 {{ c.title }}</span>
            </div>
            <div class="banned-reason"><span class="banned-label">封禁原因：</span>{{ c.ban_reason || '未填写' }}</div>
            <div class="banned-time">封禁时间：{{ formatDate(c.banned_at) }}</div>
          </div>
        </div>
      </div>

      <!-- 多人协作：协作者 / 角色 / 分成 / 署名（docs/novel-collaboration.md） -->
      <CollaboratorsPanel v-model:visible="collabVisible" :novel-id="collabNovelId" />
    </div>

  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed, watch, nextTick } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useAuthStore } from '@/stores/auth';
import { novelApi, forumApi, type Novel } from '@/api/novel';
import { authApi } from '@/api/auth';
import { adminApi } from '@/api/admin';
import { blogApi } from '@/api/social';
import { frameApi, htmlApi } from '@/api/frame';
import { apiKeyApi } from '@/api/apikey';
import { ElMessage, ElMessageBox } from 'element-plus';
import {
  ArrowDown, EditPen, Document, Collection, MagicStick, UserFilled, Camera,
  Edit, View, Download, Delete, Notebook, DataLine, Files, ChatLineRound, Key
} from '@element-plus/icons-vue';
import AnimatedNumber from '@/components/AnimatedNumber.vue';
import DashboardCharts from '@/components/DashboardCharts.vue';
import RichTextEditor from '@/components/RichTextEditor.vue';
import CollaboratorsPanel from '@/components/collab/CollaboratorsPanel.vue';

// 多人协作：作品级协作者管理面板（docs/novel-collaboration.md）
const collabVisible = ref(false);
const collabNovelId = ref(0);
function openCollab(novelId: number) {
  collabNovelId.value = novelId;
  collabVisible.value = true;
}

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();

// 进入「我的博客」管理页：/author/blog/{用户名}（数字主键兼容由页面处理）
function goMyBlogs() {
  const u = authStore.user;
  if (u?.username) {
    router.push(`/author/blog/${u.username}`);
  } else {
    router.push('/author/blog/new');
  }
}

const novels = ref<Novel[]>([]);
const loadingNovels = ref(false);
const exportingId = ref(0);

// 被封禁（176号令）的小说/章节及原因
const bannedContent = ref<{ novels: any[]; chapters: any[] }>({ novels: [], chapters: [] });
async function loadBannedContent() {
  try {
    const res = await novelApi.getBannedContent();
    if (res.data.code === 0) {
      const d = res.data.data || {};
      bannedContent.value = {
        novels: Array.isArray(d.novels) ? d.novels : [],
        chapters: Array.isArray(d.chapters) ? d.chapters : [],
      };
    }
  } catch { /* ignore */ }
}

// 回收站（用户自删内容，可恢复；管理员封禁的不可自行恢复）
const recycle = ref<{ novels: any[]; blogs: any[] }>({ novels: [], blogs: [] });
async function loadRecycle() {
  try {
    const res = await novelApi.getRecycle();
    if (res.data.code === 0) {
      const d = res.data.data || {};
      recycle.value = {
        novels: Array.isArray(d.novels) ? d.novels : [],
        blogs: Array.isArray(d.blogs) ? d.blogs : [],
      };
    }
  } catch { /* ignore */ }
}
async function restoreRecycle(type: string, id: number) {
  try {
    await ElMessageBox.confirm('确定恢复这项内容吗？', '恢复确认', {
      confirmButtonText: '恢复', type: 'warning',
    });
    await novelApi.restoreRecycle({ type, id });
    ElMessage.success('已恢复');
    loadRecycle();
    loadNovels();
    loadMyBlogs();
  } catch { /* 取消 */ }
}

// 隔离墙开关
const wallToggling = ref(0);
const sensitiveZones = ref<string[]>([]);

// 博客管理
const myBlogs = ref<any[]>([]);
const loadingBlogs = ref(false);

// 自定义 HTML 管理统计
const htmlStats = ref({ frames: 0, htmls: 0, framePending: 0, htmlPending: 0 });
async function loadHtmlStats() {
  try {
    const [fr, hr] = await Promise.all([frameApi.list(), htmlApi.list()]);
    if (fr.data.code === 0) {
      const frames = fr.data.data || [];
      htmlStats.value.frames = frames.length;
      htmlStats.value.framePending = frames.filter((f: any) => f.review_status === 'pending').length;
    }
    if (hr.data.code === 0) {
      const htmls = hr.data.data || [];
      htmlStats.value.htmls = htmls.length;
      htmlStats.value.htmlPending = htmls.filter((h: any) => h.review_status === 'pending').length;
    }
  } catch { /* ignore */ }
}

// 我创建的论坛（管理：禁言/解禁）+ 建版额度
const myForums = ref<any[]>([]);
const forumQuota = ref<any>(null);

async function loadMyForums() {
  try {
    const res = await forumApi.listMyForums();
    if (res.data.code === 0) {
      const d = res.data.data || {};
      // 后端返回 { list, quota }；兼容历史裸数组
      myForums.value = Array.isArray(d) ? d : (d.list || []);
      forumQuota.value = Array.isArray(d) ? null : (d.quota || null);
    }
  } catch { /* ignore */ }
}

async function copyRoomCode(code: string) {
  try {
    await navigator.clipboard.writeText(code);
    ElMessage.success('房间号已复制：' + code);
  } catch {
    ElMessage.warning('复制失败，请手动复制：' + code);
  }
}

function formatDate(d?: string) { if (!d) return ''; return new Date(d).toLocaleString('zh-CN'); }
async function loadMyBlogs() {
  if (!authStore.user) return;
  loadingBlogs.value = true;
  try {
    const res = await blogApi.listByAuthor(authStore.user.id);
    if (res.data.code === 0) { myBlogs.value = res.data.data.list || []; }
  } catch { /* ignore */ }
  finally { loadingBlogs.value = false; }
}
async function deleteMyBlog(id: number) {
  try { await blogApi.delete(id); ElMessage.success('已删除'); loadMyBlogs(); }
  catch { ElMessage.error('删除失败'); }
}

// 统计数据
const statsCards = reactive([
  { label: '作品总数', value: 0 },
  { label: '总字数', value: 0 },
  { label: '总章节', value: 0 },
  { label: '总评论', value: 0 },
]);

const statCardDefs = computed(() => [
  { label: '作品总数', value: statsCards[0].value, icon: Notebook, gradient: 'var(--gradient-blue)' },
  { label: '总字数', value: statsCards[1].value, icon: DataLine, gradient: 'var(--gradient-purple)' },
  { label: '总章节', value: statsCards[2].value, icon: Files, gradient: 'var(--gradient-teal)' },
  { label: '总评论', value: statsCards[3].value, icon: ChatLineRound, gradient: 'var(--gradient-amber)' },
]);

// 图表数据
const chapterTrend = ref<any>({
  title: '章节增长趋势（近7天）',
  dates: [],
  values: [],
  seriesName: '新增章节',
});

const novelWordBars = ref<any>({
  title: '各作品字数对比',
  labels: [],
  values: [],
  seriesName: '字数',
});

const commentTrend = ref<any>({
  title: '评论趋势（近7天）',
  dates: [],
  values: [],
  seriesName: '评论数',
});

const wordCountTrend = ref<any>({
  title: '字数增长趋势（近7天）',
  dates: [],
  values: [],
  seriesName: '新增字数',
});

const bookshelfCount = ref(0);
const totalViews = ref(0);
const avgRating = ref<number | null>(null);

const bookshelfBarData = computed(() => ({
  title: '总收藏',
  labels: ['收藏'],
  values: [bookshelfCount.value],
  seriesName: '收藏数',
}));

const viewsBarData = computed(() => ({
  title: '总阅读量',
  labels: ['阅读'],
  values: [totalViews.value],
  seriesName: '阅读量',
}));

// ── 多指标选项（统一 metrics 格式）──

// 趋势图：章节增长 / 字数增长 / 评论趋势
const authorTrendMetrics = computed(() => [
  { label: '章节增长趋势', data: chapterTrend.value },
  { label: '字数增长趋势', data: wordCountTrend.value },
  { label: '评论趋势', data: commentTrend.value },
]);

// 柱状/趋势图：作品字数 / 作品章节（可切换折线/柱状）
const novelBarMetrics = computed(() => {
  const wordsData = { ...novelWordBars.value };
  const chaptersData = {
    title: '各作品章节数对比',
    labels: novelWordBars.value.labels,
    values: novels.value.map(n => n.total_chapters || 0),
    seriesName: '章节数',
  };
  return [
    { label: '各作品字数', data: wordsData },
    { label: '各作品章节数', data: chaptersData },
  ];
});

// 柱状/趋势图：收藏 / 阅读
const statBarMetrics = computed(() => [
  { label: '总收藏', data: bookshelfBarData.value },
  { label: '总阅读', data: viewsBarData.value },
]);

const exportFormats: Record<string, { label: string; ext: string; mime: string; fn: (id: number) => Promise<any> }> = {
  zip: { label: 'ZIP 打包', ext: 'zip', mime: 'application/zip', fn: novelApi.exportNovel },
  epub: { label: 'EPUB 电子书', ext: 'epub', mime: 'application/epub+zip', fn: novelApi.exportNovelEPUB },
  docx: { label: 'Word 文档', ext: 'docx', mime: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', fn: novelApi.exportNovelDOCX },
  md: { label: '合并 Markdown', ext: 'md', mime: 'text/markdown', fn: novelApi.exportNovelMarkdown },
  txt: { label: '纯文本 TXT', ext: 'txt', mime: 'text/plain', fn: novelApi.exportNovelTXT },
};

async function loadDashboard() {
  try {
    const res = await adminApi.getAuthorDashboard();
    if (res.data.code === 0) {
      const d = res.data.data;

      if (d.stats) {
        statsCards[0].value = d.stats.novels || 0;
        statsCards[1].value = d.stats.total_words || 0;
        statsCards[2].value = d.stats.total_chapters || 0;
        statsCards[3].value = d.stats.total_comments || 0;
      }

      if (d.chapter_trend) {
        chapterTrend.value = {
          title: '章节增长趋势（近7天）',
          dates: d.chapter_trend.dates || [],
          values: d.chapter_trend.counts || [],
          seriesName: '新增章节',
        };
      }

      if (d.comment_trend) {
        commentTrend.value = {
          title: '评论趋势（近7天）',
          dates: d.comment_trend.dates || [],
          values: d.comment_trend.counts || [],
          seriesName: '评论数',
        };
      }

      if (d.word_count_trend) {
        wordCountTrend.value = {
          title: '字数增长趋势（近7天）',
          dates: d.word_count_trend.dates || [],
          values: d.word_count_trend.counts || [],
          seriesName: '每日新增',
          secondValues: d.word_count_trend.cumulative || [],
          secondName: '累计总字数',
        };
      }

      bookshelfCount.value = d.bookshelf_count || 0;
      totalViews.value = d.total_views || 0;

      if (d.avg_rating !== undefined) {
        avgRating.value = d.avg_rating;
      }
    }
  } catch {
    // fallback
  }
}

async function loadNovels() {
  loadingNovels.value = true;
  try {
    const res = await novelApi.getMyNovels();
    novels.value = res.data.data || [];

    // Build chart data from novels
    novelWordBars.value = {
      title: '各作品字数对比',
      labels: novels.value.map(n => n.title.length > 8 ? n.title.slice(0, 7) + '…' : n.title),
      values: novels.value.map(n => n.total_words || 0),
      seriesName: '字数',
    };

    // If dashboard didn't provide stats, compute from novels
    if (statsCards[0].value === 0) {
      statsCards[0].value = novels.value.length;
      statsCards[1].value = novels.value.reduce((sum, n) => sum + (n.total_words || 0), 0);
      statsCards[2].value = novels.value.reduce((sum, n) => sum + (n.total_chapters || 0), 0);
    }

    // Use bookshelf data from API; fallback to 0
  } catch (e) {
    console.error(e);
  } finally {
    loadingNovels.value = false;
  }
}

async function handleExport(id: number, format: string = 'zip') {
  const fmt = exportFormats[format] || exportFormats.zip;
  exportingId.value = id;
  try {
    const res = await fmt.fn.call(novelApi, id);
    const blob = new Blob([res.data], { type: fmt.mime });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `novel_${id}.${fmt.ext}`;
    a.click();
    URL.revokeObjectURL(url);
    ElMessage.success(`${fmt.label}导出成功`);
  } catch {
    ElMessage.error(`${fmt.label}导出失败`);
  } finally {
    exportingId.value = 0;
  }
}

async function handleDelete(novel: Novel) {
  try {
    await ElMessageBox.confirm(`确定要删除《${novel.title}》吗？此操作不可恢复！`, '删除确认', {
      confirmButtonText: '确认删除',
      cancelButtonText: '取消',
      type: 'warning',
    });
    await novelApi.deleteNovel(novel.id);
    ElMessage.success('已删除');
    await loadNovels();
  } catch { /* cancelled */ }
}

function matchZone(cat: string): boolean {
  if (sensitiveZones.value.includes(cat)) return true;
  for (const zone of sensitiveZones.value) {
    if (cat.includes(zone) || (zone.includes(cat) && cat.length >= 2)) return true;
  }
  return false;
}

function isSensitiveCategory(novel: any): boolean {
  const cats = novel.categories?.length ? novel.categories : (novel.category ? [novel.category] : []);
  return cats.some((c: string) => matchZone(c));
}

async function toggleWall(novel: any, enabled: boolean) {
  wallToggling.value = novel.id;
  try {
    await authApi.put('/author/profile', { wall_enabled: enabled, novel_id: novel.id });
    novel.wall_enabled = enabled;
    ElMessage.success(enabled ? '隔离确认已启用' : '隔离确认已关闭');
  } catch (e: any) {
    ElMessage.error('操作失败');
  }
  wallToggling.value = 0;
}

async function loadSensitiveZones() {
  try {
    const res = await authApi.get('/site-info');
    const d = res.data?.data;
    // 必须 wall_enabled=true 且 wall_zones 存在才启用隔离墙
    if (d?.wall_enabled && Array.isArray(d.wall_zones)) {
      sensitiveZones.value = d.wall_zones;
    }
  } catch {}
}

// ===== API Key 管理（作者工作台直接创建 / 管理） =====
const myKeys = ref<any[]>([]);
const loadingKeys = ref(false);
const roleLabel = ref('读者');
const roleQuota = reactive({ req: 60, window: 60 });
const createForm = reactive({ name: '', quota: 0, allowedIps: '', expiresAt: '' });
const creatingKey = ref(false);
const showNewKey = ref(false);
const newKeyValue = ref('');
const newKeyPrefix = ref('');

function formatTime(s?: string) {
  if (!s) return '—';
  return new Date(s).toLocaleString('zh-CN');
}

async function loadMyKeys() {
  loadingKeys.value = true;
  try {
    const res = await apiKeyApi.list();
    const d = res.data.data || {};
    myKeys.value = d.list || [];
    const rl: Record<string, string> = { admin: '站长', author: '作者', reader: '读者' };
    roleLabel.value = rl[d.role] || '读者';
    roleQuota.req = d.role_quota?.req || 60;
    roleQuota.window = d.role_quota?.window || 60;
  } catch { /* ignore */ }
  loadingKeys.value = false;
}

async function createKey() {
  if (createForm.quota < 1) createForm.quota = roleQuota.req;
  creatingKey.value = true;
  try {
    const res = await apiKeyApi.create({
      name: createForm.name,
      quota_per_min: createForm.quota,
      allowed_ips: createForm.allowedIps,
      expires_at: createForm.expiresAt || undefined,
    });
    if (res.data.code === 0) {
      newKeyValue.value = res.data.data.key || '';
      newKeyPrefix.value = res.data.data.key_prefix || '';
      showNewKey.value = true;
      ElMessage.success('API Key 已创建');
      createForm.name = '';
      createForm.allowedIps = '';
      createForm.expiresAt = '';
      await loadMyKeys();
    } else {
      ElMessage.error(res.data.message || '创建失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '创建失败');
  }
  creatingKey.value = false;
}

async function copyKey() {
  try {
    await navigator.clipboard.writeText(newKeyValue.value);
    ElMessage.success('已复制');
  } catch {
    ElMessage.warning('复制失败，请手动选择复制');
  }
}

async function toggleKey(row: any) {
  try {
    await apiKeyApi.update(row.id, { is_active: !row.is_active });
    ElMessage.success('已更新状态');
    await loadMyKeys();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function regenerateKey(row: any) {
  try {
    await ElMessageBox.confirm('重新生成后旧 Key 立即失效，确定继续？', '重新生成', { type: 'warning' });
    const res = await apiKeyApi.regenerate(row.id);
    if (res.data.code === 0) {
      newKeyValue.value = res.data.data.key || '';
      newKeyPrefix.value = res.data.data.key_prefix || '';
      showNewKey.value = true;
      ElMessage.success('已重新生成');
      await loadMyKeys();
    }
  } catch { /* cancelled */ }
}

async function deleteKey(row: any) {
  try {
    await ElMessageBox.confirm(`确定删除 API Key「${row.key_prefix}」吗？删除后立即失效。`, '删除 API Key', { type: 'warning' });
    await apiKeyApi.remove(row.id);
    ElMessage.success('已删除');
    await loadMyKeys();
  } catch { /* cancelled */ }
}

onMounted(() => {
  loadDashboard();
  loadNovels();
  loadSensitiveZones();
  loadMyBlogs();
  loadMyForums();
  loadHtmlStats();
  loadBannedContent();
  loadRecycle();
  loadMyKeys();
});

// 从编辑页面返回时自动刷新
let lastPath = route.fullPath;
watch(
  () => route.fullPath,
  (newPath) => {
    if (newPath === '/author' && lastPath !== '/author') {
      loadDashboard();
      loadNovels();
      loadMyBlogs();
      loadHtmlStats();
      loadBannedContent();
      loadRecycle();
      loadMyKeys();
    }
    lastPath = newPath;
  }
);
</script>

<style scoped>
/* ===== 全局背景 ===== */
.dashboard-bg {
  min-height: 100vh;
  background: linear-gradient(135deg, #0f172a 0%, #1e293b 40%, #0f172a 100%);
  position: relative;
}

[data-theme="light"] .dashboard-bg {
  background: linear-gradient(135deg, #e2e8f0 0%, #f1f5f9 40%, #e2e8f0 100%);
}

.dashboard-bg::before {
  content: '';
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background:
    radial-gradient(ellipse at 20% 20%, rgba(139, 92, 246, 0.08) 0%, transparent 50%),
    radial-gradient(ellipse at 80% 60%, rgba(99, 102, 241, 0.06) 0%, transparent 50%),
    radial-gradient(ellipse at 50% 80%, rgba(245, 158, 11, 0.04) 0%, transparent 50%);
  pointer-events: none;
  z-index: 0;
}

[data-theme="light"] .dashboard-bg::before {
  background:
    radial-gradient(ellipse at 20% 20%, rgba(139, 92, 246, 0.04) 0%, transparent 50%),
    radial-gradient(ellipse at 80% 60%, rgba(99, 102, 241, 0.03) 0%, transparent 50%),
    radial-gradient(ellipse at 50% 80%, rgba(245, 158, 11, 0.02) 0%, transparent 50%);
}

.dashboard-container {
  max-width: 1280px;
  margin: 0 auto;
  padding: 40px 24px 60px;
  position: relative;
  z-index: 1;
}

/* ===== Hero ===== */
.dashboard-hero {
  text-align: center;
  margin-bottom: 40px;
}

.dashboard-main-title {
  font-size: 2.4rem;
  font-weight: 800;
  margin: 0 0 8px;
  line-height: 1.3;
}

.title-glow {
  background: linear-gradient(135deg, #a78bfa, #f59e0b, #34d399);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  filter: drop-shadow(0 0 18px rgba(167, 139, 250, 0.35));
}

.dashboard-subtitle {
  color: var(--text-light);
  font-size: 0.95rem;
  margin: 0 0 16px;
}

.create-btn {
  background: linear-gradient(135deg, #818cf8, #6366f1) !important;
  border: none !important;
  color: #fff !important;
  font-weight: 600 !important;
  padding: 12px 28px !important;
  border-radius: 12px !important;
  box-shadow: 0 4px 20px rgba(99, 102, 241, 0.35);
}

.create-btn:hover {
  box-shadow: 0 6px 28px rgba(99, 102, 241, 0.5) !important;
  transform: translateY(-2px);
}

/* ===== 统计卡片 ===== */
.stats-row {
  margin-bottom: 24px;
}

.glass-stat-card {
  background: rgba(255, 255, 255, 0.06);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 16px;
  padding: 24px 20px;
  display: flex;
  align-items: center;
  gap: 16px;
  transition: transform 0.3s ease, box-shadow 0.3s ease, border-color 0.3s ease;
}

[data-theme="light"] .glass-stat-card {
  background: #fff;
  border-color: var(--border-color);
  box-shadow: var(--shadow-card);
}

.glass-stat-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3);
  border-color: rgba(255, 255, 255, 0.2);
}

[data-theme="light"] .glass-stat-card:hover {
  box-shadow: var(--shadow-lg);
  border-color: var(--border-color);
}

.glass-stat-icon {
  width: 52px;
  height: 52px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  flex-shrink: 0;
}

.glass-stat-body {
  flex: 1;
  min-width: 0;
}

.glass-stat-value :deep(.animated-number-value) {
  background: linear-gradient(135deg, #e2e8f0, #f1f5f9);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  font-size: 2rem;
}

[data-theme="light"] .glass-stat-value :deep(.animated-number-value) {
  background: var(--gradient-primary);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.glass-stat-label {
  font-size: 0.82rem;
  color: rgba(255, 255, 255, 0.5);
  margin-top: 2px;
}

[data-theme="light"] .glass-stat-label {
  color: var(--text-light);
}

/* ===== 图表行 ===== */
.chart-row {
  margin-bottom: 24px;
}

/* ===== 作品列表 ===== */
.novels-section {
  margin-top: 8px;
}

.section-title-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

[data-theme="light"] .section-title-bar {
  border-bottom-color: var(--border-color);
}

.title-bar-text {
  font-size: 1.25rem;
  font-weight: 700;
  color: #e2e8f0;
}

[data-theme="light"] .title-bar-text {
  color: var(--text-color);
}

.title-bar-count {
  font-size: 0.85rem;
  color: rgba(255, 255, 255, 0.4);
  background: rgba(255, 255, 255, 0.06);
  padding: 2px 10px;
  border-radius: 12px;
}

[data-theme="light"] .title-bar-count {
  color: var(--text-light);
  background: var(--bg-color);
}

/* ===== 封禁内容（176号令） ===== */
.banned-section { margin-top: 32px; }
.banned-list { display: flex; flex-direction: column; gap: 12px; }

/* ===== 回收站 ===== */
.recycle-list { display: flex; flex-direction: column; gap: 10px; }
.recycle-item {
  display: flex; align-items: center; gap: 10px;
  padding: 12px 16px; border-radius: 10px;
  background: rgba(245, 158, 11, 0.06);
  border: 1px solid rgba(245, 158, 11, 0.3);
  flex-wrap: wrap;
}
.recycle-title { font-weight: 600; color: #e2e8f0; flex: 1; min-width: 120px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
[data-theme="light"] .recycle-title { color: var(--text-color); }
.recycle-meta { font-size: 0.78rem; color: var(--text-light, #999); white-space: nowrap; }
.forum-admin-list { display: flex; flex-direction: column; gap: 10px; }
.forum-admin-item {
  display: flex; align-items: center; justify-content: space-between; gap: 10px;
  padding: 12px 16px; border-radius: 10px;
  background: var(--bg-card, #fff); border: 1px solid var(--glass-border, #e5e7eb);
}
.forum-admin-main { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; min-width: 0; }
.forum-admin-name { font-weight: 600; color: var(--text-color, #333); font-size: 0.95rem; }
.forum-admin-meta { font-size: 0.78rem; color: var(--text-light, #999); }
.banned-item {
  padding: 14px 16px;
  border-radius: 10px;
  border: 1px solid rgba(239, 68, 68, 0.35);
  background: rgba(239, 68, 68, 0.08);
}
.banned-head { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; }
.banned-title { font-weight: 600; color: #e2e8f0; }
[data-theme="light"] .banned-title { color: var(--text-color); }
.banned-reason { font-size: 0.88rem; color: #fca5a5; line-height: 1.5; }
[data-theme="light"] .banned-reason { color: #b91c1c; }
.banned-label { font-weight: 600; }
.banned-time { font-size: 0.78rem; color: rgba(255,255,255,0.45); margin-top: 4px; }
[data-theme="light"] .banned-time { color: var(--text-light); }

/* ===== 作品卡片 ===== */
.novel-card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 20px;
}

.glass-novel-card {
  border-radius: 16px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  background: rgba(255, 255, 255, 0.05);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border: 1px solid rgba(255, 255, 255, 0.08);
  transition: transform 0.3s ease, box-shadow 0.3s ease, border-color 0.3s ease;
}

[data-theme="light"] .glass-novel-card {
  background: #fff;
  border-color: var(--border-color);
  box-shadow: var(--shadow-card);
}

.glass-novel-card:hover {
  transform: translateY(-6px);
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.35);
  border-color: rgba(255, 255, 255, 0.15);
}

[data-theme="light"] .glass-novel-card:hover {
  box-shadow: var(--shadow-lg);
  border-color: var(--border-color);
}

.novel-card-cover {
  position: relative;
  height: 200px;
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.15), rgba(139, 92, 246, 0.1));
  overflow: hidden;
}

.cover-img {
  width: 100%;
  height: 100%;
}

.cover-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: rgba(255, 255, 255, 0.2);
}

[data-theme="light"] .cover-placeholder {
  color: rgba(0, 0, 0, 0.15);
}

.novel-status-badge {
  position: absolute;
  top: 12px;
  right: 12px;
  padding: 4px 12px;
  border-radius: 20px;
  font-size: 0.75rem;
  font-weight: 600;
  color: #fff;
  backdrop-filter: blur(6px);
}

.novel-status-badge.published {
  background: rgba(39, 174, 96, 0.85);
}

.novel-status-badge.draft {
  background: rgba(149, 165, 166, 0.85);
}

.novel-status-badge.hidden {
  background: rgba(231, 76, 60, 0.85);
}

.novel-card-body {
  padding: 16px;
  flex: 1;
}

.novel-card-title {
  font-size: 1.1rem;
  font-weight: 600;
  color: #e2e8f0;
  margin: 0 0 10px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

[data-theme="light"] .novel-card-title {
  color: var(--text-color);
}

.novel-card-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-bottom: 10px;
}

.cat-tag {
  font-size: 0.75rem;
}

.novel-card-meta {
  display: flex;
  gap: 16px;
  font-size: 0.85rem;
  color: rgba(255, 255, 255, 0.45);
}

[data-theme="light"] .novel-card-meta {
  color: var(--text-light);
}

.novel-card-meta span {
  display: flex;
  align-items: center;
  gap: 4px;
}

.novel-card-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 12px 16px;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
}

[data-theme="light"] .novel-card-actions {
  border-top-color: var(--border-color);
}

/* 博客管理列表 */
.blog-list-dash { display: flex; flex-direction: column; gap: 6px; }
.blog-item-dash {
  display: flex; align-items: center; justify-content: space-between;
  padding: 10px 14px; background: rgba(255,255,255,0.04); border-radius: 8px;
  border: 1px solid rgba(255,255,255,0.06);
}
.blog-item-main { display: flex; align-items: center; gap: 8px; flex: 1; min-width: 0; }
.blog-item-title-dash { font-weight: 500; font-size: 0.9rem; }
.blog-item-meta { font-size: 0.75rem; color: #999; white-space: nowrap; }
.blog-item-actions { display: flex; gap: 4px; flex-shrink: 0; }
[data-theme="light"] .blog-item-dash {
  background: #fafafa; border-color: #eee;
}

/* 博客 Cherry 编辑器 */
.blog-cherry-container { min-height: 360px; }

/* 自定义 HTML 管理入口 */
.html-mgr-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}
.html-mgr-card {
  display: flex;
  gap: 16px;
  align-items: flex-start;
  padding: 20px;
  background: rgba(255,255,255,0.05);
  border: 1px solid rgba(255,255,255,0.08);
  border-radius: 14px;
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s, border-color 0.2s;
}
.html-mgr-card:hover {
  transform: translateY(-3px);
  box-shadow: 0 8px 24px rgba(0,0,0,0.25);
  border-color: rgba(255,255,255,0.16);
}
[data-theme="light"] .html-mgr-card {
  background: #fff;
  border-color: var(--border-color);
  box-shadow: var(--shadow-card);
}
[data-theme="light"] .html-mgr-card:hover {
  box-shadow: var(--shadow-lg);
}
.hmc-icon { font-size: 1.8rem; line-height: 1; }
.hmc-body { flex: 1; min-width: 0; }
.hmc-title { font-weight: 600; font-size: 1rem; color: #e2e8f0; }
[data-theme="light"] .hmc-title { color: var(--text-color); }
.hmc-desc { font-size: 0.82rem; color: rgba(255,255,255,0.45); margin: 4px 0 8px; }
[data-theme="light"] .hmc-desc { color: var(--text-light); }
.hmc-meta { display: flex; gap: 6px; }

/* API Key 管理面板 */
.apikey-panel {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 14px;
  padding: 20px;
}
[data-theme="light"] .apikey-panel {
  background: #fff;
  border-color: var(--border-color);
  box-shadow: var(--shadow-card);
}
.apikey-hint {
  margin: 4px 0 0;
  font-size: 0.85rem;
  line-height: 1.5;
  color: var(--text-light, #999);
}
.apikey-hint code {
  background: #eef2ff;
  color: #4f46e5;
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 0.82rem;
}
[data-theme="dark"] .apikey-hint code {
  background: rgba(99, 102, 241, 0.15);
  color: #a5b4fc;
}
.apikey-empty {
  text-align: center;
  margin-top: 8px;
  font-size: 0.85rem;
  color: var(--text-light, #999);
}
.apikey-panel :deep(code) {
  background: #eef2ff;
  color: #4f46e5;
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 0.82rem;
}
[data-theme="dark"] .apikey-panel :deep(code) {
  background: rgba(99, 102, 241, 0.15);
  color: #a5b4fc;
}
.apikey-panel .newkey-box { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; }
.apikey-panel .newkey-code { flex: 1; background: #0f172a; color: #4ade80; padding: 10px 12px; border-radius: 8px; font-size: 0.82rem; word-break: break-all; }
[data-theme="light"] .apikey-panel .newkey-code { background: #f1f5f9; color: #15803d; }

/* ====== 移动端作者工作台适配 ====== */
@media (max-width: 768px) {
  .dashboard-container {
    padding: 20px 12px 40px;
  }
  .dashboard-hero {
    margin-bottom: 24px;
  }
  .dashboard-main-title {
    font-size: 1.5rem;
  }
  .dashboard-subtitle {
    font-size: 0.85rem;
  }
  .hero-buttons {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 8px;
  }
  .hero-buttons .el-button {
    flex: 1;
    min-width: 120px;
    margin-left: 0 !important;
  }
  /* 博客/论坛管理列表：行式改纵向 */
  .blog-item-dash,
  .forum-admin-item {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
  .blog-item-actions {
    width: 100%;
    justify-content: flex-end;
  }
  .blog-item-main {
    flex-wrap: wrap;
  }
  .blog-item-meta {
    white-space: normal;
  }
  /* 作品卡片网格：窄屏单列 */
  .novel-card-grid {
    grid-template-columns: 1fr;
    gap: 14px;
  }
  .html-mgr-grid {
    grid-template-columns: 1fr;
    gap: 12px;
  }
  .stats-row .el-col,
  .chart-row .el-col {
    margin-bottom: 14px;
  }
  .stats-row .glass-stat-card {
    padding: 14px;
  }
}
</style>