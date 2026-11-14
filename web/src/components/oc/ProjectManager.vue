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
      <el-button type="primary" size="small" @click="openEditor()">＋ 立项</el-button>
    </div>

    <!-- 项目卡片 -->
    <div class="opm-grid">
      <div v-for="p in projects" :key="p.id" class="opm-card">
        <div class="opm-cover">
          <img v-if="p.cover_url" :src="p.cover_url" :alt="p.title" loading="lazy" />
          <span v-else class="opm-cover-ph">🗺️</span>
          <el-tag size="small" :type="visTagType(p.visibility)" effect="dark" class="opm-vis">
            {{ visLabel(p.visibility) }}
          </el-tag>
        </div>
        <div class="opm-body">
          <div class="opm-title">{{ p.title }}</div>
          <div class="opm-meta">
            <span v-if="p.category" class="opm-cat">{{ p.category }}</span>
            <span class="opm-dim">世界 {{ p.world_count }} · 成员 {{ p.member_count }}</span>
          </div>
          <div class="opm-summary">{{ p.summary || '（无简介）' }}</div>
          <div class="opm-foot">
            <span class="opm-owner">{{ p.owner_name || '—' }}</span>
            <span class="opm-actions">
              <el-button size="small" text type="primary" @click="openDetail(p)">进入</el-button>
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
      <el-empty v-if="!loading && projects.length === 0" description="还没有项目，点「＋ 立项」开一个世界观" :image-size="60" />
    </div>

    <!-- 新建 / 编辑 -->
    <el-dialog v-model="editorVisible" :title="editing ? '项目设置' : '立项：新建项目 / 世界'" width="520px">
      <el-form :model="form" label-width="88px" size="default">
        <el-form-item label="项目名称" required>
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
    <el-dialog v-model="pwdVisible" :title="`输入「${detail?.title || ''}」的访问密码`" width="400px">
      <el-input v-model="pwd" type="password" show-password placeholder="访问密码" @keyup.enter="submitPassword" />
      <template #footer>
        <el-button @click="pwdVisible = false">取消</el-button>
        <el-button type="primary" :loading="pwdLoading" @click="submitPassword">进入</el-button>
      </template>
    </el-dialog>

    <!-- 项目详情 -->
    <el-drawer v-model="drawerVisible" :title="detail?.title || '项目'" size="46%">
      <div v-if="detail" class="opm-detail">
        <div class="opm-dhead">
          <img v-if="detail.cover_url" :src="detail.cover_url" class="opm-dcover" alt="" />
          <div>
            <div class="opm-dtitle">
              {{ detail.title }}
              <el-tag size="small" :type="visTagType(detail.visibility)">{{ visLabel(detail.visibility) }}</el-tag>
              <el-tag v-if="detail.category" size="small" type="info">{{ detail.category }}</el-tag>
            </div>
            <div class="opm-dsub">{{ detail.summary || '（无简介）' }}</div>
          </div>
        </div>

        <!-- 成员 -->
        <section class="opm-sec">
          <div class="opm-sec-h">成员（{{ members.length }}）</div>
          <div class="opm-mrow" v-for="m in members" :key="m.id">
            <el-avatar :size="28" :src="m.avatar_url || ''">{{ (m.nickname || m.username || '?')[0] }}</el-avatar>
            <span class="opm-mname">{{ m.nickname || m.username || m.user_id }}</span>
            <el-tag size="small" :type="m.role === 'owner' ? 'warning' : 'info'">{{ roleLabel(m.role) }}</el-tag>
            <el-button
              v-if="detail.user_id === myId && m.user_id !== myId"
              size="small" text type="danger" @click="removeMember(m)"
            >移除</el-button>
          </div>
          <div v-if="detail.user_id === myId" class="opm-madd">
            <el-input v-model="memberName" placeholder="用户名或邮箱" size="small" style="width: 170px" />
            <el-select v-model="memberRole" size="small" style="width: 100px">
              <el-option label="编辑" value="editor" />
              <el-option label="只读" value="viewer" />
            </el-select>
            <el-button size="small" type="primary" :loading="memberLoading" @click="addMember">添加</el-button>
          </div>
        </section>

        <!-- 世界 -->
        <section class="opm-sec">
          <div class="opm-sec-h">世界（{{ worlds.length }}）</div>
          <div class="opm-wrow" v-for="w in worlds" :key="w.id">
            <div class="opm-winfo">
              <div class="opm-wtitle">{{ w.title }}</div>
              <div class="opm-wsub">{{ w.summary || '（无简介）' }}</div>
            </div>
            <el-button size="small" text @click="openBookLinks(w)">跨世界链接</el-button>
            <el-button size="small" text type="danger" @click="detachWorld(w)">移出</el-button>
          </div>
          <div class="opm-wadd">
            <el-input v-model="worldTitle" placeholder="新建世界名称" size="small" style="flex:1" />
            <el-button size="small" type="primary" :loading="worldLoading" @click="createWorld">在项目内新建世界</el-button>
          </div>
          <div class="opm-wadd">
            <el-select v-model="attachId" placeholder="把已有世界加入项目" size="small" style="flex:1">
              <el-option v-for="b in candidateBooks" :key="b.id" :label="b.title" :value="b.id" />
            </el-select>
            <el-button size="small" :loading="worldLoading" :disabled="!attachId" @click="attachWorld">加入</el-button>
          </div>
        </section>

        <!-- 跨世界链接 -->
        <section class="opm-sec" v-if="linkBook">
          <div class="opm-sec-h">
            跨世界链接：{{ linkBook.title }}
            <el-button size="small" text type="primary" @click="linkBook = null">收起</el-button>
          </div>
          <div class="opm-lrow" v-for="l in links" :key="l.id">
            <span class="opm-llabel">{{ l.label || '关联' }}</span>
            <span class="opm-ltext">
              {{ l.from_book_id === linkBook.id ? l.from_title : l.to_title }}
              ↔ {{ l.from_book_id === linkBook.id ? l.to_title : l.from_title }}
            </span>
            <el-button size="small" text type="danger" @click="removeLink(l)">删除</el-button>
          </div>
          <el-empty v-if="links.length === 0" description="还没有链接" :image-size="40" />
          <div class="opm-ladd">
            <el-select v-model="linkTarget" placeholder="链接到世界" size="small" style="flex:1">
              <el-option v-for="b in linkCandidates" :key="b.id" :label="b.title" :value="b.id" />
            </el-select>
            <el-input v-model="linkLabel" placeholder="标签（如：平行世界）" size="small" style="width: 130px" />
            <el-button size="small" type="primary" :loading="linkLoading" :disabled="!linkTarget" @click="addLink">建立链接</el-button>
          </div>
        </section>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import { ocApi, ocProjectApi, type OcProject, type OcProjectMember, type OcWorldBook, type OcWorldLink } from '@/api/oc';
