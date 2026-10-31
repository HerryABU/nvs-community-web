<template>
  <div class="page">
    <!-- 接口异常常驻提示：本页没有 AdminHeader，靠这一行拿到全局错误反馈 -->
    <AdminApiAlert />
    <div class="head">
      <h2>运营位</h2>
      <span class="sub">自定义导航菜单 · 首页轮播 · 每日签到 · 访客设置 · 地图服务</span>
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
            <template #default="{ row }">
              <el-tag v-if="!row.visible" size="small" type="info">隐藏</el-tag>
              <el-tag v-if="row.require_login" size="small" type="warning">需登录</el-tag>
              <el-tag v-if="row.admin_only" size="small" type="danger">仅管理员</el-tag>
              <el-tag v-if="row.badge" size="small">{{ row.badge }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="200">
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
            <template #default="{ row }">
              <img :src="row.image_url" class="thumb" />
            </template>
          </el-table-column>
          <el-table-column prop="title" label="标题" min-width="150" />
          <el-table-column prop="link_url" label="跳转" min-width="200" />
          <el-table-column prop="sort_order" label="排序" width="80" />
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag size="small" :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '关闭' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="时段" width="230">
            <template #default="{ row }">
              <span class="small">{{ row.start_at ? fmt(row.start_at) : '—' }} ~ {{ row.end_at ? fmt(row.end_at) : '—' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="150">
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
            <span class="hint">关闭后作品列表、详情、正文一律要求登录（该开关优先于「站点设置」里的作品访问策略）</span>
          </el-form-item>
          <el-form-item label="允许未登录搜索">
            <el-switch v-model="visit.allow_guest_search" />
            <span class="hint">关闭后统一搜索与作者搜索对匿名返回 401</span>
          </el-form-item>
          <el-form-item label="未登录最多看几章">
            <el-input-number v-model="visit.guest_max_chapters" :min="0" :max="500" />
            <span class="hint">0 = 不限；仅当「站点设置 → 访客查看小说」为「试读」时生效</span>
          </el-form-item>
          <el-form-item label="展示访问量">
            <el-switch v-model="visit.show_visit_count" />
          </el-form-item>
          <el-form-item label="访客提示文案">
            <el-input v-model="visit.notice_text" />
          </el-form-item>
          <el-form-item label="首页内容">
            <el-radio-group v-model="homeView">
              <el-radio-button value="novels">作品</el-radio-button>
              <el-radio-button value="blogs">博客</el-radio-button>
              <el-radio-button value="forum">论坛</el-radio-button>
              <el-radio-button value="feed">混合流</el-radio-button>
            </el-radio-group>
            <span class="hint">决定访问站点根路径时展示哪套内容（选择后立即保存）</span>
          </el-form-item>
          <el-form-item label="地图底图">
            <el-button size="small" @click="tab = 'map'">去「地图服务」配置</el-button>
            <span class="hint">标准地图（天地图密钥 · 审图号 · 联通检测）在独立标签页配置</span>
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

      <!-- ===================== 地图服务（标准地图合规） ===================== -->
      <el-tab-pane label="地图服务" name="map">
        <div class="map-guide panel">
          <div class="mg-title">
            <el-icon><Location /></el-icon>
            <span>「访问来源地域」要用合规底图，密钥由站长自己填</span>
            <el-tag v-if="mapSetting.provider === 'tianditu' && mapSetting.tianditu_key" type="success" size="small" effect="plain">
              已启用天地图
            </el-tag>
            <el-tag v-else type="info" size="small" effect="plain">当前：本地离线示意</el-tag>
          </div>
          <p class="mg-lead">
            平台<b>不预置、不代填</b>任何地图密钥。国内唯一可对外展示的标准地图来源是
            <a href="https://www.tianditu.gov.cn/" target="_blank" rel="noopener">天地图</a>
            （国家地理信息公共服务平台 · 自然资源部），中国和全球视野都支持。
          </p>
          <ol class="mg-steps">
            <li><b>注册</b>：打开 tianditu.gov.cn → 注册账号并完成实名认证（个人/单位均可）。</li>
            <li>
              <b>建应用</b>：控制台 →「应用管理」→ 创建新应用，类型选
              <code>浏览器端</code>，把本站域名填进 <code>Referer 白名单</code>。
              <span class="mg-tip">填本站当前访问地址即可：{{ siteOrigin }}</span>
            </li>
            <li><b>取 tk</b>：应用详情里复制 tk，粘到下面，点「检测密钥」确认能取到瓦片，再保存。</li>
            <li><b>标来源</b>：按天地图要求填写审图号，前台地图下方会自动标注来源。</li>
          </ol>
          <p class="mg-note">
            不填也能用：系统会用内置 ip2region 数据按省份画气泡（本地示意边界，<b>非审图件</b>，
            仅供站内参考，只覆盖国内）。但只要有对外展示需求，就应该配置标准地图。
          </p>
        </div>

        <el-form label-width="150px" class="panel" style="max-width: 780px">
          <el-form-item label="底图来源">
            <el-radio-group v-model="mapSetting.provider">
              <el-radio-button value="offline">本地离线示意</el-radio-button>
              <el-radio-button value="tianditu">天地图（标准地图）</el-radio-button>
            </el-radio-group>
            <span class="hint">对外展示请选天地图</span>
          </el-form-item>
          <el-form-item label="默认视野">
            <el-radio-group v-model="mapSetting.scope">
              <el-radio-button value="china">中国</el-radio-button>
              <el-radio-button value="world">全球</el-radio-button>
            </el-radio-group>
            <span class="hint">前台仍可临时切换，不影响这里</span>
          </el-form-item>
          <el-form-item label="天地图 tk">
            <div class="tk-row">
              <el-input
                v-model="mapSetting.tianditu_key"
                placeholder="粘贴天地图应用里的 tk（浏览器端）"
                show-password
                clearable
              />
              <el-button :loading="mapTest.loading" @click="testMapKey">检测密钥</el-button>
            </div>
            <span class="hint">tk 需要在前端加载底图，属于该服务的固有形态；用 Referer 白名单防滥用</span>
          </el-form-item>

          <!-- 检测结果：正常/失败都要说清下一步做什么 -->
          <el-form-item v-if="mapTest.done" label="检测结果">
            <div class="mt-box" :class="mapTest.ok ? 'ok' : 'bad'">
              <div class="mt-head">
                <el-icon><component :is="mapTest.ok ? CircleCheck : CircleClose" /></el-icon>
                <b>{{ mapTest.ok ? '密钥可用' : '密钥不可用' }}</b>
                <span v-if="mapTest.status" class="mt-meta">HTTP {{ mapTest.status }} · {{ mapTest.ms }}ms</span>
              </div>
              <p class="mt-reason">{{ mapTest.reason }}</p>
              <p v-if="mapTest.browser" class="mt-browser">{{ mapTest.browser }}</p>
              <p v-if="mapTest.hint" class="mt-hint">{{ mapTest.hint }}</p>
              <p v-if="mapTest.bodyHint" class="mt-detail">天地图原文：{{ mapTest.bodyHint }}</p>
              <p v-if="mapTest.detail" class="mt-detail">{{ mapTest.detail }}</p>
              <div v-if="mapTest.ok" class="mt-preview">
                <figure>
                  <img :src="previewTiles.china" alt="中国范围瓦片预览" @error="imgErr.china = true" @load="imgErr.china = false" />
                  <figcaption>{{ imgErr.china ? '中国范围（未取到）' : '中国范围' }}</figcaption>
                </figure>
                <figure>
                  <img :src="previewTiles.world" alt="全球范围瓦片预览" @error="imgErr.world = true" @load="imgErr.world = false" />
                  <figcaption>{{ imgErr.world ? '全球范围（未取到）' : '全球范围' }}</figcaption>
                </figure>
                <span class="mg-note" style="align-self: center">
                  这两张图由<b>你的浏览器</b>直接请求天地图，能显示就说明白名单也配好了。
                </span>
              </div>
            </div>
          </el-form-item>

          <el-form-item label="审图号">
            <el-input v-model="mapSetting.review_no" placeholder="如 GS(2024)0650号" />
            <span class="hint">对外展示标准地图必须标注；留空则前台不显示审图号，只标「底图：天地图」</span>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="savingMap" @click="saveMapSetting">保存地图配置</el-button>
            <el-button @click="mapTest.done = false">清除检测结果</el-button>
          </el-form-item>
        </el-form>
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
import { onMounted, reactive, ref, computed } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Location, CircleCheck, CircleClose } from '@element-plus/icons-vue';
import api from '@/api/index';
import { opsApi, type NavMenuItem, type Carousel, type CheckInConfig, type VisitSetting } from '@/api/ops';
import StateBlock from '@/components/common/StateBlock.vue';
import AdminApiAlert from '@/components/admin/AdminApiAlert.vue';

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
    // 首页形态是独立配置（不属于 VisitSetting），单独保存
    await api.put('/admin/home-view', { view: homeView.value });
    ElMessage.success('已保存');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    savingVisit.value = false;
  }
}

