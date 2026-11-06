<template>
  <div class="page-container oc-ws">
    <div class="oc-ws-head">
      <h1 class="oc-ws-title">我的 OC 工作台</h1>
      <el-tag size="small" type="info">独立于群组 · 个人 OC 资产</el-tag>
      <span class="oc-ws-tip">OC 角色 / 世界书 / 表情包归你个人所有；在任意群组的「OC 模式 → 角色卡」里可把它们加入该群 AI。</span>
    </div>

    <el-tabs v-model="tab" class="oc-ws-tabs">
      <el-tab-pane label="角色卡" name="characters">
        <CharacterManager :group-id="0" :can-bind="false" />
      </el-tab-pane>
      <el-tab-pane label="世界书" name="books">
        <WorldBookManager :group-id="0" />
      </el-tab-pane>
      <el-tab-pane label="表情包" name="emotes">
        <EmoteManager />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import CharacterManager from '@/components/oc/CharacterManager.vue';
import WorldBookManager from '@/components/oc/WorldBookManager.vue';
import EmoteManager from '@/components/oc/EmoteManager.vue';

const tab = ref('characters');
</script>

<style scoped>
.oc-ws { max-width: 1120px; margin: 0 auto; padding: 20px 16px 48px; }
.oc-ws-head { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-bottom: 16px; }
.oc-ws-title { font-size: 1.5rem; margin: 0; }
.oc-ws-tip { font-size: 0.8rem; color: var(--text-muted); flex-basis: 100%; margin-top: 4px; }
</style>
