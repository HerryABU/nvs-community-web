<template>
  <div class="opm-wrap" v-loading="loading">
    <!-- 顶部筛选 -->
    <div class="opm-bar">
      <el-radio-group v-model="scope" size="small" @change="load">
        <el-radio-button value="usable">可参与</el-radio-button>
        <el-radio-button value="mine">我的项目</el-radio-button>
        <el-radio-button value="joined">已加入</el-radio-button>
        <el-radio-button value="public">公开库</el-radio-button>
      </el-radio-group>
      <el-select v-model="category" size="small" placeholder="全部类别" clearable style="width: 140px" @change="load">
        <el-option v-for="c in CATEGORIES" :key="c" :label="c" :value="c" />
      </el-select>
      <el-input v-model="keyword" placeholder="搜索项目" size="small" style="width: 180px" clearable @change="load" />
      <el-button type="primary" size="small" @click="openEditor()">＋ 创建世界</el-button>
    </div>

    <!-- 项目卡片 -->
    <div class="opm-grid">
      <div v-for="p in projects" :key="p._federated ? 'fed-' + p.id : 'local-' + p.id" class="opm-card" @click="openProject(p)">
        <div class="opm-cover">
          <img v-if="p.cover_url" :src="mediaUrl(p.cover_url)" :alt="p.title" loading="lazy" />
          <span v-else class="opm-cover-ph">🗺️</span>
          <el-tag size="small" :type="visTagType(p.visibility)" effect="dark" class="opm-vis">
            {{ visLabel(p.visibility) }}
          </el-tag>
        </div>
        <div class="opm-body">
          <div class="opm-title"><span v-if="p._federated" class="fed-badge">🌐 {{ p._site }}</span>{{ p.title }}</div>
          <div class="opm-meta">
            <span v-if="p.category" class="opm-cat">{{ p.category }}</span>
            <span class="opm-dim">世界 {{ p.world_count }} · 成员 {{ p.member_count }}</span>
          </div>
          <div class="opm-summary">{{ p.summary || '（无简介）' }}</div>
          <div class="opm-foot">
            <span class="opm-owner">{{ p.owner_name || '—' }}</span>
            <span class="opm-actions" @click.stop>
              <el-button size="small" text type="primary" @click="enterProject(p)">进入</el-button>
              <template v-if="p.user_id === myId">
                <el-button size="small" text @click="openEditor(p)">设置</el-button>
                <el-popconfirm title="确认删除该项目？（世界会解绑保留）" @confirm="removeProject(p)">
                  <template #reference><el-button size="small" text type="danger">删除</el-button></template>
                </el-popconfirm>
              </template>
            </span>
          </div>
        </div>
      </div>
      <el-empty
        v-if="!loading && projects.length === 0"
        description="还没有世界，点「＋ 创建世界」开一个世界观"
        :image-size="60"
      />
    </div>

    <!-- 新建 / 编辑 -->
    <el-dialog v-model="editorVisible" :title="editing ? '世界设置' : '创建世界'" width="520px">
      <el-form :model="form" label-width="88px" size="default">
        <el-form-item label="世界名称" required>
          <el-input v-model="form.title" maxlength="60" show-word-limit placeholder="例如：铁堡纪元" />
        </el-form-item>
        <el-form-item label="类别">
          <el-select v-model="form.category" placeholder="选择或输入类别" allow-create filterable style="width: 100%">
            <el-option v-for="c in CATEGORIES" :key="c" :label="c" :value="c" />
          </el-select>
        </el-form-item>
        <el-form-item label="简介">
          <el-input v-model="form.summary" type="textarea" :rows="3" maxlength="2000" show-word-limit />
        </el-form-item>
        <el-form-item label="封面 URL">
          <el-input v-model="form.cover_url" placeholder="可选，填图片链接" />
        </el-form-item>
        <el-form-item label="可见性">
          <el-radio-group v-model="form.visibility">
            <el-radio-button value="private">私有（仅成员）</el-radio-button>
            <el-radio-button value="public">公开</el-radio-button>
            <el-radio-button value="password">密码公开</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="form.visibility === 'password'" label="访问密码">
          <el-input v-model="form.password" type="password" show-password placeholder="至少 6 位；留空表示不修改（编辑时）" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editorVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveProject">保存</el-button>
      </template>
    </el-dialog>

    <!-- 密码访问 -->
    <el-dialog v-model="pwdVisible" title="输入访问密码" width="400px">
      <el-input v-model="pwd" type="password" show-password placeholder="访问密码" @keyup.enter="submitPassword" />
      <template #footer>
        <el-button @click="pwdVisible = false">取消</el-button>
        <el-button type="primary" :loading="pwdLoading" @click="submitPassword">进入</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import { ocProjectApi, type OcProject } from '@/api/oc';
