<template>
  <div class="wd page-container">
    <!-- 密码公开的世界：先要访问码 -->
    <el-card v-if="locked" class="wd-lock" shadow="never">
      <template #header><b>「{{ project?.title || '这个世界' }}」需要访问密码</b></template>
      <div class="wd-lock-row">
        <el-input v-model="pwd" type="password" show-password placeholder="访问密码" @keyup.enter="submitPwd" />
        <el-button type="primary" :loading="pwdLoading" @click="submitPwd">进入</el-button>
      </div>
      <el-alert type="info" :closable="false" title="站长或负责人可提供访问密码；密码只用于本世界，7 天内免重复输入。" />
    </el-card>

    <template v-else>
      <!-- 面包屑 -->
      <div class="wd-crumb">
        <router-link to="/subculture">亚文化创作区</router-link>
        <span class="wd-sep">/</span>
        <span class="wd-crumb-cur">{{ project?.title || '世界' }}</span>
      </div>

      <!-- 世界头 -->
      <header v-loading="loading" class="wd-hero">
        <div class="wd-cover">
          <img v-if="project?.cover_url" :src="mediaUrl(project.cover_url)" :alt="project.title" />
          <span v-else class="wd-cover-ph">🗺️</span>
        </div>
        <div class="wd-hero-main">
          <h1 class="wd-title">
            {{ project?.title || '加载中…' }}
            <el-tag v-if="project" size="small" :type="visType">{{ visLabel }}</el-tag>
            <el-tag v-if="project?.category" size="small" type="info">{{ project.category }}</el-tag>
          </h1>
          <p class="wd-summary">{{ project?.summary || '还没有简介，点右侧「编辑世界」写下世界观梗概。' }}</p>
          <div class="wd-stats">
            <span><b>{{ chars.length }}</b> 角色</span>
            <span><b>{{ worlds.length }}</b> 世界书</span>
            <span><b>{{ arts.length }}</b> 形象图</span>
            <span><b>{{ members.length }}</b> 成员</span>
            <span v-if="project?.owner_name" class="wd-owner">负责人 {{ project.owner_name }}</span>
          </div>
        </div>
        <div class="wd-hero-act">
          <el-button v-if="canEdit" size="small" @click="openSetting">编辑世界</el-button>
        </div>
      </header>

      <el-tabs v-model="tab" class="wd-tabs">
        <!-- ============ 角色 ============ -->
        <el-tab-pane label="角色" name="chars">
          <el-alert
            v-if="worlds.length === 0"
            class="wd-hint"
            type="warning"
            :closable="false"
            title="本世界还没有世界书，角色需要先归属一个世界书；正在自动创建…"
          />
          <CharacterManager
            ref="charMgr"
            :group-id="0"
            :can-bind="false"
            :project-id="projectId"
            @changed="reloadChars"
          />
        </el-tab-pane>

        <!-- ============ 形象图库 ============ -->
        <el-tab-pane label="形象图库" name="gallery">
          <div class="wd-gbar">
            <el-select v-model="galFilter" size="small" clearable placeholder="全部角色" style="width: 180px">
              <el-option v-for="c in chars" :key="c.id" :label="c.name" :value="c.id" />
            </el-select>
            <span class="wd-gtip">每张图可标注「这张图强调的外貌特征」，配合角色的样子与性格一起看。</span>
            <el-button size="small" type="primary" plain :disabled="!chars.length" @click="openArt(chars[0])">
              ＋ 添加形象图
            </el-button>
          </div>

          <el-empty v-if="!chars.length" description="先在「角色」里创建角色，再回来放立绘" :image-size="70" />

          <section v-for="c in galChars" :key="c.id" class="wd-gsec">
            <div class="wd-ghead">
              <el-avatar :size="40" :src="mediaUrl(c.avatar_url)" :alt="c.name">{{ (c.name || '?')[0] }}</el-avatar>
              <div class="wd-ginfo">
                <div class="wd-gname">
                  {{ c.name }}
                  <el-tag v-if="c.oc_kind === 'furry'" size="small" type="warning">🐾 FURRY</el-tag>
                  <el-tag v-if="c.species" size="small" type="info">{{ c.species }}</el-tag>
                </div>
                <div class="wd-gmeta">
                  <span v-if="c.species_traits">动物特征：{{ c.species_traits }}</span>
                  <span v-else>形象图 {{ artsOf(c.id).length }} 张</span>
                </div>
              </div>
              <el-button size="small" type="primary" plain @click="openArt(c)">＋ 添加</el-button>
            </div>

            <div class="wd-traits">
              <div class="wd-trait">
                <span class="wd-trait-k">样子</span>
                <span class="wd-trait-v">{{ c.appearance || '（未填外貌）' }}</span>
              </div>
              <div class="wd-trait">
                <span class="wd-trait-k">性格</span>
                <span class="wd-trait-v">{{ c.persona || '（未填人设）' }}</span>
              </div>
            </div>

            <div v-if="artsOf(c.id).length" class="wd-arts">
              <figure v-for="a in artsOf(c.id)" :key="a.id" class="wd-art">
                <el-image
                  :src="mediaUrl(a.url)"
                  :preview-src-list="artUrlsOf(c.id)"
                  fit="cover"
                  preview-teleported
                  hide-on-click-modal
                  class="wd-art-img"
                />
                <figcaption class="wd-art-cap">
                  <div class="wd-art-line">
                    <span class="wd-art-t">{{ a.title || kindLabel(a.kind) }}</span>
                    <el-tag size="small" effect="plain">{{ kindLabel(a.kind) }}</el-tag>
                  </div>
                  <div v-if="a.traits" class="wd-art-x">{{ a.traits }}</div>
                  <div v-if="a.note" class="wd-art-n">{{ a.note }}</div>
                  <div class="wd-art-act">
                    <el-button size="small" text @click="openArt(c, a)">编辑</el-button>
                    <el-popconfirm title="删除这张形象图？" @confirm="removeArt(a)">
                      <template #reference><el-button size="small" text type="danger">删除</el-button></template>
                    </el-popconfirm>
                  </div>
                </figcaption>
              </figure>
            </div>
            <div v-else class="wd-art-empty">还没有形象图。点「＋ 添加」放一张立绘，或让 AI 先出一版设定稿。</div>
          </section>
        </el-tab-pane>

        <!-- ============ 世界书 ============ -->
        <el-tab-pane label="世界书" name="books">
          <div class="wd-books">
            <aside class="wd-blist">
              <div class="wd-blist-h">世界书（{{ worlds.length }}）</div>
              <div
                v-for="b in worlds"
                :key="b.id"
                class="wd-bitem"
                :class="{ on: activeBookId === b.id }"
                @click="selectBook(b)"
              >
                <div class="wd-btitle">{{ b.title }}</div>
                <div class="wd-bsub">v{{ b.version }} · {{ b.summary || '无简介' }}</div>
              </div>
              <div class="wd-bnew">
                <el-input v-model="newBookTitle" size="small" placeholder="新世界书名称" @keyup.enter="createBook" />
                <el-button size="small" :loading="bookSaving" @click="createBook">新建</el-button>
              </div>
            </aside>

            <main class="wd-bmain">
              <template v-if="activeBook">
                <div class="wd-bmain-h">
                  <b>{{ activeBook.title }}</b>
                  <span class="wd-bsub">条目 {{ entries.length }} · 分类覆盖 世界/势力/地点/规则/事件</span>
                  <span class="wd-grow" />
                  <el-button size="small" type="primary" @click="openEntry()">＋ 新增条目</el-button>
                </div>
                <el-table :data="entries" size="small" empty-text="还没有条目，先加「世界背景」这类常驻设定">
                  <el-table-column label="分类" width="86">
                    <template #default="{ row }"><el-tag size="small" effect="plain">{{ entryKindLabel(row.kind) }}</el-tag></template>
                  </el-table-column>
                  <el-table-column label="触发词" min-width="140">
                    <template #default="{ row }">
                      <span class="wd-key">{{ row.key || '（常驻）' }}</span>
                      <span v-if="row.secondary_keys" class="wd-key2"> / {{ row.secondary_keys }}</span>
                    </template>
                  </el-table-column>
                  <el-table-column prop="content" label="内容" min-width="240" show-overflow-tooltip />
                  <el-table-column label="常驻" width="66" align="center">
                    <template #default="{ row }">{{ row.constant ? '是' : '—' }}</template>
                  </el-table-column>
                  <el-table-column label="操作" width="128" align="right">
                    <template #default="{ row }">
                      <el-button size="small" text @click="openEntry(row)">编辑</el-button>
                      <el-popconfirm title="删除该条目？" @confirm="removeEntry(row)">
                        <template #reference><el-button size="small" text type="danger">删除</el-button></template>
                      </el-popconfirm>
                    </template>
                  </el-table-column>
                </el-table>
              </template>
              <el-empty v-else description="左侧选一本世界书开始设计条目" :image-size="60" />
            </main>
          </div>
        </el-tab-pane>

        <!-- ============ 成员 / 跨世界 ============ -->
        <el-tab-pane label="成员 / 跨世界" name="community">
          <div class="wd-two">
            <section class="wd-panel">
              <div class="wd-panel-h">成员（{{ members.length }}）</div>
              <div v-for="m in members" :key="m.id" class="wd-mrow">
                <el-avatar :size="28" :src="mediaUrl(m.avatar_url)">{{ (m.nickname || m.username || '?')[0] }}</el-avatar>
                <span class="wd-mname">{{ m.nickname || m.username || ('用户' + m.user_id) }}</span>
                <el-tag size="small" :type="m.role === 'owner' ? 'warning' : 'info'">{{ roleLabel(m.role) }}</el-tag>
                <el-button v-if="isOwner && m.user_id !== myId" size="small" text type="danger" @click="removeMember(m)">
                  移除
                </el-button>
              </div>
              <div v-if="isOwner" class="wd-madd">
                <el-input v-model="memberName" size="small" placeholder="用户名或邮箱" style="width: 170px" />
                <el-select v-model="memberRole" size="small" style="width: 96px">
                  <el-option label="编辑" value="editor" />
                  <el-option label="只读" value="viewer" />
                </el-select>
                <el-button size="small" type="primary" :loading="memberSaving" @click="addMember">添加</el-button>
              </div>
            </section>

            <section class="wd-panel">
              <div class="wd-panel-h">
                跨世界链接
                <el-select v-model="linkBookId" size="small" placeholder="选择世界书" style="width: 160px" @change="loadLinks">
                  <el-option v-for="b in worlds" :key="b.id" :label="b.title" :value="b.id" />
                </el-select>
              </div>
              <template v-if="linkBookId">
                <div v-for="l in links" :key="l.id" class="wd-lrow">
                  <span class="wd-llabel">{{ l.label || '关联' }}</span>
                  <span class="wd-ltext">
                    {{ l.from_book_id === linkBookId ? l.from_title : l.to_title }} ↔
                    {{ l.from_book_id === linkBookId ? l.to_title : l.from_title }}
                  </span>
                  <el-button size="small" text type="danger" @click="removeLink(l)">删除</el-button>
                </div>
                <el-empty v-if="!links.length" description="还没有链接" :image-size="50" />
                <div class="wd-ladd">
                  <el-select v-model="linkTarget" size="small" placeholder="链接到世界" style="flex: 1">
                    <el-option v-for="b in linkCandidates" :key="b.id" :label="b.title" :value="b.id" />
                  </el-select>
                  <el-input v-model="linkLabel" size="small" placeholder="标签" style="width: 110px" />
                  <el-button size="small" type="primary" :disabled="!linkTarget" :loading="linkSaving" @click="addLink">
                    建立
                  </el-button>
                </div>
              </template>
              <div v-else class="wd-mute">先选一本世界书（没有就在「世界书」里新建）</div>
            </section>
          </div>
        </el-tab-pane>

        <!-- ============ 设置 ============ -->
        <el-tab-pane label="设置" name="settings">
          <el-form v-if="canEdit" class="wd-setform" label-width="96px">
            <el-form-item label="世界名称"><el-input v-model="setForm.title" maxlength="60" show-word-limit /></el-form-item>
            <el-form-item label="类别">
              <el-select v-model="setForm.category" allow-create filterable placeholder="选择或输入" style="width: 100%">
                <el-option v-for="c in CATEGORIES" :key="c" :label="c" :value="c" />
              </el-select>
            </el-form-item>
            <el-form-item label="简介">
              <el-input v-model="setForm.summary" type="textarea" :rows="4" maxlength="2000" show-word-limit />
            </el-form-item>
            <el-form-item label="封面 URL"><el-input v-model="setForm.cover_url" placeholder="封面图片链接" /></el-form-item>
            <el-form-item label="可见性">
              <el-radio-group v-model="setForm.visibility">
                <el-radio-button value="private">私有（仅成员）</el-radio-button>
                <el-radio-button value="public">公开</el-radio-button>
                <el-radio-button value="password">密码公开</el-radio-button>
              </el-radio-group>
            </el-form-item>
            <el-form-item v-if="setForm.visibility === 'password'" label="访问密码">
              <el-input v-model="setForm.password" type="password" show-password placeholder="至少 6 位；留空表示不修改" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="setSaving" @click="saveSetting">保存设置</el-button>
              <el-popconfirm title="确认删除这个世界？（世界书与角色会解绑保留）" @confirm="removeProject">
                <template #reference><el-button type="danger" plain>删除世界</el-button></template>
              </el-popconfirm>
            </el-form-item>
          </el-form>
          <el-empty v-else description="只有负责人/编辑可以修改世界设置" :image-size="60" />
        </el-tab-pane>
      </el-tabs>
    </template>

    <!-- 形象图新增/编辑 -->
    <el-dialog v-model="artDlg" :title="artForm.id ? '编辑形象图' : `给「${artChar?.name || ''}」添加形象图`" width="560px">
      <el-form label-width="88px">
        <el-form-item label="图片地址" required>
          <el-input v-model="artForm.url" placeholder="/uploads/… 或 http(s) 图片链接" />
        </el-form-item>
        <el-form-item label="AI 生成">
          <el-button size="small" :loading="artGen" @click="genArt">按角色的样子与性格生成一张</el-button>
          <span class="wd-mute" style="margin-left: 8px">会填进上面的地址，可再改</span>
        </el-form-item>
        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item label="标题"><el-input v-model="artForm.title" maxlength="60" placeholder="如：夏日常服" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="分类">
              <el-select v-model="artForm.kind" style="width: 100%">
                <el-option label="立绘" value="portrait" />
                <el-option label="表情 / 半身" value="emote" />
                <el-option label="设定参考（三视图/服饰）" value="ref" />
                <el-option label="场景 / 同人" value="scene" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="强调特征">
          <el-input v-model="artForm.traits" maxlength="240" placeholder="这张图重点呈现的外貌特征：银发、断角、左眼疤…" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="artForm.note" type="textarea" :rows="2" maxlength="1000" placeholder="画师 / 授权 / 使用限制" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="artDlg = false">取消</el-button>
        <el-button type="primary" :loading="artSaving" @click="saveArt">保存</el-button>
      </template>
    </el-dialog>

    <!-- 世界书条目 -->
    <el-dialog v-model="entryDlg" :title="entryForm.id ? '编辑条目' : '新增条目'" width="640px" top="7vh">
      <el-form label-width="96px">
        <el-row :gutter="12">
          <el-col :span="10">
            <el-form-item label="分类">
              <el-select v-model="entryForm.kind" style="width: 100%">
                <el-option label="世界背景" value="world" />
                <el-option label="势力" value="faction" />
                <el-option label="地点" value="place" />
                <el-option label="规则" value="rule" />
                <el-option label="事件" value="event" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="14">
            <el-form-item label="常驻注入">
              <el-switch v-model="entryForm.constant" />
              <span class="wd-mute" style="margin-left: 8px">常驻=每次都进提示词（有固定开销）</span>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="触发词" required>
          <el-input v-model="entryForm.key" placeholder="逗号分隔，如：铁堡,铁壁城" />
        </el-form-item>
        <el-form-item label="次触发词">
          <el-input v-model="entryForm.secondary_keys" placeholder="可选：需与主触发词同时命中（逗号分隔）" />
        </el-form-item>
        <el-form-item label="内容" required>
          <el-input v-model="entryForm.content" type="textarea" :rows="6" placeholder="命中触发词时才注入这段设定（省 token 的关键）" />
        </el-form-item>
        <el-row :gutter="12">
          <el-col :span="8">
            <el-form-item label="插入顺序"><el-input-number v-model="entryForm.insertion_order" :min="0" :max="9999" size="small" /></el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="优先级"><el-input-number v-model="entryForm.priority" :min="0" :max="100" size="small" /></el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="概率"><el-input-number v-model="entryForm.probability" :min="0" :max="100" size="small" /></el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="entryDlg = false">取消</el-button>
        <el-button type="primary" :loading="entrySaving" @click="saveEntry">保存</el-button>
      </template>
    </el-dialog>

    <!-- 世界设置（头部快捷入口） -->
    <el-dialog v-model="setDlg" title="编辑世界" width="560px">
      <el-form label-width="88px">
        <el-form-item label="名称"><el-input v-model="setForm.title" maxlength="60" /></el-form-item>
        <el-form-item label="类别">
          <el-select v-model="setForm.category" allow-create filterable placeholder="选择或输入" style="width: 100%">
            <el-option v-for="c in CATEGORIES" :key="c" :label="c" :value="c" />
          </el-select>
        </el-form-item>
        <el-form-item label="简介"><el-input v-model="setForm.summary" type="textarea" :rows="4" maxlength="2000" /></el-form-item>
        <el-form-item label="封面 URL"><el-input v-model="setForm.cover_url" /></el-form-item>
        <el-form-item label="可见性">
          <el-radio-group v-model="setForm.visibility">
            <el-radio-button value="private">私有</el-radio-button>
            <el-radio-button value="public">公开</el-radio-button>
            <el-radio-button value="password">密码公开</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="setForm.visibility === 'password'" label="访问密码">
          <el-input v-model="setForm.password" type="password" show-password placeholder="至少 6 位；留空不修改" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="setDlg = false">取消</el-button>
        <el-button type="primary" :loading="setSaving" @click="saveSetting">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, nextTick, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import CharacterManager from '@/components/oc/CharacterManager.vue';
