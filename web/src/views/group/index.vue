<template>
  <div class="page-container gpage">
    <div class="ghead">
      <div>
        <h1 class="page-title">群组</h1>
        <p class="gsub">围绕作品、作者与话题的半私密空间：群号加入、群公告、群文件、群内 @ai</p>
      </div>
      <div class="ghead-actions">
        <el-button v-if="auth.isLoggedIn" type="primary" @click="openCreate">
          <el-icon><Plus /></el-icon> 创建群组
        </el-button>
      </div>
    </div>

    <!-- 加入：群号 / 扫码 -->
    <div class="join-bar">
      <el-input
        v-model="joinNo"
        placeholder="输入群号或邀请码加入，如 1234567"
        clearable
        style="max-width: 280px"
        @keyup.enter="joinByNo"
      />
      <el-button :loading="joining" @click="joinByNo">加入</el-button>
      <el-button @click="scanVisible = true">📷 扫码 / 图片识别</el-button>
      <span class="join-tip">图片仅本地解析，不上传服务器</span>
    </div>

    <el-tabs v-model="tab" @tab-change="onTab">
      <el-tab-pane label="发现群组" name="discover" />
      <el-tab-pane v-if="auth.isLoggedIn" label="我的群组" name="my" />
    </el-tabs>

    <div v-if="tab === 'discover'" class="filter-row">
      <el-select v-model="category" clearable placeholder="全部分类" size="small" style="width:150px" @change="load">
        <el-option v-for="c in categories" :key="c" :label="c" :value="c" />
      </el-select>
      <el-input v-model="keyword" size="small" placeholder="搜索群名 / 简介" clearable style="width:220px" @keyup.enter="load" />
      <el-button size="small" @click="load">搜索</el-button>
    </div>

    <div v-loading="loading" class="ggrid">
      <div v-for="g in list" :key="g.id" class="gcard" @click="enter(g)">
        <div class="gcard-top">
          <span class="gcard-name">
            {{ g.name }}
            <span v-if="(g.unread || 0) > 0" class="gcard-unread">{{ g.unread! > 99 ? '99+' : g.unread }}</span>
          </span>
          <el-tag size="small" :type="visTag(g.visibility)" effect="plain">{{ visLabel(g.visibility) }}</el-tag>
        </div>
        <p class="gcard-desc">{{ g.description || '暂无简介' }}</p>
        <div class="gcard-meta">
          <span>群号 {{ g.group_no }}</span>
          <span>{{ g.member_count }} 人</span>
          <span>{{ g.post_count }} 帖</span>
        </div>
        <div class="gcard-foot">
          <span v-if="g.category" class="gcard-cat">{{ g.category }}</span>
          <span v-if="g.my_status === 'pending'" class="gcard-pending">待审核</span>
          <span v-else-if="g.is_member" class="gcard-joined">{{ roleLabel(g.my_role) }}</span>
        </div>
      </div>
      <EmptyState v-if="!loading && !list.length" icon="👥" text="还没有群组，试试创建一个" />
    </div>

    <div v-if="total > size" class="gpager">
      <el-pagination layout="prev, pager, next" :total="total" :page-size="size" :current-page="page" @current-change="(p: number) => { page = p; load(); }" />
    </div>

    <!-- 创建群组 -->
    <el-dialog v-model="createVisible" title="创建群组" width="560px">
      <el-form label-width="88px">
        <el-form-item label="群组名" required>
          <el-input v-model="form.name" maxlength="32" show-word-limit placeholder="2~32 个字符" />
        </el-form-item>
        <el-form-item label="简介">
          <el-input v-model="form.description" type="textarea" :rows="3" maxlength="500" show-word-limit placeholder="这个群讨论什么、面向谁" />
        </el-form-item>
        <el-form-item label="分类">
          <el-input v-model="form.category" maxlength="32" placeholder="如 读书会 / 创作互助 / 科幻迷" />
        </el-form-item>
        <el-form-item label="可见性">
          <el-radio-group v-model="form.visibility">
            <el-radio-button value="public">公开</el-radio-button>
            <el-radio-button value="semi">半公开</el-radio-button>
            <el-radio-button value="private">私密</el-radio-button>
          </el-radio-group>
          <div class="fhint">半公开：能看到群存在，内容仅成员；私密：仅成员可见</div>
        </el-form-item>
        <el-form-item label="加入方式">
          <el-radio-group v-model="form.join_mode">
            <el-radio-button value="open">直接加入</el-radio-button>
            <el-radio-button value="apply">申请审核</el-radio-button>
            <el-radio-button value="invite">邀请码</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="人数上限">
          <el-input-number v-model="form.member_limit" :min="2" :max="10000" controls-position="right" />
          <span class="fhint" style="margin-left:8px">受站长全局上限约束</span>
        </el-form-item>
        <el-form-item label="成员可拉人">
          <el-switch v-model="form.allow_member_invite" />
          <span class="fhint" style="margin-left:8px">关闭后只有群主/管理员能邀请</span>
        </el-form-item>
        <el-form-item label="先审后发">
          <el-switch v-model="form.review_first" />
          <span class="fhint" style="margin-left:8px">私密/半公开默认开启</span>
        </el-form-item>
        <el-form-item label="群内 @ai">
          <el-switch v-model="form.ai_enabled" />
          <span class="fhint" style="margin-left:8px">开启后成员 @ai 可召唤智能体（消耗发起人额度）</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="submitCreate">创建</el-button>
      </template>
    </el-dialog>

    <!-- 创建成功：展示群二维码 -->
    <el-dialog v-model="createdVisible" title="群组已创建" width="460px">
      <GroupQrCard v-if="createdGroup" :group="createdGroup" :invite-code="createdGroup.invite_code" />
      <template #footer>
        <el-button @click="createdVisible = false">稍后</el-button>
        <el-button type="primary" @click="goCreated">进入群组</el-button>
      </template>
    </el-dialog>

    <QrScanDialog v-model="scanVisible" @result="onScanResult" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import { Plus } from '@element-plus/icons-vue';