import { useAuthStore } from '@/stores/auth';

const CATEGORIES = ['原创世界', 'furry 兽人', '现代都市', '奇幻', '科幻', '古风', '武侠', '末日', '校园', '其他'];

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

const drawerVisible = ref(false);
const detail = ref<OcProject | null>(null);
const members = ref<OcProjectMember[]>([]);
const worlds = ref<OcWorldBook[]>([]);
const candidateBooks = ref<OcWorldBook[]>([]);
const memberName = ref('');
const memberRole = ref('editor');
const memberLoading = ref(false);
const worldTitle = ref('');
const worldLoading = ref(false);
const attachId = ref<number | null>(null);

const linkBook = ref<OcWorldBook | null>(null);
const links = ref<OcWorldLink[]>([]);
const linkCandidates = ref<OcWorldBook[]>([]);
const linkTarget = ref<number | null>(null);
const linkLabel = ref('');
const linkLoading = ref(false);

function tokenKey(id: number) { return `oc_project_token_${id}`; }

function visLabel(v: string) {
  return v === 'public' ? '公开' : v === 'password' ? '密码公开' : '私有';
}
function visTagType(v: string) {
  return v === 'public' ? 'success' : v === 'password' ? 'warning' : 'info';
}
function roleLabel(r: string) {
  return r === 'owner' ? '负责人' : r === 'viewer' ? '只读' : '编辑';
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
    projects.value = res.data?.data?.list || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载项目失败');
  } finally {
    loading.value = false;
  }
}