// ---- 首页形态（作品 / 博客 / 论坛 / 混合流）----
const homeView = ref<'novels' | 'blogs' | 'forum' | 'feed'>('novels');

// ---- 地图服务（标准地图合规：离线示意 / 天地图，国内+全球）----
const mapSetting = reactive({
  provider: 'offline' as 'offline' | 'tianditu',
  scope: 'china' as 'china' | 'world',
  tianditu_key: '',
  review_no: '',
});
const savingMap = ref(false);

// 检测结果（后端带 Referer 拉一张真实瓦片，把 HTTP 结果翻成人话）
const mapTest = reactive({
  loading: false,
  done: false,
  ok: false,
  reason: '',
  hint: '',
  browser: '',
  bodyHint: '',
  detail: '',
  status: 0,
  ms: 0,
});
const imgErr = reactive({ china: false, world: false });

// 本站当前访问地址（cpolar / 反代域名也据此判断），用于提示 Referer 白名单该填什么
const siteOrigin = computed(() => (typeof window === 'undefined' ? '' : window.location.origin));

/**
 * 预览瓦片：直接由浏览器请求，im 能显示 = 密钥 + Referer 白名单都通。
 * z=4 中国中东部；z=2 覆盖东亚+西太平洋，用来验证「全球视野」同样可用。
 */
