<template>
  <div v-if="poll" class="poll-card">
    <div class="poll-head">
      <span class="poll-icon">🗳️</span>
      <div class="poll-title">
        <strong>{{ poll.question }}</strong>
        <span class="poll-status" :class="poll.status">{{ poll.status === 'closed' ? '已结束' : '投票中' }}</span>
      </div>
      <el-button v-if="isAuthor && poll.status === 'open'" link type="danger" size="small" @click="closePoll">终止投票</el-button>
    </div>
    <p v-if="poll.description" class="poll-desc">{{ poll.description }}</p>

    <div class="poll-options">
      <label
        v-for="o in options"
        :key="o.id"
        class="poll-option"
        :class="{ picked: mine.includes(o.id), closed: poll.status === 'closed' }"
      >
        <input
          v-if="poll.status === 'open'"
          :type="poll.multiple ? 'checkbox' : 'radio'"
          :name="'poll-' + poll.id"
          :checked="mine.includes(o.id)"
          :disabled="submitting"
          @change="toggle(o.id)"
        />
        <span class="poll-label">{{ o.label }}</span>
        <span class="poll-bar" :style="{ width: pct(o) + '%' }"></span>
        <span class="poll-count">{{ o.votes }} 票 · {{ pct(o) }}%</span>
      </label>
    </div>

    <div class="poll-foot">
      <span>{{ voterCount }} 人参与</span>
      <el-button v-if="poll.status === 'open'" type="primary" size="small" :loading="submitting" @click="submit">投票</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import { useAuthStore } from '@/stores/auth';
import { pollApi } from '@/api/poll';

const props = defineProps<{ id: number; authorId?: number }>();
const auth = useAuthStore();
const poll = ref<any>(null);
const options = ref<any[]>([]);
const mine = ref<number[]>([]);
const voterCount = ref(0);
const submitting = ref(false);
const selected = ref<number[]>([]);
const isAuthor = computed(() => !!auth.user && (auth.user.id === props.authorId || auth.user.id === poll.value?.author_id));

async function load() {
  try {
    const r = await pollApi.get(props.id);
    const d = r.data?.data || {};
    poll.value = d.poll;
    options.value = d.options || [];
    mine.value = d.mine || [];
    selected.value = [...mine.value];
    voterCount.value = d.voter_count || 0;
  } catch { /* 投票不存在则静默隐藏 */ }
}
onMounted(load);

function toggle(id: number) {
  if (poll.value.multiple) {
    selected.value = selected.value.includes(id) ? selected.value.filter(x => x !== id) : [...selected.value, id];
  } else {
    selected.value = [id];
  }
}
function pct(o: any) {
  const total = options.value.reduce((s, x) => s + (x.votes || 0), 0);
  return total ? Math.round((o.votes || 0) * 100 / total) : 0;
}
async function submit() {
  if (!selected.value.length) { ElMessage.warning('请先选择选项'); return; }
  submitting.value = true;
  try {
    await pollApi.vote(props.id, selected.value);
    ElMessage.success('投票成功');
    await load();
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '投票失败'); }
  finally { submitting.value = false; }
}
async function closePoll() {
  try {
    await pollApi.close(props.id);
    ElMessage.success('投票已终止');
    await load();
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '终止失败'); }
}
</script>

<style scoped>
.poll-card { margin: 14px 0; padding: 16px; border: 1px solid var(--border-color, #e5e7eb); border-radius: 12px; background: var(--bg-soft, #f8fafc); }
.poll-head { display: flex; align-items: center; gap: 8px; }
.poll-icon { font-size: 1.2rem; }
.poll-title { flex: 1; display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.poll-status { font-size: 0.7rem; padding: 1px 8px; border-radius: 999px; background: #e0e7ff; color: #4338ca; }
.poll-status.closed { background: #f1f5f9; color: #64748b; }
.poll-desc { font-size: 0.8rem; color: var(--text-light, #888); margin: 6px 0; }
.poll-options { display: flex; flex-direction: column; gap: 6px; margin: 10px 0; }
.poll-option { position: relative; display: flex; align-items: center; gap: 8px; padding: 8px 10px; border: 1px solid var(--border-color, #e5e7eb); border-radius: 8px; overflow: hidden; cursor: pointer; background: var(--bg-card, #fff); }
.poll-option.picked { border-color: var(--primary-color, #6366f1); }
.poll-option.closed { cursor: default; }
.poll-label { position: relative; z-index: 1; flex: 1; }
.poll-bar { position: absolute; left: 0; top: 0; bottom: 0; background: rgba(99,102,241,0.12); z-index: 0; transition: width 0.3s; }
.poll-count { position: relative; z-index: 1; font-size: 0.76rem; color: var(--text-light, #888); }
.poll-foot { display: flex; align-items: center; justify-content: space-between; font-size: 0.78rem; color: var(--text-light, #888); }
[data-theme="dark"] .poll-card { background: #1e293b; border-color: #334155; }
[data-theme="dark"] .poll-option { background: #0f172a; border-color: #334155; }
</style>