import { groupApi, type Group } from '@/api/group';
import { useAuthStore } from '@/stores/auth';
import { parseGroupQr } from '@/utils/groupQr';
import GroupQrCard from '@/components/group/GroupQrCard.vue';
import QrScanDialog from '@/components/group/QrScanDialog.vue';
import EmptyState from '@/components/common/EmptyState.vue';

const router = useRouter();
const auth = useAuthStore();

const tab = ref<'discover' | 'my'>('discover');
const list = ref<Group[]>([]);
const categories = ref<string[]>([]);
const total = ref(0);
const page = ref(1);
const size = ref(20);
const loading = ref(false);
const category = ref('');
const keyword = ref('');

const joinNo = ref('');
const joining = ref(false);
const scanVisible = ref(false);

const createVisible = ref(false);
const createdVisible = ref(false);
const creating = ref(false);
const createdGroup = ref<Group | null>(null);
const form = reactive({
  name: '', description: '', category: '',
  visibility: 'public', join_mode: 'open',
  member_limit: 200, allow_member_invite: true, review_first: false, ai_enabled: false,
});

function visLabel(v: string) {
  return v === 'semi' ? '半公开' : v === 'private' ? '私密' : '公开';
}
function visTag(v: string) {
  return v === 'semi' ? 'warning' : v === 'private' ? 'info' : 'success';
}
function roleLabel(r?: string) {
  return r === 'owner' ? '群主' : r === 'admin' ? '管理员' : '成员';
}