import api from '@/api/index';
import { mediaUrl } from '@/utils/prefix';
import { useAuthStore } from '@/stores/auth';
import {
  ocApi,
  ocProjectApi,
  type OcProject,
  type OcProjectMember,
  type OcWorldBook,
  type OcWorldEntry,
  type OcWorldLink,
  type OcCharacter,
  type OcCharacterArt,
} from '@/api/oc';

/**
 * 世界详情（亚文化创作区）
 *
 * 设计的出发点：立项（创建世界）之后不该继续待在列表页/抽屉里，
 * 而应该进入一个"世界自己的地方"——在这个路由下写角色、放形象图、设计世界书、
 * 管成员与跨世界链接。所以这里把原先挤在 ProjectManager 抽屉里的东西全部展开成 4 个区块。
 */
const route = useRoute();
const router = useRouter();

const CATEGORIES = ['原创世界', 'furry 兽人', '现代都市', '奇幻', '科幻', '古风', '武侠', '末日', '校园', '其他'];

const auth = useAuthStore();
const projectId = computed(() => Number(route.params.id) || 0);
const project = ref<OcProject | null>(null);
const worlds = ref<OcWorldBook[]>([]);
const chars = ref<OcCharacter[]>([]);
const arts = ref<OcCharacterArt[]>([]);
const members = ref<OcProjectMember[]>([]);
const loading = ref(false);
const tab = ref('chars');
const charMgr = ref<InstanceType<typeof CharacterManager> | null>(null);