function tileUrl(tk: string, z: number, row: number, col: number) {
  const q = [
    'SERVICE=WMTS',
    'REQUEST=GetTile',
    'VERSION=1.0.0',
    'LAYER=vec',
    'STYLE=default',
    'TILEMATRIXSET=w',
    'FORMAT=tiles',
    `TILEMATRIX=${z}`,
    `TILEROW=${row}`,
    `TILECOL=${col}`,
    `tk=${encodeURIComponent(tk)}`,
  ].join('&');
  return `https://t0.tianditu.gov.cn/vec_w/wmts?${q}`;
}
const previewTiles = computed(() => ({
  china: tileUrl(mapSetting.tianditu_key.trim(), 4, 6, 12),
  world: tileUrl(mapSetting.tianditu_key.trim(), 2, 1, 2),
}));

/** 用 <img> 实测一张瓦片：只有浏览器自己请求，才是站长真正会看到的体验 */
function probeTile(url: string, timeout = 8000): Promise<boolean> {
  return new Promise((resolve) => {
    const img = new Image();
    let settled = false;
    const finish = (ok: boolean) => {
      if (settled) return;
      settled = true;
      resolve(ok);
    };
    const timer = setTimeout(() => finish(false), timeout);
    img.onload = () => {
      clearTimeout(timer);
      finish(img.naturalWidth > 0 || img.width > 0);
    };
    img.onerror = () => {
      clearTimeout(timer);
      finish(false);
    };
    img.src = url;
  });
}

