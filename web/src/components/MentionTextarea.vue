<template>
  <div class="mention-textarea">
    <el-input
      ref="inputRef"
      :model-value="modelValue"
      type="textarea"
      :rows="rows"
      :maxlength="maxlength"
      :placeholder="resolvedPlaceholder"
      @update:model-value="onUpdate"
      @input="onInput"
      @keydown="onKeydown"
      @click="onInput"
    />
    <!-- @ 联想下拉 -->
    <div v-if="showDropdown" class="mention-dropdown" ref="dropdownRef">
      <div
        v-for="(c, i) in candidates"
        :key="c.id"
        class="mention-item"
        :class="{ active: i === activeIndex }"
        @mousedown.prevent="selectAt(i)"
        @mouseenter="activeIndex = i"
      >
        <span class="mention-avatar">{{ (c.nickname || c.username || '?').slice(0, 1) }}</span>
        <span class="mention-name">{{ c.nickname || c.username }}</span>
        <span class="mention-username">@{{ c.username }}</span>
      </div>
      <div v-if="candidates.length === 0" class="mention-empty">无匹配用户</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick } from 'vue';
import { ElInput } from 'element-plus';
import { editorKeydownHandler, MARKDOWN_TOKENS } from '@/utils/editorShortcuts';
import { mentionApi } from '@/api/novel/comments';

const props = withDefaults(
  defineProps<{
    modelValue: string;
    placeholder?: string;
    rows?: number;
    maxlength?: number | string;
    source?: string; // participants / global / manual（手动无联想）
    targetType?: string;
    targetID?: number;
  }>(),
  { rows: 3, maxlength: 2000, source: 'participants', targetType: '', targetID: 0 },
);

const emit = defineEmits<{
  'update:modelValue': [v: string];
  save: [];
}>();

const inputRef = ref<InstanceType<typeof ElInput>>();
const dropdownRef = ref<HTMLElement>();

const showDropdown = ref(false);
const candidates = ref<{ id: number; username: string; nickname: string }[]>([]);
const activeIndex = ref(0);
let triggerStart = -1; // @ 符号在文本中的索引
let debounceTimer: ReturnType<typeof setTimeout> | null = null;

const resolvedPlaceholder = computed(() =>
  props.placeholder ||
  '写下你的评论... 支持 Markdown（Ctrl+B 粗体 / Ctrl+I 斜体 / Ctrl+U 下划线 / Ctrl+S 发布），输入 @ 可提及他人',
);

function nativeTextarea(): HTMLTextAreaElement | null {
  return (inputRef.value as any)?.textarea ?? null;
}

function onUpdate(v: string) {
  emit('update:modelValue', v);
}

// 检测光标前是否处于 @触发状态：@ 前为行首或空白，且 @ 与光标间为合法用户名片段（无空格）
function detectTrigger(text: string, caret: number): { start: number; query: string } | null {
  let j = caret - 1;
  while (j >= 0) {
    const ch = text[j];
    if (ch === '@') {
      const prev = j > 0 ? text[j - 1] : ' ';
      if (j === 0 || /\s/.test(prev)) {
        const query = text.slice(j + 1, caret);
        if (/^[A-Za-z0-9_\-一-龥]*$/.test(query)) {
          return { start: j, query };
        }
      }
      return null;
    }
    if (/\s/.test(ch)) return null; // 遇到空白，触发链断开
    j--;
  }
  return null;
}

async function fetchCandidates(q: string) {
  const scope = props.source === 'global' ? 'global' : 'participants';
  try {
    const res = await mentionApi.getCandidates({
      scope,
      target_type: props.targetType,
      target_id: props.targetID,
      q,
      limit: 8,
    });
    candidates.value = res.data?.data || [];
  } catch {
    candidates.value = [];
  }
  activeIndex.value = 0;
}

function onInput() {
  const ta = nativeTextarea();
  if (!ta) return;
  const caret = ta.selectionStart ?? ta.value.length;
  const trig = detectTrigger(ta.value, caret);
  if (!trig || props.source === 'manual') {
    showDropdown.value = false;
    return;
  }
  triggerStart = trig.start;
  showDropdown.value = true;
  if (debounceTimer) clearTimeout(debounceTimer);
  debounceTimer = setTimeout(() => fetchCandidates(trig.query), 200);
}

function selectAt(i: number) {
  const c = candidates.value[i];
  if (!c) return;
  const ta = nativeTextarea();
  const value = props.modelValue;
  const caret = ta ? ta.selectionStart ?? value.length : value.length;
  const before = value.slice(0, triggerStart);
  const after = value.slice(caret);
  const insert = '@' + c.username + ' ';
  const newVal = before + insert + after;
  emit('update:modelValue', newVal);
  showDropdown.value = false;
  // 还原光标到插入文本之后
  nextTick(() => {
    const t = nativeTextarea();
    if (t) {
      const pos = before.length + insert.length;
      t.focus();
      t.setSelectionRange(pos, pos);
    }
  });
}

function move(dir: number) {
  if (candidates.value.length === 0) return;
  activeIndex.value = (activeIndex.value + dir + candidates.value.length) % candidates.value.length;
}

function onKeydown(e: KeyboardEvent) {
  if (showDropdown.value) {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      move(1);
      return;
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault();
      move(-1);
      return;
    }
    if (e.key === 'Enter' || e.key === 'Tab') {
      e.preventDefault();
      selectAt(activeIndex.value);
      return;
    }
    if (e.key === 'Escape') {
      e.preventDefault();
      showDropdown.value = false;
      return;
    }
  }
  // Markdown 快捷 + Ctrl/Cmd+S 拦截浏览器保存并发布
  editorKeydownHandler(e, { tokens: MARKDOWN_TOKENS, onSave: () => emit('save') });
}
</script>

<style scoped>
.mention-textarea {
  position: relative;
}
.mention-dropdown {
  position: absolute;
  left: 0;
  right: 0;
  top: 100%;
  z-index: 20;
  margin-top: 4px;
  background: var(--bg-elevated, #fff);
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 8px;
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.12);
  max-height: 220px;
  overflow-y: auto;
  padding: 4px;
}
.mention-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.88rem;
}
.mention-item.active,
.mention-item:hover {
  background: var(--primary-soft, rgba(99, 102, 241, 0.12));
}
.mention-avatar {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: var(--primary-color, #4f46e5);
  color: #fff;
  font-size: 0.72rem;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.mention-name {
  font-weight: 600;
}
.mention-username {
  color: var(--text-light, #999);
  font-size: 0.78rem;
}
.mention-empty {
  padding: 8px 10px;
  color: var(--text-light, #999);
  font-size: 0.82rem;
}
</style>

<style>
/* @ 提及链接（评论正文里渲染）：高亮可点击 */
.mention-link {
  color: var(--primary-color, #4f46e5);
  font-weight: 600;
  text-decoration: none;
  cursor: pointer;
}
.mention-link:hover {
  text-decoration: underline;
}
[data-theme="dark"] .mention-link {
  color: #93c5fd;
}
</style>
