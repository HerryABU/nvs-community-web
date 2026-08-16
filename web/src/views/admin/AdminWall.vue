<template>
  <div class="admin-page">
    <div class="admin-page-header">
      <el-button text @click="$router.push('/admin')"><el-icon><ArrowLeft /></el-icon> 返回后台</el-button>
      <h2>隔离墙配置</h2>
    </div>
    <div class="admin-page-body">
      <p class="hint-text">管理敏感分区列表及其确认流程。点击分区名称展开/收起详细配置。</p>
      <el-form label-width="100px">
        <el-form-item label="启用隔离墙">
          <el-switch v-model="wallEnabled" @change="saveWallConfig" />
        </el-form-item>
        <el-form-item label="跨域警告">
          <el-switch v-model="wallCrossDomainWarning" @change="saveWallConfig" />
        </el-form-item>
      </el-form>

      <div class="wall-zone-list">
        <div v-for="(z, i) in wallZones" :key="i" class="wall-zone-card">
          <div class="wzc-header" @click="toggleZoneDetail(i)">
            <el-icon class="wzc-arrow" :class="{ rotated: expandedZoneIdx === i }"><ArrowRight /></el-icon>
            <el-tag type="danger" size="large">{{ z }}</el-tag>
            <el-button size="small" type="danger" text @click.stop="removeWallZone(i)" style="margin-left:auto">删除</el-button>
          </div>
          <div v-if="expandedZoneIdx === i" class="wzc-body">
            <el-form label-width="110px" size="small">
              <el-form-item label="确认步数">
                <el-input-number v-model="zoneDetails[i].steps" :min="1" :max="5" />
                <span class="hint-text" style="margin-left:8px">需要用户确认几次（1~5步，默认3）</span>
              </el-form-item>
              <el-form-item label="最终确认语">
                <el-input v-model="zoneDetails[i].confirm_text" placeholder="如：我承诺承担全部阅读责任" />
                <span class="hint-text" style="margin-left:8px">最后一步要求输入的文字（留空则不要求输入）</span>
              </el-form-item>
              <el-form-item label="跨域额外步数">
                <el-input-number v-model="zoneDetails[i].cross_domain_extra" :min="0" :max="5" />
                <span class="hint-text" style="margin-left:8px">从其他敏感区跨入时的额外步数</span>
              </el-form-item>
              <el-form-item label="介绍文案">
                <el-input v-model="zoneDetails[i].intro_text" type="textarea" :rows="2" placeholder="第一步显示的介绍内容" />
              </el-form-item>
              <el-form-item label="警告列表">
                <div class="warning-list">
                  <div v-for="(w, wi) in zoneDetails[i].warnings" :key="wi" class="warning-row">
                    <el-input v-model="zoneDetails[i].warnings[wi]" placeholder="警告内容" style="flex:1" />
                    <el-button size="small" type="danger" text @click="zoneDetails[i].warnings.splice(wi,1)"><el-icon><Delete /></el-icon></el-button>
                  </div>
                </div>
                <el-button size="small" @click="zoneDetails[i].warnings.push('')" :icon="Plus" style="margin-top:4px">添加警告</el-button>
              </el-form-item>
            </el-form>
          </div>
        </div>
      </div>

      <div class="add-zone-row" style="margin-top:12px">
        <el-input v-model="newWallZone" placeholder="输入分区名称后回车添加" size="small" style="width:240px" @keyup.enter="addWallZone" />
        <el-button size="small" type="primary" @click="addWallZone" :icon="Plus" style="margin-left:8px">添加分区</el-button>
        <el-button size="small" type="success" @click="saveWallConfig" :loading="savingWall" style="margin-left:8px">保存配置</el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { ArrowLeft, ArrowRight, Delete, Plus } from '@element-plus/icons-vue';
import { adminApi } from '@/api/admin';
import { ElMessage } from 'element-plus';