async function testMapKey() {
  const tk = mapSetting.tianditu_key.trim();
  if (!tk) {
    ElMessage.warning('先填天地图 tk，再点检测');
    return;
  }
  mapTest.loading = true;
  Object.assign(mapTest, { done: false, ok: false, reason: '', hint: '', browser: '', bodyHint: '', detail: '', status: 0, ms: 0 });
  imgErr.china = false;
  imgErr.world = false;

  const tiles = previewTiles.value;
  const serverReq = api
    .post<{ code: number; data: any }>('/admin/map-setting/test', { tk })
    .then((r) => r.data?.data || ({} as any))
    .catch((e: any) => ({ __error: e?.response?.data?.message || '检测请求失败' }));

  // 浏览器侧（判据主来源）+ 服务端侧（给出 HTTP 诊断）一起跑，谁先有结论都不浪费时间
  const [cnOk, worldOk, srv] = await Promise.all([probeTile(tiles.china), probeTile(tiles.world), serverReq]);

  imgErr.china = !cnOk;
  imgErr.world = !worldOk;
  const browserOk = cnOk || worldOk;
  // 服务端被 WAF 拦或出网受限时，浏览器结果就是唯一可信判据
  const serverBlocked =
    !!srv.__error ||
    srv.stage === 'network' ||
    (typeof srv.reason === 'string' && srv.reason.includes('WAF'));

  mapTest.done = true;
  mapTest.ok = browserOk || (!!srv.ok && !serverBlocked);
  mapTest.status = srv.status || 0;
  mapTest.ms = srv.ms || 0;
  mapTest.bodyHint = srv.body_hint || '';
  mapTest.detail = srv.content_type ? `${srv.content_type} · ${srv.bytes ?? 0} bytes` : '';
  mapTest.browser = `浏览器实测：中国范围${cnOk ? '已取到' : '未取到'} · 全球范围${worldOk ? '已取到' : '未取到'}`;

  if (browserOk) {
    mapTest.reason = '密钥可用：浏览器已成功取到天地图瓦片';
    mapTest.hint = srv.ok
      ? '保存后立即生效，前台地图会换成天地图底图'
      : '以浏览器结果为准（服务端探测被天地图 WAF 拦下，属正常现象，不影响前台）';
  } else if (serverBlocked) {
    // 服务端被拦、浏览器也没取到：只能确认「不可用」，但说不出更细的原因
    mapTest.reason = '浏览器没能取到天地图瓦片，暂不可用';
    mapTest.hint =
      '依次确认：① tk 来自「浏览器端」应用；② Referer 白名单含 ' +
      siteOrigin.value +
      '；③ 浏览器能访问 t0.tianditu.gov.cn（公司网络/插件可能拦截）';
  } else {
    mapTest.reason = srv.reason || '浏览器未能取到瓦片';
    mapTest.hint = srv.hint || '核对 tk 是否「浏览器端」、Referer 白名单是否含本站域名';
  }
  mapTest.loading = false;
}

async function saveMapSetting() {
  savingMap.value = true;
  try {
    await api.put('/admin/map-setting', { ...mapSetting });
    ElMessage.success(mapSetting.provider === 'tianditu' && !mapSetting.tianditu_key.trim()
      ? '已保存；但 tk 为空，前台仍会用本地离线示意'
      : '已保存');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    savingMap.value = false;
  }
}

async function loadMapSetting() {
  try {
    const res = await api.get<{ code: number; data: any }>('/admin/map-setting');
    const d = res.data?.data;
    if (d) {
      mapSetting.provider = d.provider || 'offline';
      mapSetting.scope = d.scope || 'china';
      mapSetting.tianditu_key = d.tianditu_key || '';
      mapSetting.review_no = d.review_no || '';
    }
  } catch {
    /* 读不到保持默认 */
  }
}