function openEditor(p?: OcProject) {
  editing.value = p || null;
  form.value = p
    ? { title: p.title, summary: p.summary || '', cover_url: p.cover_url || '', category: p.category || '', visibility: p.visibility, password: '' }
    : { title: '', summary: '', cover_url: '', category: '', visibility: 'private', password: '' };
  editorVisible.value = true;
}

async function saveProject() {
  if (!form.value.title.trim()) { ElMessage.warning('请填写项目名称'); return; }
  if (form.value.visibility === 'password' && !editing.value && form.value.password.length < 6) {
    ElMessage.warning('访问密码至少 6 位'); return;
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
    if (editing.value) await ocProjectApi.update(editing.value.id, payload);
    else await ocProjectApi.create(payload);
    editorVisible.value = false;
    ElMessage.success('已保存');
    await load();
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

// ── 详情（含密码项目访问） ──
async function openDetail(p: OcProject) {
  detail.value = p;
  drawerVisible.value = true;
  members.value = [];
  worlds.value = [];
  linkBook.value = null;
  links.value = [];
  const token = localStorage.getItem(tokenKey(p.id)) || '';
  try {
    await ocProjectApi.get(p.id, token || undefined);
    await loadDetailData();
  } catch (e: any) {
    if (p.visibility === 'password') {
      pwdVisible.value = true;
    } else {
      ElMessage.error(e?.response?.data?.message || '无法打开项目');
      drawerVisible.value = false;
    }
  }
}

async function submitPassword() {
  if (!detail.value) return;
  pwdLoading.value = true;
  try {
    const res = await ocProjectApi.access(detail.value.id, pwd.value);
    const token = res.data?.data?.token;
    if (token) localStorage.setItem(tokenKey(detail.value.id), token);
    pwdVisible.value = false;
    pwd.value = '';
    await loadDetailData();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '密码错误');
  } finally {
    pwdLoading.value = false;
  }
}

async function loadDetailData() {
  if (!detail.value) return;
  const id = detail.value.id;
  const token = localStorage.getItem(tokenKey(id)) || undefined;
  try {
    const [m, w, mine] = await Promise.all([
      ocProjectApi.members(id),
      ocProjectApi.worlds(id),
      ocApi.listBooks({ scope: 'usable', page_size: 50 }),
    ]);
    members.value = m.data?.data?.list || [];
    worlds.value = w.data?.data?.list || [];
    candidateBooks.value = (mine.data?.data?.list || []).filter((b: OcWorldBook) => !worlds.value.some((x) => x.id === b.id));
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载项目内容失败');
  }
}

async function addMember() {
  if (!detail.value || !memberName.value.trim()) return;
  memberLoading.value = true;
  try {
    await ocProjectApi.addMember(detail.value.id, memberName.value.trim(), memberRole.value);
    memberName.value = '';
    await loadDetailData();
    ElMessage.success('已添加成员');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '添加失败');
  } finally {
    memberLoading.value = false;
  }
}

async function removeMember(m: OcProjectMember) {
  if (!detail.value) return;
  try {
    await ocProjectApi.removeMember(detail.value.id, m.user_id);
    await loadDetailData();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '移除失败');
  }
}

