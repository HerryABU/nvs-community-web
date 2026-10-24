<template>
  <el-drawer
    :model-value="visible"
    title="章节版本（分支 / 历史 / 合并）"
    size="72%"
    @update:model-value="(v: boolean) => emit('update:visible', v)"
  >
    <div class="vp-wrap" v-loading="loading">
      <!-- 分支栏 -->
      <div class="vp-branch-bar">
        <el-select v-model="branch" size="small" style="width: 200px" @change="load">
          <el-option v-for="b in branches" :key="b.name" :label="`${b.name}（v${b.head_ver}）`" :value="b.name" />
        </el-select>
        <el-button size="small" @click="branchDialog = true">开分支</el-button>
        <el-button size="small" type="warning" @click="mergeDialog = true" :disabled="branches.length < 2">
          合并分支
        </el-button>
        <span class="vp-tip">
          当前 head：v{{ head }} · 打开编辑器时记下版本号，保存会带上进行冲突检测
        </span>
      </div>

      <!-- 版本时间线 -->
      <div class="vp-list">
        <div v-for="v in versions" :key="v.id" class="vp-item" :class="{ head: v.is_head }">
          <div class="vp-item-head">
            <span class="vp-ver">v{{ v.version }}</span>
            <el-tag v-if="v.is_head" size="small" type="success">head</el-tag>
            <el-tag v-if="v.force" size="small" type="danger">强制覆盖</el-tag>
            <el-tag v-if="v.merge_from" size="small" type="warning">合并 {{ v.merge_from }} v{{ v.merge_from_ver }}</el-tag>
            <el-tag v-if="v.deleted" size="small" type="info">正文已清理</el-tag>
            <span class="vp-author">{{ v.author_name || `用户${v.author_id}` }}</span>
            <span class="vp-time">{{ formatTime(v.created_at) }}</span>
            <span class="vp-words">{{ v.word_count }} 字</span>
          </div>
          <div class="vp-msg">{{ v.message || '（无说明）' }}</div>
          <div class="vp-actions">
            <el-button size="small" text @click="openDiff(v.version, prevVer(v.version))">与上一版对比</el-button>
            <el-button size="small" text @click="viewVersion(v)">查看内容</el-button>
            <el-button size="small" text type="primary" :disabled="v.is_head" @click="doRevert(v)">回滚到此版</el-button>
            <el-button
              size="small"
              text
              type="danger"
              :disabled="v.is_head || v.deleted"
              @click="doDelete(v)"
            >删除快照</el-button>
          </div>
        </div>
        <el-empty v-if="versions.length === 0" description="该分支还没有版本" :image-size="60" />
      </div>

      <!-- 并排 diff -->
      <div v-if="diffData" class="vp-diff">
        <div class="vp-diff-head">
          <span>v{{ diffFrom }} → v{{ diffTo }}</span>
          <span class="vp-tip">+{{ diffData.added }} / -{{ diffData.removed }}</span>
          <el-button size="small" text @click="diffData = null">收起</el-button>
        </div>
        <div class="vp-diff-body">
          <div
            v-for="(l, i) in diffData.lines"
            :key="i"
            class="vp-diff-row"
            :class="l.type"
          >
            <span class="vp-no">{{ l.left_no || '' }}</span>
            <span class="vp-no">{{ l.right_no || '' }}</span>
            <span class="vp-text">{{ l.text || ' ' }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 开分支 -->
    <el-dialog v-model="branchDialog" title="开分支" width="420px" append-to-body>
      <el-form label-width="90px">
        <el-form-item label="分支名">
          <el-input v-model="newBranch" placeholder="如 rewrite / draft-b" maxlength="32" />
        </el-form-item>
        <el-form-item label="基于">
          <el-select v-model="branchFrom" style="width: 100%">
            <el-option v-for="b in branches" :key="b.name" :label="`${b.name}（v${b.head_ver}）`" :value="b.name" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="branchDialog = false">取消</el-button>
        <el-button type="primary" :loading="busy" @click="doCreateBranch">创建</el-button>
      </template>
    </el-dialog>

    <!-- 合并 -->
    <el-dialog v-model="mergeDialog" title="合并分支" width="520px" append-to-body>
      <el-form label-width="90px">
        <el-form-item label="来源分支">
          <el-select v-model="mergeFrom" style="width: 100%">
            <el-option v-for="b in branches" :key="b.name" :label="`${b.name}（v${b.head_ver}）`" :value="b.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="合并到">
          <el-select v-model="mergeInto" style="width: 100%">
            <el-option v-for="b in branches" :key="b.name" :label="`${b.name}（v${b.head_ver}）`" :value="b.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model="mergeMsg" placeholder="合并说明（可选）" maxlength="60" />
        </el-form-item>
      </el-form>
      <div v-if="mergeConflicts.length" class="vp-merge-conflicts">
        <el-alert type="warning" :closable="false" title="存在冲突片段，已按「目标分支」内容暂存，请人工确认后手动调整正文" />
        <div v-for="(c, i) in mergeConflicts" :key="i" class="vp-conflict-block">
          <div><b>原文</b>：{{ c.base.join(' / ') || '（空）' }}</div>
          <div><b>目标分支</b>：{{ c.ours.join(' / ') || '（空）' }}</div>
          <div><b>来源分支</b>：{{ c.theirs.join(' / ') || '（空）' }}</div>
        </div>
      </div>
      <template #footer>
        <el-button @click="mergeDialog = false">关闭</el-button>
        <el-button type="primary" :loading="busy" @click="doMerge">合并</el-button>
      </template>
    </el-dialog>

    <!-- 查看历史版本内容 -->
    <el-dialog v-model="viewVisible" :title="`v${viewVer} 内容`" width="720px" append-to-body>
      <pre class="vp-view-content">{{ viewContent || '（该快照正文已被清理）' }}</pre>
    </el-dialog>
  </el-drawer>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import { ElMessage } from 'element-plus';
import { vcsApi, type ChapterVersion, type ChapterBranchVO, type DiffResult, type MergeConflict } from '@/api/novel/vcs';

const props = defineProps<{ visible: boolean; novelId: number; chapterNum: number }>();
const emit = defineEmits<{ 'update:visible': [boolean] }>();

const loading = ref(false);
const busy = ref(false);
const branch = ref('main');
const branches = ref<ChapterBranchVO[]>([]);
const versions = ref<ChapterVersion[]>([]);
const head = ref(0);

const diffData = ref<DiffResult | null>(null);
const diffFrom = ref(0);
const diffTo = ref(0);

const branchDialog = ref(false);
const newBranch = ref('');
const branchFrom = ref('main');

const mergeDialog = ref(false);
const mergeFrom = ref('');
const mergeInto = ref('main');
const mergeMsg = ref('');
const mergeConflicts = ref<MergeConflict[]>([]);

const viewVisible = ref(false);
const viewVer = ref(0);
const viewContent = ref('');

function formatTime(t: string) {
  if (!t) return '';
  return String(t).replace('T', ' ').slice(0, 19);
}

function prevVer(v: number) {
  return v > 1 ? v - 1 : 1;
}

async function load() {
  if (!props.novelId || !props.chapterNum) return;
  loading.value = true;
  try {
    const res = await vcsApi.listVersions(props.novelId, props.chapterNum, branch.value);
    const d = res.data.data || ({} as any);
    versions.value = d.list || [];
    branches.value = d.branches || [];
    head.value = d.head || 0;
    branch.value = d.branch || 'main';
    if (branches.value.length > 0) {
      branchFrom.value = d.branch || 'main';
      if (!mergeInto.value) mergeInto.value = 'main';
      const other = branches.value.find((b) => b.name !== 'main');
      if (other && !mergeFrom.value) mergeFrom.value = other.name;
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载版本失败');
  } finally {
    loading.value = false;
  }
}

async function openDiff(from: number, to: number) {
  if (from === to) {
    ElMessage.info('该版本与上一版内容一致');
    return;
  }
  loading.value = true;
  try {
    const res = await vcsApi.diff(props.novelId, props.chapterNum, from, to, branch.value);
    diffData.value = res.data.data?.diff || null;
    diffFrom.value = from;
    diffTo.value = to;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '对比失败');
  } finally {
    loading.value = false;
  }
}

async function viewVersion(v: ChapterVersion) {
  viewVer.value = v.version;
  try {
    const res = await vcsApi.getVersion(props.novelId, props.chapterNum, v.version, branch.value);
    viewContent.value = res.data.data?.content || '';
  } catch {
    viewContent.value = '';
  }
  viewVisible.value = true;
}

async function doRevert(v: ChapterVersion) {
  busy.value = true;
  try {
    await vcsApi.revert(props.novelId, props.chapterNum, v.version, branch.value);
    ElMessage.success('已回滚并生成新版本');
    await load();
    emit('update:visible', false);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '回滚失败');
  } finally {
    busy.value = false;
  }
}