const wallZones = ref<string[]>([]);
const wallEnabled = ref(false);
const wallCrossDomainWarning = ref(false);
const savingWall = ref(false);
const expandedZoneIdx = ref<number | null>(null);
const newWallZone = ref('');
const zoneDetails = ref<any[]>([]);

async function loadWallConfig() {
  try {
    const res = await adminApi.getWallConfig();
    const cfg = res.data.data || res.data || {};
    wallZones.value = cfg.zones || [];
    wallEnabled.value = cfg.enabled !== false;
    wallCrossDomainWarning.value = cfg.cross_domain_warning !== false;
    zoneDetails.value = (cfg.zone_details || wallZones.value.map((z: string) => ({ name: z, warnings: [] })));
  } catch { /* ignore */ }
}

function toggleZoneDetail(idx: number) {
  expandedZoneIdx.value = expandedZoneIdx.value === idx ? null : idx;
}

async function saveWallConfig() {
  savingWall.value = true;
  try {
    const details = wallZones.value.map((z, i) => {
      const existing = zoneDetails.value[i] || { name: z, steps: 3, confirm_text: '我承诺承担全部阅读责任', warnings: ['该分区内容属于敏感题材。'], intro_text: '', cross_domain_extra: 2 };
      return { name: z, steps: existing.steps || 3, confirm_text: existing.confirm_text || '', warnings: existing.warnings?.filter((w: string) => w.trim()) || [], intro_text: existing.intro_text || '', cross_domain_extra: existing.cross_domain_extra ?? 2 };
    });
    zoneDetails.value = details;
    await adminApi.updateWallConfig({
      zones: wallZones.value,
      zone_details: details,
      enabled: wallEnabled.value,
      cross_domain_warning: wallCrossDomainWarning.value,
    });
    ElMessage.success('隔离墙配置已保存');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  savingWall.value = false;
}

function addWallZone() {
  const name = newWallZone.value.trim();
  if (!name) return;
  if (wallZones.value.includes(name)) { ElMessage.warning('该分区已存在'); return; }
  wallZones.value.push(name);
  zoneDetails.value.push({ name, steps: 3, confirm_text: '我承诺承担全部阅读责任', warnings: ['该分区内容属于敏感题材。'], intro_text: '', cross_domain_extra: 2 });
  newWallZone.value = '';
  saveWallConfig();
}

function removeWallZone(idx: number) {
  wallZones.value.splice(idx, 1);
  zoneDetails.value.splice(idx, 1);
  if (expandedZoneIdx.value === idx) expandedZoneIdx.value = null;
  else if (expandedZoneIdx.value !== null && expandedZoneIdx.value > idx) expandedZoneIdx.value--;
  saveWallConfig();
}

onMounted(loadWallConfig);
</script>

<style scoped>
.admin-page { min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-header { display: flex; align-items: center; gap: 16px; margin-bottom: 20px; }
.admin-page-header h2 { margin: 0; font-size: 1.3rem; color: var(--text-color, #333); }
.admin-page-body { max-width: 900px; }
.hint-text { color: var(--text-light, #666); font-size: 0.85rem; }
.wall-zone-list { display: flex; flex-direction: column; gap: 10px; }
.wall-zone-card { border: 1px solid rgba(220,38,38,0.25); border-radius: 10px; padding: 12px; background: #fff; }
.wzc-header { display: flex; align-items: center; gap: 10px; cursor: pointer; }
.wzc-arrow { transition: transform 0.2s; }
.wzc-arrow.rotated { transform: rotate(90deg); }
.wzc-body { margin-top: 12px; padding-top: 12px; border-top: 1px dashed #eee; }
.warning-list { display: flex; flex-direction: column; gap: 6px; width: 100%; }
.warning-row { display: flex; gap: 6px; align-items: center; }
[data-theme="dark"] .admin-page { background: #0f172a; }
[data-theme="dark"] .admin-page-header h2 { color: #e2e8f0; }
[data-theme="dark"] .wall-zone-card { background: #1e293b; }
[data-theme="dark"] .wzc-body { border-top-color: rgba(255,255,255,0.08); }
</style>
