<template>
  <!-- F19 狂暴模式：实时连接状态小徽标（绿色脉冲 = 实时中；灰色 = 已断开，重试中） -->
  <el-tooltip
    :content="connected ? '实时评论已连接，新评论将自动出现' : '实时连接已断开，正在自动重试'"
    placement="top"
  >
    <span class="live-badge" :class="connected ? 'live-badge--on' : 'live-badge--off'">
      <span class="live-dot" />
      {{ connected ? '实时中' : '已断开，重试中' }}
    </span>
  </el-tooltip>
</template>

<script setup lang="ts">
defineProps<{
  connected: boolean;
}>();
</script>

<style scoped>
.live-badge {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 0.72rem;
  line-height: 1;
  padding: 3px 8px;
  border-radius: 999px;
  user-select: none;
  white-space: nowrap;
}

.live-badge--on {
  color: #16a34a;
  background: rgba(22, 163, 74, 0.1);
}

.live-badge--off {
  color: var(--text-light, #999);
  background: rgba(148, 163, 184, 0.12);
}

.live-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}

.live-badge--on .live-dot {
  background: #22c55e;
  animation: live-pulse 1.6s ease-in-out infinite;
}

.live-badge--off .live-dot {
  background: #94a3b8;
}

@keyframes live-pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.45; transform: scale(0.8); }
}
</style>

<style>
/* 暗色主题适配（与 FloorComments 一致的 data-theme 约定） */
[data-theme="dark"] .live-badge--on { color: #4ade80; background: rgba(74, 222, 128, 0.12); }
[data-theme="dark"] .live-badge--off { color: #94a3b8; background: rgba(148, 163, 184, 0.1); }
</style>