async function doDelete(v: ChapterVersion) {
  busy.value = true;
  try {
    await vcsApi.deleteVersion(props.novelId, props.chapterNum, v.version, branch.value);
    ElMessage.success('已删除该历史快照（作者与提交记录保留）');
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  } finally {
    busy.value = false;
  }
}

async function doCreateBranch() {
  const name = newBranch.value.trim();
  if (!name) {
    ElMessage.warning('请输入分支名');
    return;
  }
  busy.value = true;
  try {
    await vcsApi.createBranch(props.novelId, props.chapterNum, { name, from_branch: branchFrom.value });
    ElMessage.success('已创建分支 ' + name);
    branchDialog.value = false;
    newBranch.value = '';
    branch.value = name;
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '创建分支失败');
  } finally {
    busy.value = false;
  }
}

async function doMerge() {
  busy.value = true;
  mergeConflicts.value = [];
  try {
    const res = await vcsApi.merge(props.novelId, props.chapterNum, {
      from_branch: mergeFrom.value,
      into_branch: mergeInto.value,
      message: mergeMsg.value,
    });
    const d: any = res.data?.data || {};
    if (d.conflict) {
      mergeConflicts.value = d.conflicts || [];
      ElMessage.warning('合并存在冲突，请人工确认');
    } else {
      ElMessage.success('已合并并生成新版本');
      mergeDialog.value = false;
      branch.value = mergeInto.value;
      await load();
    }
  } catch (e: any) {
    // 409 冲突：后端回传冲突片段
    const d = e?.response?.data?.data;
    if (d?.conflicts) {
      mergeConflicts.value = d.conflicts;
      ElMessage.warning('合并存在冲突，请人工确认');
    } else {
      ElMessage.error(e?.response?.data?.message || '合并失败');
    }
  } finally {
    busy.value = false;
  }
}