async function loadHomeView() {
  try {
    const res = await api.get<{ code: number; data: { view: string } }>('/admin/home-view');
    const v = res.data?.data?.view;
    if (v) homeView.value = v as any;
  } catch {
    /* 读不到就保持默认，不弹错打断运营操作 */
  }
}

onMounted(async () => {
  await loadMenus();
  await loadSlides();
  await loadCheckin();
  await loadVisit();
  await loadHomeView();
  await loadMapSetting();
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

/* ===== 地图服务：说明卡 + 检测结果 ===== */
.map-guide {
  max-width: 900px;
  padding: 14px 16px;
  border: 1px solid var(--border-color, #e5e7eb);
  border-left: 3px solid var(--el-color-primary, #409eff);
  border-radius: 8px;
  background: var(--bg-soft, #fafbfc);
}
.mg-title {
  display: flex; align-items: center; gap: 8px; flex-wrap: wrap;
  font-size: 0.92rem; font-weight: 600; color: var(--text-main, #1f2937);
}
.mg-lead { margin: 8px 0 0; font-size: 0.8rem; line-height: 1.75; color: var(--text-main, #374151); }
.mg-lead a { color: var(--el-color-primary, #409eff); text-decoration: none; }
.mg-lead a:hover { text-decoration: underline; }
.mg-steps { margin: 8px 0 0; padding-left: 20px; font-size: 0.8rem; line-height: 1.85; color: var(--text-main, #374151); }
.mg-steps code {
  padding: 0 5px; border-radius: 4px; font-size: 0.75rem;
  background: var(--bg-page, #eef2f7); color: var(--text-main, #1f2937);
}
.mg-tip { display: block; font-size: 0.74rem; color: var(--text-muted, #9ca3af); word-break: break-all; }
.mg-note { margin: 8px 0 0; font-size: 0.75rem; line-height: 1.7; color: var(--text-muted, #6b7280); }

.tk-row { display: flex; gap: 8px; width: 100%; }
.mt-box {
  width: 100%; padding: 10px 12px; border-radius: 8px;
  border: 1px solid var(--border-color, #e5e7eb);
  background: var(--bg-card, #fff);
}
.mt-box.ok { border-color: var(--el-color-success-light-5, #b3e19d); background: var(--el-color-success-light-9, #f0f9eb); }
.mt-box.bad { border-color: var(--el-color-danger-light-5, #fab6b6); background: var(--el-color-danger-light-9, #fef0f0); }
.mt-head { display: flex; align-items: center; gap: 6px; font-size: 0.82rem; }
.mt-box.ok .mt-head { color: var(--el-color-success, #67c23a); }
.mt-box.bad .mt-head { color: var(--el-color-danger, #f56c6c); }
.mt-meta { margin-left: auto; font-size: 0.72rem; color: var(--text-muted, #9ca3af); font-variant-numeric: tabular-nums; }
.mt-reason { margin: 6px 0 0; font-size: 0.8rem; color: var(--text-main, #374151); }
.mt-browser { margin: 6px 0 0; font-size: 0.76rem; color: var(--text-main, #374151); }
.mt-hint { margin: 4px 0 0; font-size: 0.75rem; line-height: 1.7; color: var(--text-muted, #6b7280); word-break: break-all; }
.mt-detail { margin: 4px 0 0; font-size: 0.72rem; color: var(--text-muted, #9ca3af); font-variant-numeric: tabular-nums; }
.mt-preview { display: flex; gap: 10px; flex-wrap: wrap; margin-top: 10px; }
.mt-preview figure { margin: 0; }
.mt-preview img {
  width: 120px; height: 90px; object-fit: cover; border-radius: 6px;
  border: 1px solid var(--border-color, #e5e7eb); background: var(--bg-page, #eef2f7);
}
.mt-preview figcaption { font-size: 0.7rem; text-align: center; color: var(--text-muted, #9ca3af); margin-top: 3px; }
</style>