const locked = ref(false);
const pwd = ref('');
const pwdLoading = ref(false);

const myId = computed(() => auth.user?.id || 0);
/** 负责人或成员编辑权才能改世界本身（公开世界的访客只能看） */
const canEdit = computed(() => {
  if (!project.value) return false;
  if (project.value.user_id === myId.value) return true;
  return myRole.value === 'owner' || myRole.value === 'editor';
});
const isOwner = computed(() => !!project.value && project.value.user_id === myId.value);
const myRole = computed(() => {
  const m = members.value.find((x) => x.user_id === myId.value);
  return m?.role || '';
});

const visLabel = computed(() =>
  project.value?.visibility === 'public' ? '公开' : project.value?.visibility === 'password' ? '密码公开' : '私有',
);
const visType = computed<'success' | 'warning' | 'info'>(() =>
  project.value?.visibility === 'public' ? 'success' : project.value?.visibility === 'password' ? 'warning' : 'info',
);

function tokenKey(id: number) {
  return `oc_project_token_${id}`;
}
function projectToken() {
  return localStorage.getItem(tokenKey(projectId.value)) || '';
}

// ---------- 加载 ----------
async function loadAll() {
  loading.value = true;
  try {
    const [p, w, c, a, m] = await Promise.all([
      ocProjectApi.get(projectId.value, projectToken() || undefined),
      ocProjectApi.worlds(projectId.value),
      ocApi.listCharacters({ scope: 'project', project_id: projectId.value, page_size: 50 }),
      ocApi.listProjectArts(projectId.value, projectToken() || undefined),
      ocProjectApi.members(projectId.value),
    ]);
    project.value = p.data?.data || null;
    worlds.value = w.data?.data?.list || [];
    chars.value = c.data?.data?.list || [];
    arts.value = a.data?.data?.list || [];
    members.value = m.data?.data?.list || [];
    locked.value = false;
    // 有世界之后才能建角色：项目刚建、还没有世界书时自动补一本同名世界书
    await ensureDefaultBook();
    // 默认选中第一本世界书，省一次手动点击
    if (worlds.value.length && !activeBookId.value) selectBook(worlds.value[0]);
    // 子组件可能早于上面这步就加载过（拿到空的世界书列表），这里让它重取一次，
    // 否则「新建角色卡」会被误判成「还没有世界」。
    await nextTick();
    charMgr.value?.reloadBooks();
    charMgr.value?.reload();
  } catch (e: any) {
    const code = e?.response?.status;
    const msg = e?.response?.data?.message || '';
    if (code === 403 && /密码|访问码|token/i.test(msg)) {
      locked.value = true;
      // 拿不到项目也能显示标题：密码框提示用
      project.value = project.value || null;
    } else if (code === 403) {
      locked.value = true;
    } else {
      ElMessage.error(msg || '加载世界失败');
    }
  } finally {
    loading.value = false;
  }
}