import { mediaUrl } from '@/utils/prefix';
import { loadFederated as loadFedItems, mergeFederated, fedHref } from '@/utils/federation';
import { useAuthStore } from '@/stores/auth';

/**
 * 世界列表（亚文化创作区 · 项目/世界）
 *
 * 这里只负责「找到世界 / 创建世界 / 进入世界」。
 * 成员、世界书、角色、形象图库、跨世界链接全部搬到世界详情页（/subculture/world/:id），
 * 不再用抽屉塞在列表里——两套 UI 维护同一批数据只会互相打脸。
 */
const CATEGORIES = ['原创世界', 'furry 兽人', '现代都市', '奇幻', '科幻', '古风', '武侠', '末日', '校园', '其他'];

const router = useRouter();
const auth = useAuthStore();
const myId = computed(() => auth.user?.id || 0);

const projects = ref<OcProject[]>([]);
const loading = ref(false);
const scope = ref('usable');
const category = ref('');
const keyword = ref('');

const editorVisible = ref(false);
const editing = ref<OcProject | null>(null);
const saving = ref(false);
const form = ref({ title: '', summary: '', cover_url: '', category: '', visibility: 'private', password: '' });

const pwdVisible = ref(false);
const pwd = ref('');
const pwdLoading = ref(false);
const pendingId = ref(0);

function tokenKey(id: number) {
  return `oc_project_token_${id}`;
}

function visLabel(v: string) {
  return v === 'public' ? '公开' : v === 'password' ? '密码公开' : '私有';
}
function visTagType(v: string) {
  return v === 'public' ? 'success' : v === 'password' ? 'warning' : 'info';
}

async function load() {
  loading.value = true;
  try {
    const res = await ocProjectApi.list({
      scope: scope.value,
      category: category.value || undefined,
      keyword: keyword.value || undefined,
      page: 1,
      page_size: 50,
    });
    const local = res.data?.data?.list || [];
    const fed = await loadFedItems('subculture', 6);
    projects.value = mergeFederated(local, fed, (f: any) => ({
      id: f.id, title: f.title, name: f.title, summary: f.summary,
      cover_url: f.cover_url, visibility: 'public',
      _federated: true, _site: f.site_name, source_url: fedHref(f),
    })) as any;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载项目失败');
  } finally {
    loading.value = false;
  }
}

function openEditor(p?: OcProject) {
  editing.value = p || null;
  form.value = p
    ? {
        title: p.title,
        summary: p.summary || '',
        cover_url: p.cover_url || '',
        category: p.category || '',
        visibility: p.visibility,
        password: '',
      }
    : { title: '', summary: '', cover_url: '', category: '', visibility: 'private', password: '' };
  editorVisible.value = true;
}