async function load() {
  loading.value = true;
  try {
    const res = await groupApi.list({
      scope: tab.value === 'my' ? 'my' : 'discover',
      category: category.value || undefined,
      q: keyword.value || undefined,
      page: page.value,
      size: size.value,
    });
    if (res.data.code === 0) {
      list.value = res.data.data.list || [];
      total.value = res.data.data.total || 0;
      categories.value = res.data.data.categories || [];
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  }
  loading.value = false;
}

function onTab() {
  page.value = 1;
  load();
}

function enter(g: Group) {
  router.push(`/group/${g.id}`);
}

async function joinByNo() {
  const key = joinNo.value.trim();
  if (!key) return;
  if (!auth.isLoggedIn) {
    ElMessage.warning('请先登录');
    router.push('/login');
    return;
  }
  joining.value = true;
  try {
    const res = await groupApi.joinByNo({ group_no: key });
    if (res.data.code === 0) {
      ElMessage.success(res.data.message || '已加入');
      router.push(`/group/${res.data.data.group.id}`);
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加入失败');
  }
  joining.value = false;
}

function onScanResult(text: string) {
  const payload = parseGroupQr(text);
  if (!payload.no) {
    ElMessage.warning('二维码内容无法识别为群组信息');
    return;
  }
  joinNo.value = payload.no;
  ElMessage.success(`已识别：${payload.no}`);
  joinByNo();
}

function openCreate() {
  if (!auth.isLoggedIn) {
    ElMessage.warning('请先登录');
    router.push('/login');
    return;
  }
  createVisible.value = true;
}

async function submitCreate() {
  if (!form.name.trim()) {
    ElMessage.warning('请填写群组名');
    return;
  }
  creating.value = true;
  try {
    const res = await groupApi.create({ ...form });
    if (res.data.code === 0) {
      createVisible.value = false;
      createdGroup.value = res.data.data.group;
      createdVisible.value = true;
      await load();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '创建失败');
  }
  creating.value = false;
}

function goCreated() {
  createdVisible.value = false;
  if (createdGroup.value) router.push(`/group/${createdGroup.value.id}`);
}

onMounted(load);
</script>

<style scoped>
.gpage { max-width: 1080px; margin: 0 auto; }
.ghead { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.gsub { margin: 6px 0 14px; font-size: 0.84rem; color: var(--text-light, #64748b); }
.ghead-actions { display: flex; gap: 8px; }
.join-bar {
  display: flex; align-items: center; gap: 8px; flex-wrap: wrap;
  padding: 12px 14px; border-radius: 12px;
  background: var(--fill-light, #f8fafc); border: 1px solid var(--border-color, #e5e7eb);
}
.join-tip { font-size: 0.72rem; color: var(--text-light, #94a3b8); }
.filter-row { display: flex; align-items: center; gap: 8px; margin: 4px 0 12px; flex-wrap: wrap; }
.ggrid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 14px; min-height: 120px; }
.gcard {
  border: 1px solid var(--border-color, #e5e7eb); border-radius: 12px; padding: 14px;
  background: var(--card-bg, #fff); cursor: pointer; transition: transform 0.15s, box-shadow 0.15s;
  display: flex; flex-direction: column; gap: 8px;
}
.gcard:hover { transform: translateY(-2px); box-shadow: 0 8px 22px rgba(15, 23, 42, 0.08); }
.gcard-top { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.gcard-name { font-weight: 600; font-size: 1rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.gcard-desc {
  margin: 0; font-size: 0.8rem; color: var(--text-light, #64748b); line-height: 1.55;
  display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; min-height: 2.5em;
}
.gcard-meta { display: flex; gap: 10px; font-size: 0.72rem; color: var(--text-light, #94a3b8); flex-wrap: wrap; }
.gcard-foot { display: flex; align-items: center; gap: 8px; margin-top: auto; }
.gcard-cat { font-size: 0.7rem; padding: 1px 8px; border-radius: 999px; background: rgba(64, 158, 255, 0.1); color: #3b82f6; }
.gcard-joined { font-size: 0.72rem; color: #10b981; }
.gcard-unread {
  display: inline-block; margin-left: 6px; min-width: 16px; padding: 0 5px;
  border-radius: 999px; background: #ef4444; color: #fff;
  font-size: 0.64rem; line-height: 16px; text-align: center; vertical-align: middle;
}
.gcard-pending { font-size: 0.72rem; color: #f59e0b; }
.gpager { display: flex; justify-content: center; margin-top: 18px; }
.fhint { font-size: 0.72rem; color: var(--text-light, #94a3b8); }
</style>