async function submitPwd() {
  if (!pwd.value.trim()) return;
  pwdLoading.value = true;
  try {
    const r = await ocProjectApi.access(projectId.value, pwd.value);
    const t = r.data?.data?.token;
    if (t) localStorage.setItem(tokenKey(projectId.value), t);
    pwd.value = '';
    await loadAll();
    ElMessage.success('已进入世界');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '密码错误');
  } finally {
    pwdLoading.value = false;
  }
}

/** 角色必须归属世界书；项目刚建立时自动补一本，避免「请先创建世界」的死循环 */
async function ensureDefaultBook() {
  if (worlds.value.length > 0 || !project.value) return;
  try {
    await ocApi.createBook({
      title: project.value.title || '世界设定',
      summary: project.value.summary || '',
      visibility: 'private',
      project_id: projectId.value,
    });
    const w = await ocProjectApi.worlds(projectId.value);
    worlds.value = w.data?.data?.list || [];
  } catch {
    /* 只读成员建不了，忽略 */
  }
}

/** 角色 tab 内增删角色后同步本页的角色列表（形象图库按它分组） */
async function reloadChars() {
  try {
    const r = await ocApi.listCharacters({ scope: 'project', project_id: projectId.value, page_size: 50 });
    chars.value = r.data?.data?.list || [];
  } catch {
    /* 静默：图库沿用旧列表 */
  }
}

