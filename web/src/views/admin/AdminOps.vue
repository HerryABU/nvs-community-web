<template>
  <div class="page">
    <div class="head">
      <h2>运营位</h2>
      <span class="sub">自定义导航菜单 · 首页轮播 · 每日签到 · 访客设置</span>
    </div>

    <el-tabs v-model="tab">
      <!-- 导航菜单 -->
      <el-tab-pane label="导航菜单" name="menu">
        <div class="toolbar">
          <el-button type="primary" @click="editMenu(null)">新增一级菜单</el-button>
          <span class="hint">外链只允许 http(s)；站内需以 / 开头。外链默认新窗口打开。</span>
        </div>
        <el-table :data="menus" size="small" row-key="id" default-expand-all class="panel">
          <template #empty>
            <StateBlock
              state="empty"
              skeleton="table"
              :rows="4"
              compact
              title="暂无数据"
              description="换个筛选条件试试，或等有数据后再来看。"
            />
          </template>
          <el-table-column prop="title" label="标题" min-width="160" />
          <el-table-column prop="url" label="地址" min-width="220" />
          <el-table-column prop="sort_order" label="排序" width="80" />
          <el-table-column label="可见性" width="180">
            <template #empty>
              <StateBlock
                state="empty"
                skeleton="table"
                :rows="4"
                compact
                title="暂无数据"
                description="换个筛选条件试试，或等有数据后再来看。"
              />
            </template>
            <template #default="{ row }">
              <el-tag v-if="!row.visible" size="small" type="info">隐藏</el-tag>
              <el-tag v-if="row.require_login" size="small" type="warning">需登录</el-tag>
              <el-tag v-if="row.admin_only" size="small" type="danger">仅管理员</el-tag>
              <el-tag v-if="row.badge" size="small">{{ row.badge }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="200">
            <template #empty>
              <StateBlock
                state="empty"
                skeleton="table"
                :rows="4"
                compact
                title="暂无数据"
                description="换个筛选条件试试，或等有数据后再来看。"
              />
            </template>
            <template #default="{ row }">
              <el-button text size="small" @click="editMenu(row)">编辑</el-button>
              <el-button text size="small" @click="editMenu({ parent_id: row.id, visible: true, sort_order: 0 })">
                加子项
              </el-button>
              <el-button text size="small" type="danger" @click="delMenu(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <!-- 轮播 -->
      <el-tab-pane label="首页轮播" name="carousel">
        <div class="toolbar">
          <el-button type="primary" @click="editSlide(null)">新增轮播</el-button>
          <span class="hint">后端按时段过滤：过期/未开始/关闭的不会下发到前台</span>
        </div>
        <el-table :data="slides" size="small" class="panel">
          <template #empty>
            <StateBlock
              state="empty"
              skeleton="table"
              :rows="4"
              compact
              title="暂无数据"
              description="换个筛选条件试试，或等有数据后再来看。"
            />
          </template>
          <el-table-column label="图" width="110">
            <template #empty>
              <StateBlock
                state="empty"
                skeleton="table"
                :rows="4"
                compact
                title="暂无数据"
                description="换个筛选条件试试，或等有数据后再来看。"
              />
            </template>
            <template #default="{ row }">
              <img :src="row.image_url" class="thumb" />
            </template>
          </el-table-column>
          <el-table-column prop="title" label="标题" min-width="150" />
          <el-table-column prop="link_url" label="跳转" min-width="200" />
          <el-table-column prop="sort_order" label="排序" width="80" />
          <el-table-column label="状态" width="100">
            <template #empty>
              <StateBlock
                state="empty"
                skeleton="table"
                :rows="4"
                compact
                title="暂无数据"
                description="换个筛选条件试试，或等有数据后再来看。"
              />
            </template>
            <template #default="{ row }">
              <el-tag size="small" :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '关闭' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="时段" width="230">
            <template #empty>
              <StateBlock
                state="empty"
                skeleton="table"
                :rows="4"
                compact
                title="暂无数据"
                description="换个筛选条件试试，或等有数据后再来看。"
              />
            </template>
            <template #default="{ row }">
              <span class="small">{{ row.start_at ? fmt(row.start_at) : '—' }} ~ {{ row.end_at ? fmt(row.end_at) : '—' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="150">
            <template #empty>
              <StateBlock
                state="empty"
                skeleton="table"
                :rows="4"
                compact
                title="暂无数据"
                description="换个筛选条件试试，或等有数据后再来看。"
              />
            </template>
            <template #default="{ row }">
              <el-button text size="small" @click="editSlide(row)">编辑</el-button>
              <el-button text size="small" type="danger" @click="delSlide(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <!-- 签到 -->
      <el-tab-pane label="每日签到" name="checkin">
        <el-form label-width="150px" class="panel" style="max-width: 680px">
          <el-form-item label="启用签到">
            <el-switch v-model="checkin.enabled" />
            <span class="hint">默认关闭：避免升级后凭空多出代币支出</span>
          </el-form-item>
          <el-form-item label="基础奖励">
            <el-input-number v-model="checkin.base_reward" :min="0" :max="1000" />
            <span class="hint">代币/天</span>
          </el-form-item>
          <el-form-item label="连签加成">
            <el-input-number v-model="checkin.streak_bonus" :min="0" :max="100" />
            <span class="hint">每多一天加这么多，连签上限</span>
            <el-input-number v-model="checkin.max_streak" :min="1" :max="365" style="margin-left: 6px" />
            <span class="hint">天</span>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="savingCheckin" @click="saveCheckin">保存</el-button>
            <span class="hint" style="margin-left: 10px">同一天重复签到不会重复发币（唯一索引 + 先落记录后发币）</span>
          </el-form-item>
        </el-form>
        <el-table :data="checkinStats" size="small" class="panel">
          <template #empty>
            <StateBlock
              state="empty"
              skeleton="table"
              :rows="4"
              compact
              title="暂无数据"
              description="换个筛选条件试试，或等有数据后再来看。"
            />
          </template>
          <el-table-column prop="date" label="日期" width="140" />
          <el-table-column prop="users" label="签到人数" width="120" />
          <el-table-column prop="reward" label="发出代币" width="120" />
        </el-table>
      </el-tab-pane>

      <!-- 直播 -->
      <el-tab-pane label="直播" name="live">
        <el-alert type="info" :closable="false" class="mb"
          title="本站不提供推流/转码/HLS：只做入口与嵌入。外部直播间填 embed_url（必须 https），录播可填 video_url 直链。" />
        <el-form label-width="150px" class="panel" style="max-width: 760px">
          <el-form-item label="启用直播入口"><el-switch v-model="live.enabled" /></el-form-item>
          <el-form-item label="标题"><el-input v-model="live.title" /></el-form-item>
          <el-form-item label="状态">
            <el-radio-group v-model="live.status">
              <el-radio-button value="offline">未开播</el-radio-button>
              <el-radio-button value="preview">预告</el-radio-button>
              <el-radio-button value="live">直播中</el-radio-button>
            </el-radio-group>
          </el-form-item>
          <el-form-item label="嵌入地址(https)">
            <el-input v-model="live.embed_url" placeholder="https://live.example.com/room/123" />
          </el-form-item>
          <el-form-item label="视频直链(可选)">
            <el-input v-model="live.video_url" placeholder="https://cdn.example.com/live.mp4 或 /uploads/..." />
          </el-form-item>
          <el-form-item label="海报(可选)">
            <el-input v-model="live.poster_url" placeholder="/uploads/... 或 https://" />
          </el-form-item>
          <el-form-item label="开播时间">
            <el-date-picker v-model="live.starts_at" type="datetime" placeholder="可选，用于倒计时" />
          </el-form-item>
          <el-form-item label="公告">
            <el-input v-model="live.notice" type="textarea" :rows="2" placeholder="如：今晚 8 点连麦答疑" />
          </el-form-item>
          <el-form-item label="聊天群组 ID">
            <el-input-number v-model="live.chat_group_id" :min="0" />
            <span class="hint">0 = 不关联；关联后直播页给「进入直播聊天群」按钮（复用群组，不新造聊天）</span>
          </el-form-item>
          <el-form-item label="需要登录">
            <el-switch v-model="live.require_login" />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="savingLive" @click="saveLive">保存</el-button>
            <el-button @click="$router.push('/live')">预览直播页</el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <!-- 访客 -->
      <el-tab-pane label="访客设置" name="visit">
        <el-form label-width="170px" class="panel" style="max-width: 700px">
          <el-form-item label="允许未登录浏览">
            <el-switch v-model="visit.allow_guest_browse" />
          </el-form-item>
          <el-form-item label="允许未登录搜索">
            <el-switch v-model="visit.allow_guest_search" />
          </el-form-item>
          <el-form-item label="未登录最多看几章">
            <el-input-number v-model="visit.guest_max_chapters" :min="0" :max="500" />
            <span class="hint">0 = 不限</span>
          </el-form-item>
          <el-form-item label="展示访问量">
            <el-switch v-model="visit.show_visit_count" />
          </el-form-item>
          <el-form-item label="访客提示文案">
            <el-input v-model="visit.notice_text" />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="savingVisit" @click="saveVisit">保存</el-button>
          </el-form-item>
        </el-form>
        <el-table :data="visitStats" size="small" class="panel">
          <template #empty>
            <StateBlock
              state="empty"
              skeleton="table"
              :rows="4"
              compact
              title="暂无数据"
              description="换个筛选条件试试，或等有数据后再来看。"
            />
          </template>
          <el-table-column prop="date" label="日期" width="140" />
          <el-table-column prop="pv" label="PV" width="120" />
          <el-table-column prop="uv" label="UV（按 IP）" width="140" />
        </el-table>
      </el-tab-pane>
    </el-tabs>

    <!-- 菜单弹窗 -->
    <el-dialog v-model="menuVisible" :title="menuForm.id ? '编辑菜单' : '新增菜单'" width="520px">
      <el-form label-width="90px">
        <el-form-item label="标题"><el-input v-model="menuForm.title" /></el-form-item>
        <el-form-item label="地址">
          <el-input v-model="menuForm.url" placeholder="/novels 或 https://example.com" />
        </el-form-item>
        <el-form-item label="图标"><el-input v-model="menuForm.icon" placeholder="emoji 或图标名" /></el-form-item>
        <el-form-item label="排序"><el-input-number v-model="menuForm.sort_order" :min="0" :max="999" /></el-form-item>
        <el-form-item label="小标"><el-input v-model="menuForm.badge" placeholder="NEW / HOT" /></el-form-item>
        <el-form-item label="可见"><el-switch v-model="menuForm.visible" /></el-form-item>
        <el-form-item label="需登录"><el-switch v-model="menuForm.require_login" /></el-form-item>
        <el-form-item label="仅管理员"><el-switch v-model="menuForm.admin_only" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="menuVisible = false">取消</el-button>
        <el-button type="primary" @click="saveMenu">保存</el-button>
      </template>
    </el-dialog>

    <!-- 轮播弹窗 -->
    <el-dialog v-model="slideVisible" :title="slideForm.id ? '编辑轮播' : '新增轮播'" width="520px">
      <el-form label-width="90px">
        <el-form-item label="标题"><el-input v-model="slideForm.title" /></el-form-item>
        <el-form-item label="图片地址">
          <el-input v-model="slideForm.image_url" placeholder="/uploads/... 或 https://" />
        </el-form-item>
        <el-form-item label="跳转地址"><el-input v-model="slideForm.link_url" /></el-form-item>
        <el-form-item label="排序"><el-input-number v-model="slideForm.sort_order" :min="0" :max="999" /></el-form-item>
        <el-form-item label="启用"><el-switch v-model="slideForm.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="slideVisible = false">取消</el-button>
        <el-button type="primary" @click="saveSlide">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import api from '@/api/index';
import { opsApi, type NavMenuItem, type Carousel, type CheckInConfig, type VisitSetting } from '@/api/ops';
import StateBlock from '@/components/common/StateBlock.vue';

const tab = ref('menu');

const menus = ref<NavMenuItem[]>([]);
const menuVisible = ref(false);
const menuForm = reactive<Partial<NavMenuItem>>({});

const slides = ref<Carousel[]>([]);
const slideVisible = ref(false);
const slideForm = reactive<Partial<Carousel>>({});

const checkin = reactive<CheckInConfig>({ enabled: false, base_reward: 1, streak_bonus: 1, max_streak: 7, require_post: false });
const checkinStats = ref<any[]>([]);
const savingCheckin = ref(false);

const visit = reactive<VisitSetting>({
  allow_guest_browse: true, allow_guest_search: true, allow_guest_read_all: false,
  guest_max_chapters: 10, show_visit_count: true, block_guest_post: true, notice_text: '',
});
const visitStats = ref<any[]>([]);
const savingVisit = ref(false);

// 直播（gap-audit #23）
const live = reactive<any>({
  enabled: false, title: '直播间', status: 'offline', embed_url: '', video_url: '',
  poster_url: '', starts_at: null, notice: '', chat_group_id: 0, require_login: false,
});
const savingLive = ref(false);

function fmt(s?: string | null) {
  return s ? new Date(s).toLocaleString('zh-CN') : '';
}

function editMenu(row: Partial<NavMenuItem> | null) {
  Object.keys(menuForm).forEach((k) => delete (menuForm as any)[k]);
  Object.assign(menuForm, row || { visible: true, sort_order: 0, target: 'self', parent_id: 0 });
  menuVisible.value = true;
}
async function saveMenu() {
  try {
    await opsApi.saveNavMenu({ ...menuForm });
    ElMessage.success('已保存');
    menuVisible.value = false;
    await loadMenus();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
}
async function delMenu(row: NavMenuItem) {
  await ElMessageBox.confirm(`删除「${row.title}」？子项会一起删除。`, '确认', { type: 'warning' });
  await opsApi.deleteNavMenu(row.id);
  ElMessage.success('已删除');
  await loadMenus();
}

function editSlide(row: Partial<Carousel> | null) {
  Object.keys(slideForm).forEach((k) => delete (slideForm as any)[k]);
  Object.assign(slideForm, row || { enabled: true, sort_order: 0, target: 'blank' });
  slideVisible.value = true;
}
async function saveSlide() {
  try {
    await opsApi.saveCarousel({ ...slideForm });
    ElMessage.success('已保存');
    slideVisible.value = false;
    await loadSlides();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
}
async function delSlide(row: Carousel) {
  await ElMessageBox.confirm(`删除轮播「${row.title}」？`, '确认', { type: 'warning' });
  await opsApi.deleteCarousel(row.id);
  ElMessage.success('已删除');
  await loadSlides();
}

async function loadMenus() {
  const res = await opsApi.adminNavMenus();
  menus.value = res.data.data.items || [];
}
async function loadSlides() {
  const res = await opsApi.adminCarousels();
  slides.value = res.data.data.items || [];
}
async function loadCheckin() {
  const res = await opsApi.adminCheckIn();
  Object.assign(checkin, res.data.data.config);
  checkinStats.value = res.data.data.stats || [];
}
async function saveCheckin() {
  savingCheckin.value = true;
  try {
    await opsApi.saveCheckIn({ ...checkin });
    ElMessage.success('已保存');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    savingCheckin.value = false;
  }
}
async function loadLive() {
  const res = await api.get<{ code: number; data: { config: any } }>('/admin/live');
  Object.assign(live, res.data.data.config);
}
async function saveLive() {
  savingLive.value = true;
  try {
    await api.put('/admin/live', { ...live });
    ElMessage.success('已保存');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    savingLive.value = false;
  }
}

async function loadVisit() {
  const res = await opsApi.adminVisit();
  Object.assign(visit, res.data.data.config);
  visitStats.value = res.data.data.stats || [];
}
async function saveVisit() {
  savingVisit.value = true;
  try {
    await opsApi.saveVisit({ ...visit });
    ElMessage.success('已保存');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    savingVisit.value = false;
  }
}

onMounted(async () => {
  await loadMenus();
  await loadSlides();
  await loadCheckin();
  await loadVisit();
  await loadLive();
});
</script>

<style scoped>
.page { padding: 16px; }
.head { display: flex; align-items: baseline; gap: 12px; margin-bottom: 10px; }
.head h2 { margin: 0; }
.sub, .hint, .small { color: var(--text-muted); font-size: 0.78rem; }
.panel { margin-top: 12px; max-width: 1000px; }
.toolbar { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.thumb { width: 88px; height: 48px; object-fit: cover; border-radius: 6px; }
</style>