async function saveProject() {
  if (!form.value.title.trim()) {
    ElMessage.warning('请填写世界名称');
    return;
  }
  if (form.value.visibility === 'password' && !editing.value && form.value.password.length < 6) {
    ElMessage.warning('访问密码至少 6 位');
    return;
  }
  saving.value = true;
  try {
    const payload = {
      title: form.value.title,
      summary: form.value.summary,
      cover_url: form.value.cover_url,
      category: form.value.category,
      visibility: form.value.visibility,
      password: form.value.password || undefined,
    };
    if (editing.value) {
      await ocProjectApi.update(editing.value.id, payload);
      editorVisible.value = false;
      ElMessage.success('已保存');
      await load();
    } else {
      // 创建完直接进新世界：立项之后就该待在这个世界里，而不是回到列表
      const r = await ocProjectApi.create(payload);
      const id = r.data?.data?.id;
      editorVisible.value = false;
      ElMessage.success('世界已创建，开始写第一个角色吧');
      if (id) router.push(`/subculture/world/${id}`);
      else await load();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

async function removeProject(p: OcProject) {
  try {
    await ocProjectApi.remove(p.id);
    ElMessage.success('项目已删除');
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

/** 进入世界：密码公开的先验访问码，再跳真实路由 */
async function enterProject(p: OcProject) {
  const token = localStorage.getItem(tokenKey(p.id)) || '';
  try {
    await ocProjectApi.get(p.id, token || undefined);
    router.push(`/subculture/world/${p.id}`);
  } catch (e: any) {
    if (p.visibility === 'password') {
      pendingId.value = p.id;
      pwd.value = '';
      pwdVisible.value = true;
    } else {
      ElMessage.error(e?.response?.data?.message || '无法进入该世界');
    }
  }
}

async function submitPassword() {
  if (!pendingId.value || !pwd.value.trim()) return;
  pwdLoading.value = true;
  try {
    const res = await ocProjectApi.access(pendingId.value, pwd.value);
    const token = res.data?.data?.token;
    if (token) localStorage.setItem(tokenKey(pendingId.value), token);
    pwdVisible.value = false;
    router.push(`/subculture/world/${pendingId.value}`);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '密码错误');
  } finally {
    pwdLoading.value = false;
  }
}

onMounted(load);
</script>

<style scoped>
.opm-bar { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-bottom: 14px; }
.opm-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 14px; min-height: 120px; }
.opm-card {
  background: var(--bg-card, #fff); border: 1px solid var(--border-color, #e5e7eb);
  border-radius: 10px; overflow: hidden; display: flex; flex-direction: column; cursor: pointer;
  transition: transform 0.14s ease, box-shadow 0.14s ease;
}
.opm-card:hover { transform: translateY(-2px); box-shadow: 0 6px 18px rgba(0, 0, 0, 0.08); }
.opm-cover { position: relative; height: 110px; background: var(--fill-color, #f3f4f6); display: flex; align-items: center; justify-content: center; }
.opm-cover img { width: 100%; height: 100%; object-fit: cover; }
.opm-cover-ph { font-size: 2rem; }
.opm-vis { position: absolute; top: 8px; right: 8px; }
.opm-body { padding: 10px 12px; display: flex; flex-direction: column; gap: 6px; flex: 1; }
.opm-title { font-weight: 600; font-size: 0.95rem; }
.opm-meta { display: flex; gap: 8px; align-items: center; font-size: 0.78rem; color: var(--text-muted, #6b7280); }
.opm-cat { color: var(--primary-color, #409eff); }
.opm-summary {
  font-size: 0.82rem; color: var(--text-muted, #6b7280); line-height: 1.4;
  display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden;
}
.opm-foot { display: flex; justify-content: space-between; align-items: center; margin-top: auto; padding-top: 6px; }
.opm-owner { font-size: 0.78rem; color: var(--text-muted, #6b7280); }
.opm-actions { display: flex; gap: 2px; }

.fed-badge, .bl-fed-badge, .th-fed-badge { display:inline-block; margin:0 6px 4px 0; padding:1px 8px; border-radius:999px; background:rgba(99,102,241,.12); color:var(--primary-color,#6366f1); font-size:.72rem; }
</style>