// ---------- 形象图库 ----------
const galFilter = ref<number | undefined>(undefined);
const galChars = computed(() => (galFilter.value ? chars.value.filter((c) => c.id === galFilter.value) : chars.value));

function artsOf(cid: number) {
  return arts.value.filter((a) => a.character_id === cid);
}
function artUrlsOf(cid: number) {
  return artsOf(cid).map((a) => mediaUrl(a.url));
}
function kindLabel(k: string) {
  return k === 'emote' ? '表情' : k === 'ref' ? '设定参考' : k === 'scene' ? '场景' : '立绘';
}

const artDlg = ref(false);
const artSaving = ref(false);
const artGen = ref(false);
const artChar = ref<OcCharacter | null>(null);
const artForm = reactive<Partial<OcCharacterArt>>({ url: '', title: '', kind: 'portrait', traits: '', note: '', sort: 0 });

function openArt(c: OcCharacter, a?: OcCharacterArt) {
  if (!c) return;
  artChar.value = c;
  Object.assign(
    artForm,
    a
      ? { ...a }
      : { id: undefined, url: '', title: '', kind: 'portrait', traits: c.species_traits || '', note: '', sort: 0, character_id: c.id },
  );
  artDlg.value = true;
}

async function genArt() {
  const c = artChar.value;
  if (!c) return;
  artGen.value = true;
  try {
    const prompt = [
      c.oc_kind === 'furry' ? `furry 兽人角色，物种 ${c.species || ''}，动物特征 ${c.species_traits || ''}` : '原创角色立绘',
      c.appearance || '',
      c.persona ? `性格气质：${c.persona}` : '',
      '全身立绘，设定稿风格，干净背景',
    ]
      .filter(Boolean)
      .join('；');
    const res = await api.post<{ code: number; data: { images?: string[] } }>('/ai/image', {
      prompt,
      ratio: '3:4',
      n: 1,
    });
    const imgs = res.data?.data?.images || [];
    if (res.data.code === 0 && imgs.length) {
      artForm.url = imgs[0];
      ElMessage.success('已生成，记得点保存');
    } else {
      ElMessage.error('生成失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '生成失败');
  } finally {
    artGen.value = false;
  }
}

async function saveArt() {
  const c = artChar.value;
  if (!c) return;
  if (!artForm.url?.trim()) {
    ElMessage.warning('请填写图片地址或先 AI 生成');
    return;
  }
  artSaving.value = true;
  try {
    const payload = {
      url: artForm.url,
      title: artForm.title || '',
      kind: artForm.kind || 'portrait',
      traits: artForm.traits || '',
      note: artForm.note || '',
      sort: artForm.sort || 0,
    };
    if (artForm.id) await ocApi.updateCharacterArt(artForm.id, payload);
    else await ocApi.createCharacterArt(c.id, payload);
    ElMessage.success('已保存');
    artDlg.value = false;
    await reloadArts();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    artSaving.value = false;
  }
}

async function removeArt(a: OcCharacterArt) {
  try {
    await ocApi.deleteCharacterArt(a.id);
    ElMessage.success('已删除');
    await reloadArts();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

async function reloadArts() {
  try {
    const r = await ocApi.listProjectArts(projectId.value, projectToken() || undefined);
    arts.value = r.data?.data?.list || [];
  } catch {
    /* 忽略 */
  }
}

// ---------- 世界书 + 条目 ----------
const activeBookId = ref(0);
const activeBook = computed(() => worlds.value.find((b) => b.id === activeBookId.value) || null);
const entries = ref<OcWorldEntry[]>([]);
const newBookTitle = ref('');
const bookSaving = ref(false);

function selectBook(b: OcWorldBook) {
  activeBookId.value = b.id;
  loadEntries();
}

async function loadEntries() {
  if (!activeBookId.value) return;
  try {
    const r = await ocApi.getBook(activeBookId.value, projectToken() || undefined);
    entries.value = r.data?.data?.entries || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载条目失败');
  }
}

async function createBook() {
  const title = newBookTitle.value.trim();
  if (!title) {
    ElMessage.warning('请填写世界书名称');
    return;
  }
  bookSaving.value = true;
  try {
    const r: any = await ocApi.createBook({ title, visibility: 'private', project_id: projectId.value });
    newBookTitle.value = '';
    const w = await ocProjectApi.worlds(projectId.value);
    worlds.value = w.data?.data?.list || [];
    const id = r?.data?.data?.id;
    if (id) selectBook({ id, title } as OcWorldBook);
    ElMessage.success('已创建世界书');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '创建失败');
  } finally {
    bookSaving.value = false;
  }
}

const entryDlg = ref(false);
const entrySaving = ref(false);
const entryForm = reactive<Partial<OcWorldEntry>>({
  kind: 'world', key: '', content: '', secondary_keys: '', constant: false,
  selective: false, probability: 100, insertion_order: 100, priority: 50, enabled: true, sort_order: 0,
});

function openEntry(e?: OcWorldEntry) {
  Object.assign(
    entryForm,
    e
      ? { ...e }
      : {
          id: undefined, kind: 'world', key: '', content: '', secondary_keys: '', constant: false,
          selective: false, probability: 100, insertion_order: 100, priority: 50, enabled: true, sort_order: 0,
        },
  );
  entryDlg.value = true;
}

async function saveEntry() {
  if (!activeBookId.value) return;
  if (!entryForm.key?.trim() || !entryForm.content?.trim()) {
    ElMessage.warning('触发词与内容不能为空');
    return;
  }
  entrySaving.value = true;
  try {
    // camelCase：后端 binding 结构体没有 json tag，snake_case 会被静默丢弃
    await ocApi.saveEntry(activeBookId.value, {
      id: entryForm.id,
      kind: entryForm.kind,
      key: entryForm.key,
      content: entryForm.content,
      secondaryKeys: entryForm.secondary_keys,
      selective: entryForm.selective,
      constant: entryForm.constant,
      enabled: entryForm.enabled,
      probability: entryForm.probability,
      insertionOrder: entryForm.insertion_order,
      priority: entryForm.priority,
      sortOrder: entryForm.sort_order,
    });
    ElMessage.success('已保存条目');
    entryDlg.value = false;
    await loadEntries();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    entrySaving.value = false;
  }
}

async function removeEntry(e: OcWorldEntry) {
  try {
    await ocApi.deleteEntry(activeBookId.value, e.id);
    ElMessage.success('已删除');
    await loadEntries();
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.message || '删除失败');
  }
}

function entryKindLabel(k: string) {
  return k === 'faction' ? '势力' : k === 'place' ? '地点' : k === 'rule' ? '规则' : k === 'event' ? '事件' : '世界';
}

// ---------- 成员 / 跨世界 ----------
const memberName = ref('');
const memberRole = ref('editor');
const memberSaving = ref(false);

async function addMember() {
  if (!memberName.value.trim()) return;
  memberSaving.value = true;
  try {
    await ocProjectApi.addMember(projectId.value, memberName.value.trim(), memberRole.value);
    memberName.value = '';
    const m = await ocProjectApi.members(projectId.value);
    members.value = m.data?.data?.list || [];
    ElMessage.success('已添加成员');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '添加失败');
  } finally {
    memberSaving.value = false;
  }
}

async function removeMember(m: OcProjectMember) {
  try {
    await ocProjectApi.removeMember(projectId.value, m.user_id);
    const r = await ocProjectApi.members(projectId.value);
    members.value = r.data?.data?.list || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '移除失败');
  }
}