watch(
  () => props.visible,
  (v) => {
    if (v) load();
  },
);
watch(() => props.chapterNum, () => { if (props.visible) load(); });
</script>

<style scoped>
.vp-wrap {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.vp-branch-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.vp-tip {
  font-size: 0.75rem;
  color: var(--text-muted);
}
.vp-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 42vh;
  overflow-y: auto;
}
.vp-item {
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 10px 12px;
  background: var(--bg-card);
}
.vp-item.head {
  border-color: var(--el-color-success);
}
.vp-item-head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.vp-ver {
  font-weight: 700;
  font-size: 0.95rem;
}
.vp-author,
.vp-time,
.vp-words {
  font-size: 0.78rem;
  color: var(--text-muted);
}
.vp-msg {
  font-size: 0.85rem;
  margin: 4px 0 6px;
  word-break: break-all;
}
.vp-actions {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}
.vp-diff {
  border: 1px solid var(--border-color);
  border-radius: 8px;
  overflow: hidden;
}
.vp-diff-head {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 10px;
  background: var(--bg-soft);
  font-size: 0.85rem;
}
.vp-diff-body {
  max-height: 34vh;
  overflow-y: auto;
  font-family: ui-monospace, Consolas, monospace;
  font-size: 0.8rem;
}
.vp-diff-row {
  display: grid;
  grid-template-columns: 48px 48px 1fr;
  gap: 6px;
  padding: 1px 8px;
  white-space: pre-wrap;
  word-break: break-all;
}
.vp-diff-row.add {
  background: rgba(34, 197, 94, 0.14);
}
.vp-diff-row.del {
  background: rgba(239, 68, 68, 0.14);
}
.vp-no {
  color: var(--text-muted);
  text-align: right;
  user-select: none;
}
.vp-merge-conflicts {
  margin-top: 8px;
}
.vp-conflict-block {
  border: 1px dashed var(--el-color-warning);
  border-radius: 6px;
  padding: 8px 10px;
  margin-top: 8px;
  font-size: 0.82rem;
  line-height: 1.7;
}
.vp-view-content {
  max-height: 60vh;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
  font-family: inherit;
  line-height: 1.8;
}
</style>