async function createWorld() {
  if (!detail.value || !worldTitle.value.trim()) return;
  worldLoading.value = true;
  try {
    await ocApi.createBook({ title: worldTitle.value.trim(), summary: '', visibility: 'private', project_id: detail.value.id });
    worldTitle.value = '';
    await loadDetailData();
    ElMessage.success('已创建世界');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '创建失败');
  } finally {
    worldLoading.value = false;
  }
}

async function attachWorld() {
  if (!detail.value || !attachId.value) return;
  worldLoading.value = true;
  try {
    await ocProjectApi.attachWorld(detail.value.id, attachId.value);
    attachId.value = null;
    await loadDetailData();
    ElMessage.success('已加入项目');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加入失败');
  } finally {
    worldLoading.value = false;
  }
}

async function detachWorld(w: OcWorldBook) {
  if (!detail.value) return;
  try {
    await ocProjectApi.detachWorld(detail.value.id, w.id);
    if (linkBook.value?.id === w.id) linkBook.value = null;
    await loadDetailData();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '移出失败');
  }
}

// ── 跨世界链接 ──
async function openBookLinks(w: OcWorldBook) {
  linkBook.value = w;
  linkTarget.value = null;
  linkLabel.value = '';
  try {
    const res = await ocProjectApi.listLinks(w.id);
    links.value = res.data?.data?.list || [];
    const mine = await ocApi.listBooks({ scope: 'usable', page_size: 50 });
    linkCandidates.value = (mine.data?.data?.list || []).filter((b: OcWorldBook) => b.id !== w.id);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载链接失败');
  }
}

async function addLink() {
  if (!linkBook.value || !linkTarget.value) return;
  linkLoading.value = true;
  try {
    await ocProjectApi.addLink(linkBook.value.id, linkTarget.value, linkLabel.value.trim());
    linkTarget.value = null;
    linkLabel.value = '';
    await openBookLinks(linkBook.value);
    ElMessage.success('已建立跨世界链接');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '建立链接失败');
  } finally {
    linkLoading.value = false;
  }
}

async function removeLink(l: OcWorldLink) {
  try {
    await ocProjectApi.removeLink(l.id);
    await openBookLinks(linkBook.value!);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

onMounted(load);
</script>

<style scoped>
.opm-bar { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-bottom: 14px; }
.opm-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 14px; min-height: 120px; }
.opm-card { background: var(--surface-color, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: 10px; overflow: hidden; display: flex; flex-direction: column; }
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

.opm-detail { display: flex; flex-direction: column; gap: 18px; }
.opm-dhead { display: flex; gap: 12px; align-items: flex-start; }
.opm-dcover { width: 72px; height: 72px; object-fit: cover; border-radius: 8px; }
.opm-dtitle { font-weight: 600; display: flex; gap: 6px; align-items: center; flex-wrap: wrap; }
.opm-dsub { font-size: 0.82rem; color: var(--text-muted, #6b7280); margin-top: 4px; }
.opm-sec { border-top: 1px dashed var(--border-color, #e5e7eb); padding-top: 12px; }
.opm-sec-h { font-weight: 600; margin-bottom: 10px; display: flex; justify-content: space-between; align-items: center; }
.opm-mrow { display: flex; align-items: center; gap: 8px; padding: 6px 0; }
.opm-mname { flex: 1; font-size: 0.88rem; }
.opm-madd, .opm-wadd, .opm-ladd { display: flex; gap: 8px; margin-top: 8px; }
.opm-wrow { display: flex; align-items: center; gap: 8px; padding: 8px 0; border-bottom: 1px solid var(--fill-color, #f3f4f6); }
.opm-winfo { flex: 1; min-width: 0; }
.opm-wtitle { font-size: 0.9rem; font-weight: 500; }
.opm-wsub { font-size: 0.78rem; color: var(--text-muted, #6b7280); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.opm-lrow { display: flex; align-items: center; gap: 8px; padding: 6px 0; font-size: 0.85rem; }
.opm-llabel { color: var(--primary-color, #409eff); }
.opm-ltext { flex: 1; color: var(--text-muted, #6b7280); }
</style>
