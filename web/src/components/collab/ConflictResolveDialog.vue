<template>
  <el-dialog
    :model-value="visible"
    title="章节已被他人更新"
    width="80%"
    :close-on-click-modal="false"
    :close-on-press-escape="false"
    @update:model-value="(v: boolean) => emit('update:visible', v)"
  >
    <el-alert type="warning" :closable="false" show-icon class="cf-alert">
      <template #title>
        你基于 v{{ conflict?.base_version }} 编辑，服务端已是 v{{ conflict?.server_version }}（他人已改过本章）。
        请选择「以我的为准」覆盖，或「放弃修改」重载服务端内容。任一选择都会留下版本记录。
      </template>
    </el-alert>

    <div class="cf-cols">
      <div class="cf-col">
        <div class="cf-col-head">我的修改（未保存）</div>
        <div class="cf-body">
          <div v-for="(l, i) in mineLines" :key="'m' + i" class="cf-line" :class="{ hit: mineChanged.has(l) }">
            <span class="cf-no">{{ i + 1 }}</span><span class="cf-text">{{ l || ' ' }}</span>
          </div>
        </div>
      </div>
      <div class="cf-col">
        <div class="cf-col-head">服务端最新 v{{ conflict?.server_version }}</div>
        <div class="cf-body">
          <div v-for="(l, i) in serverLines" :key="'s' + i" class="cf-line" :class="{ hit: serverChanged.has(l) }">
            <span class="cf-no">{{ i + 1 }}</span><span class="cf-text">{{ l || ' ' }}</span>
          </div>
        </div>
      </div>
    </div>

    <details class="cf-base">
      <summary>查看共同基线 v{{ conflict?.base_version }}（点击展开）</summary>
      <pre>{{ conflict?.base_content || '（无）' }}</pre>
    </details>

    <template #footer>
      <el-button @click="emit('resolve', 'reload')">放弃我的修改，重载服务端</el-button>
      <el-button type="danger" @click="emit('resolve', 'mine')">以我的为准（强制覆盖）</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import type { ConflictPayload } from '@/api/novel/vcs';

const props = defineProps<{ visible: boolean; conflict: ConflictPayload | null }>();
const emit = defineEmits<{ 'update:visible': [boolean]; resolve: ['mine' | 'reload'] }>();

function toLines(s: string): string[] {
  return (s || '').replace(/\r\n/g, '\n').split('\n');
}

const mineLines = computed(() => toLines(props.conflict?.mine_content || ''));
const serverLines = computed(() => toLines(props.conflict?.server_content || ''));

// 与基线不同的行做高亮（用基线做参照，突出「这版到底改了什么」）
const baseSet = computed(() => new Set(toLines(props.conflict?.base_content || '')));
const mineChanged = computed(() => new Set(mineLines.value.filter((l) => !baseSet.value.has(l))));
const serverChanged = computed(() => new Set(serverLines.value.filter((l) => !baseSet.value.has(l))));
</script>

<style scoped>
.cf-alert {
  margin-bottom: 12px;
}
.cf-cols {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
.cf-col {
  border: 1px solid var(--border-color);
  border-radius: 8px;
  overflow: hidden;
}
.cf-col-head {
  padding: 6px 10px;
  background: var(--bg-soft);
  font-size: 0.85rem;
  font-weight: 600;
}
.cf-body {
  max-height: 46vh;
  overflow-y: auto;
  font-family: ui-monospace, Consolas, monospace;
  font-size: 0.8rem;
}
.cf-line {
  display: grid;
  grid-template-columns: 44px 1fr;
  gap: 6px;
  padding: 1px 8px;
  white-space: pre-wrap;
  word-break: break-all;
}
.cf-line.hit {
  background: rgba(239, 68, 68, 0.14);
}
.cf-no {
  color: var(--text-muted);
  text-align: right;
  user-select: none;
}
.cf-base {
  margin-top: 12px;
  font-size: 0.8rem;
  color: var(--text-muted);
}
.cf-base pre {
  max-height: 200px;
  overflow: auto;
  white-space: pre-wrap;
  background: var(--bg-soft);
  padding: 8px;
  border-radius: 6px;
}

@media (max-width: 768px) {
  .cf-cols {
    grid-template-columns: 1fr;
  }
}
</style>