function roleLabel(r: string) {
  return r === 'owner' ? '负责人' : r === 'viewer' ? '只读' : '编辑';
}

const linkBookId = ref<number | undefined>(undefined);
const links = ref<OcWorldLink[]>([]);
const linkCandidates = ref<OcWorldBook[]>([]);
const linkTarget = ref<number | undefined>(undefined);
const linkLabel = ref('');
const linkSaving = ref(false);

async function loadLinks() {
  if (!linkBookId.value) return;
  try {
    const r = await ocProjectApi.listLinks(linkBookId.value);
    links.value = r.data?.data?.list || [];
    const mine = await ocApi.listBooks({ scope: 'usable', page_size: 50 });
    linkCandidates.value = (mine.data?.data?.list || []).filter((b: OcWorldBook) => b.id !== linkBookId.value);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载链接失败');
  }
}

async function addLink() {
  if (!linkBookId.value || !linkTarget.value) return;
  linkSaving.value = true;
  try {
    await ocProjectApi.addLink(linkBookId.value, linkTarget.value, linkLabel.value.trim());
    linkTarget.value = undefined;
    linkLabel.value = '';
    await loadLinks();
    ElMessage.success('已建立跨世界链接');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '建立链接失败');
  } finally {
    linkSaving.value = false;
  }
}

async function removeLink(l: OcWorldLink) {
  try {
    await ocProjectApi.removeLink(l.id);
    await loadLinks();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

// ---------- 世界设置 ----------
const setDlg = ref(false);
const setSaving = ref(false);
const setForm = reactive({ title: '', summary: '', cover_url: '', category: '', visibility: 'private', password: '' });

function openSetting() {
  if (!project.value) return;
  Object.assign(setForm, {
    title: project.value.title,
    summary: project.value.summary || '',
    cover_url: project.value.cover_url || '',
    category: project.value.category || '',
    visibility: project.value.visibility,
    password: '',
  });
  setDlg.value = true;
  tab.value = 'settings';
}

async function saveSetting() {
  if (!setForm.title.trim()) {
    ElMessage.warning('请填写世界名称');
    return;
  }
  setSaving.value = true;
  try {
    await ocProjectApi.update(projectId.value, {
      title: setForm.title,
      summary: setForm.summary,
      cover_url: setForm.cover_url,
      category: setForm.category,
      visibility: setForm.visibility,
      password: setForm.password || undefined,
    });
    setDlg.value = false;
    ElMessage.success('已保存');
    const p = await ocProjectApi.get(projectId.value, projectToken() || undefined);
    project.value = p.data?.data || project.value;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    setSaving.value = false;
  }
}

async function removeProject() {
  try {
    await ocProjectApi.remove(projectId.value);
    ElMessage.success('世界已删除');
    router.push('/subculture');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

// 同一组件被不同 :id 复用时不会重新 mount（Vue Router 同记录复用实例），
// 必须监听参数变化手动重载，否则从 A 世界跳 B 世界会看到 A 的数据。
watch(
  () => route.params.id,
  (v) => {
    if (!v) return;
    activeBookId.value = 0;
    entries.value = [];
    linkBookId.value = undefined;
    links.value = [];
    tab.value = 'chars';
    loadAll();
  },
);

onMounted(() => {
  loadAll();
});
</script>

<style scoped>
.wd { max-width: 1120px; margin: 0 auto; padding: 16px 16px 56px; }
.wd-lock { max-width: 460px; margin: 60px auto; }
.wd-lock-row { display: flex; gap: 8px; margin-bottom: 10px; }
.wd-crumb { font-size: 0.8rem; color: var(--text-muted); margin-bottom: 10px; display: flex; gap: 6px; align-items: center; }
.wd-crumb a { color: var(--el-color-primary); }
.wd-sep { opacity: 0.5; }
.wd-crumb-cur { color: var(--text-color, #303133); }

.wd-hero { display: flex; gap: 14px; align-items: flex-start; padding: 14px; border: 1px solid var(--border-color, #e5e7eb); border-radius: 12px; background: var(--bg-card, #fff); margin-bottom: 12px; }
.wd-cover { width: 88px; height: 88px; flex: 0 0 88px; border-radius: 10px; overflow: hidden; background: var(--fill-color, #f3f4f6); display: flex; align-items: center; justify-content: center; }
.wd-cover img { width: 100%; height: 100%; object-fit: cover; }
.wd-cover-ph { font-size: 2rem; }
.wd-hero-main { flex: 1; min-width: 0; }
.wd-title { font-size: 1.35rem; margin: 0 0 6px; display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.wd-summary { margin: 0 0 8px; font-size: 0.85rem; color: var(--text-muted); line-height: 1.6; }
.wd-stats { display: flex; gap: 14px; flex-wrap: wrap; font-size: 0.8rem; color: var(--text-muted); }
.wd-stats b { color: var(--text-color, #303133); font-size: 0.92rem; }
.wd-owner { opacity: 0.8; }
.wd-hero-act { flex: 0 0 auto; }
.wd-tabs { margin-top: 4px; }

.wd-hint { margin-bottom: 10px; }
.wd-grow { flex: 1; }
.wd-mute { font-size: 0.76rem; color: var(--text-muted); }

/* 图库 */
.wd-gbar { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; margin-bottom: 14px; }
.wd-gtip { font-size: 0.76rem; color: var(--text-muted); flex: 1; min-width: 200px; }
.wd-gsec { border: 1px solid var(--border-color, #e5e7eb); border-radius: 12px; padding: 12px; margin-bottom: 14px; background: var(--bg-card, #fff); }
.wd-ghead { display: flex; gap: 10px; align-items: center; }
.wd-ginfo { flex: 1; min-width: 0; }
.wd-gname { font-weight: 600; display: flex; gap: 6px; align-items: center; flex-wrap: wrap; }
.wd-gmeta { font-size: 0.76rem; color: var(--text-muted); margin-top: 2px; }
.wd-traits { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin: 10px 0 12px; }
.wd-trait { display: flex; gap: 8px; font-size: 0.8rem; line-height: 1.6; }
.wd-trait-k { flex: 0 0 36px; color: var(--text-muted); }
.wd-trait-v { color: var(--text-color, #303133); display: -webkit-box; -webkit-line-clamp: 3; -webkit-box-orient: vertical; overflow: hidden; }
.wd-arts { display: grid; grid-template-columns: repeat(auto-fill, minmax(160px, 1fr)); gap: 10px; }
.wd-art { margin: 0; border: 1px solid var(--border-color, #e5e7eb); border-radius: 10px; overflow: hidden; background: var(--bg-card, #fff); }
.wd-art-img { width: 100%; aspect-ratio: 3 / 4; display: block; background: var(--fill-color, #f3f4f6); }
.wd-art-cap { padding: 8px; display: flex; flex-direction: column; gap: 4px; }
.wd-art-line { display: flex; gap: 6px; align-items: center; justify-content: space-between; }
.wd-art-t { font-size: 0.82rem; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.wd-art-x { font-size: 0.74rem; color: var(--text-muted); line-height: 1.5; }
.wd-art-n { font-size: 0.72rem; color: var(--text-muted); opacity: 0.85; }
.wd-art-act { display: flex; justify-content: flex-end; gap: 2px; }
.wd-art-empty { font-size: 0.8rem; color: var(--text-muted); padding: 14px; border: 1px dashed var(--border-color, #e5e7eb); border-radius: 10px; text-align: center; }

/* 世界书 */
.wd-books { display: grid; grid-template-columns: 240px 1fr; gap: 14px; align-items: start; }
.wd-blist { border: 1px solid var(--border-color, #e5e7eb); border-radius: 10px; padding: 10px; background: var(--bg-card, #fff); }
.wd-blist-h { font-weight: 600; font-size: 0.88rem; margin-bottom: 8px; }
.wd-bitem { padding: 8px; border-radius: 8px; cursor: pointer; }
.wd-bitem:hover { background: var(--fill-color, #f3f4f6); }
.wd-bitem.on { background: var(--el-color-primary-light-9, #ecf5ff); }
.wd-btitle { font-size: 0.86rem; font-weight: 500; }
.wd-bsub { font-size: 0.72rem; color: var(--text-muted); margin-top: 2px; }
.wd-bnew { display: flex; gap: 6px; margin-top: 10px; }
.wd-bmain { border: 1px solid var(--border-color, #e5e7eb); border-radius: 10px; padding: 12px; background: var(--bg-card, #fff); min-height: 200px; }
.wd-bmain-h { display: flex; gap: 8px; align-items: center; margin-bottom: 10px; flex-wrap: wrap; }
.wd-key { font-size: 0.8rem; }
.wd-key2 { font-size: 0.74rem; color: var(--text-muted); }

/* 成员 / 跨世界 */
.wd-two { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; align-items: start; }
.wd-panel { border: 1px solid var(--border-color, #e5e7eb); border-radius: 10px; padding: 12px; background: var(--bg-card, #fff); }
.wd-panel-h { font-weight: 600; font-size: 0.88rem; margin-bottom: 10px; display: flex; gap: 8px; align-items: center; justify-content: space-between; }
.wd-mrow { display: flex; gap: 8px; align-items: center; padding: 5px 0; }
.wd-mname { flex: 1; font-size: 0.85rem; }
.wd-madd { display: flex; gap: 6px; margin-top: 10px; flex-wrap: wrap; }
.wd-lrow { display: flex; gap: 8px; align-items: center; font-size: 0.82rem; padding: 5px 0; }
.wd-llabel { color: var(--el-color-primary); }
.wd-ltext { flex: 1; color: var(--text-muted); }
.wd-ladd { display: flex; gap: 6px; margin-top: 10px; }

.wd-setform { max-width: 620px; }

@media (max-width: 860px) {
  .wd-books { grid-template-columns: 1fr; }
  .wd-two { grid-template-columns: 1fr; }
  .wd-hero { flex-wrap: wrap; }
  .wd-traits { grid-template-columns: 1fr; }
}
</style>
