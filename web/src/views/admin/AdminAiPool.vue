<template>
  <div class="admin-page">
    <AdminHeader title="AI 模型池" subtitle="OpenAI 兼容上游配置：密钥加密存储、按量计费，编辑区 AI 助手的数据来源" :icon="MagicStick" />

    <!-- 两级导航：一级「模型配置 / 模型统计 / 路由与定价」，二级再按渠道·模型·厂商分视图。
         本页信息量太大（原先 5 块全摊一屏），一级切主题、二级切视角，
         当前两级都写进 URL hash（#config/channel），刷新停在原处、可直接分享。 -->
    <div class="pool-nav">
      <div class="pool-tabs pool-tabs-l1" role="tablist">
        <button
          v-for="t in level1Tabs"
          :key="t.key"
          type="button"
          role="tab"
          class="pool-tab"
          :class="{ active: level1 === t.key }"
          :aria-selected="level1 === t.key"
          @click="switchLevel1(t.key)"
        >
          <span class="pool-tab-ico">{{ t.icon }}</span>
          <span class="pool-tab-label">{{ t.label }}</span>
        </button>
      </div>
      <div class="pool-tabs pool-tabs-l2" role="tablist">
        <button
          v-for="t in level2Tabs"
          :key="t.key"
          type="button"
          role="tab"
          class="pool-tab pool-tab-sm"
          :class="{ active: level2 === t.key }"
          :aria-selected="level2 === t.key"
          @click="switchLevel2(t.key)"
        >
          <span class="pool-tab-label">{{ t.label }}</span>
          <span v-if="t.count !== null" class="pool-tab-count">{{ t.count }}</span>
        </button>
      </div>
    </div>

    <div class="admin-page-body">
      <p class="hint-text">
        AI 为<b>默认关闭</b>的增值能力：开启后用户可在 AI 工作台使用对话 / 绘画 / 视频，作者还能在章节编辑器调用「续写 / 润色 / 扩写 / 摘要」。
        <b>对话</b>按 tokens 计费（每 1K 代币价，0 = 免费），<b>绘画 / 视频</b>按次计费（每次调用代币价）；真实用量以上游返回为准，价格快照随代币流水落库可对账。
        上游密钥经 AES-256-GCM 加密落库，<b>只写不读</b>（编辑时留空表示不更换）。
        每种类型至少配置一个启用模型，用户端才会显示对应入口。
      </p>

      <!-- ============ 总开关 ============ -->
      <div class="admin-panel">
        <div class="enable-row">
          <span>启用 AI 功能</span>
          <el-switch v-model="config.enabled" @change="saveConfig" />
        </div>
        <p class="muted small" style="margin:0">关闭后用户端 AI 入口立即隐藏，进行中的调用不受影响。</p>
      </div>

      <!-- ============ 路由策略 ============ -->
      <div v-show="at('policy', 'route')" class="admin-panel" style="margin-top:16px">
        <SectionHeader title="路由策略" subtitle="自动选模型：最便宜 / 最快 / 按场景；失败自动降级并在输出里明确提示" :icon="'🧭'" gradient="blue" />
        <el-form label-width="120px" style="max-width:760px;margin-top:10px">
          <el-form-item label="全局策略">
            <el-radio-group v-model="route.strategy">
              <el-radio-button value="manual">手动（按优先级）</el-radio-button>
              <el-radio-button value="cheapest">最便宜</el-radio-button>
              <el-radio-button value="fastest">最快（近 7 天实测）</el-radio-button>
              <el-radio-button value="scene">按场景</el-radio-button>
            </el-radio-group>
          </el-form-item>
          <el-form-item v-if="route.strategy === 'scene'" label="场景策略">
            <div class="route-scenes">
              <div v-for="(v, k) in route.scene_strategy" :key="k" class="route-scene-row">
                <span class="route-scene-key">{{ k }}</span>
                <el-select v-model="route.scene_strategy[k]" size="small" style="width:150px">
                  <el-option label="最便宜" value="cheapest" />
                  <el-option label="最快" value="fastest" />
                  <el-option label="手动" value="manual" />
                </el-select>
              </div>
              <span class="muted small">场景：ai_hub（工作台）/ oc_turn（OC 发言）/ oc_publish（发布润色）/ group_bot（群内 AI）</span>
            </div>
          </el-form-item>
          <el-form-item label="失败降级">
            <el-switch v-model="route.fallback" />
            <el-input-number v-model="route.max_fallback" :min="1" :max="3" style="margin-left:12px" />
            <span class="muted small" style="margin-left:8px">最多切换几个备用模型；降级会在输出里标注「模型已降级：A → B（原因）」</span>
          </el-form-item>
          <el-form-item label="境内优先">
            <el-switch v-model="route.prefer_domestic" />
            <span class="muted small" style="margin-left:8px">仅影响排序（合规取向），不硬性排除境外模型</span>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="routeSaving" @click="saveRoute">保存策略</el-button>
            <span class="muted small" style="margin-left:10px">{{ routeHint }}</span>
          </el-form-item>
        </el-form>
      </div>

      <!-- ============ 自由添加（手工逐个建，不受渠道约束） ============ -->
      <div v-show="at('config', 'free')" class="admin-panel" style="margin-top:16px">
        <SectionHeader title="自由添加" subtitle="手工填上游地址与模型 ID，一个也行——不依赖渠道能否列出模型" :icon="'✍️'" gradient="indigo" />
        <div class="free-grid">
          <div class="free-card">
            <div class="free-step">1</div>
            <h5 class="free-title">填上游与模型 ID</h5>
            <p class="free-desc">
              适合上游不支持 <code>/models</code> 列表、或只想接某几个特定模型的情况。
              填完即用，密钥 AES-256-GCM 加密落库。
            </p>
            <el-button type="primary" :icon="Plus" @click="openCreate">手动添加模型</el-button>
          </div>
          <div class="free-card">
            <div class="free-step">2</div>
            <h5 class="free-title">也可以从渠道批量拉</h5>
            <p class="free-desc">
              上游是 OpenAI 兼容且开放 <code>/models</code> 时，填一次地址与密钥就能把可用模型全列出来勾选导入，
              免去逐个手填。
            </p>
            <el-button @click="openPull"><span class="btn-emoji">🔌</span>从渠道拉取</el-button>
          </div>
          <div class="free-card">
            <div class="free-step">3</div>
            <h5 class="free-title">调价与可见性</h5>
            <p class="free-desc">
              添加后在「按模型」里设价格与「导入后对用户可见」；
              对话按 1K tokens 计价，绘画/视频按次计价，0 = 免费。
            </p>
            <el-button @click="switchLevel1('config'); switchLevel2('model')">去按模型查看</el-button>
          </div>
        </div>
      </div>

      <!-- ============ 按渠道（渠道视角：地址 → 模型数 → 拉取/查余额） ============ -->
      <div v-show="at('config', 'channel')" class="admin-panel" style="margin-top:16px">
        <SectionHeader title="按渠道" subtitle="同一个上游地址下的模型共用一份密钥与余额；先看渠道，再决定拉什么" :icon="'🏢'" gradient="teal" />
        <div class="table-toolbar">
          <span class="muted small">共 {{ channelGroups.length }} 条渠道 · {{ models.length }} 个模型</span>
          <span>
            <el-button @click="openMonitor()">临时监控</el-button>
            <el-button :loading="chLoading" @click="openChannels">渠道余额</el-button>
            <el-button type="primary" @click="openPull"><span class="btn-emoji">🔌</span>新增渠道并拉取</el-button>
          </span>
        </div>
        <el-table v-if="channelGroups.length" :data="channelGroups" style="width:100%" size="small">
          <el-table-column prop="base_url" label="上游地址" min-width="280" show-overflow-tooltip />
          <el-table-column prop="models" label="模型数" width="90" />
          <el-table-column label="卡密池" width="130">
            <template #default="{ row }">
              <el-tag v-if="keyStats[row.base_url]" size="small"
                :type="keyStats[row.base_url].enabled > 0 ? 'success' : 'warning'">
                {{ keyStats[row.base_url].enabled }} / {{ keyStats[row.base_url].total }} 可用
              </el-tag>
              <el-tag v-else-if="row.hasKey" size="small" type="info">模型自带</el-tag>
              <el-tag v-else size="small" type="danger">未配置</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="250">
            <template #default="{ row }">
              <el-button link size="small" type="primary" @click="openChannelKeys(row.base_url)">卡密池</el-button>
              <!-- 渠道内直接监控：地址自动带过去，站长只需填令牌与用户 ID -->
              <el-button link size="small" @click="openMonitor(row.base_url)">监控</el-button>
              <el-button link size="small" @click="switchLevel2('model')">查看模型</el-button>
            </template>
          </el-table-column>
        </el-table>
        <StateBlock v-else state="empty" title="还没有渠道" description="点「新增渠道并拉取」填上游地址与密钥，即可批量导入模型" />
      </div>

      <!-- ============ 模型统计（三个维度共用一张表，按二级切换） ============ -->
      <div v-show="level1 === 'stats'" class="admin-panel" style="margin-top:16px">
        <SectionHeader
          :title="`用量统计 · 按${statsDim === 'channel' ? '渠道' : statsDim === 'vendor' ? '厂商' : '模型'}`"
          subtitle="近 30 天调用日志聚合：调用量 / tokens / 实扣代币 / 成功率 / 平均延迟"
          :icon="'📊'"
          gradient="blue"
        />
        <div class="stats-sum">
          <span>调用 <b>{{ usageTotals.req }}</b> 次</span>
          <span>tokens <b>{{ usageTotals.tok }}</b></span>
          <span>实扣代币 <b>{{ usageTotals.cost }}</b></span>
        </div>
        <el-table v-if="usageRows.length" :data="usageRows" v-loading="usageLoading" style="width:100%" size="small">
          <el-table-column prop="name" :label="statsDim === 'channel' ? '上游地址' : statsDim === 'vendor' ? '厂商' : '模型'" min-width="200" show-overflow-tooltip />
          <el-table-column v-if="statsDim !== 'model'" prop="model_count" label="模型数" width="80" />
          <el-table-column prop="requests" label="调用" width="80" sortable />
          <el-table-column label="成功率" width="90">
            <template #default="{ row }">
              <span :class="{ 'stat-bad': row.success_pct < 90 }">{{ row.success_pct }}%</span>
            </template>
          </el-table-column>
          <el-table-column label="tokens 入/出" width="150">
            <template #default="{ row }">{{ row.tokens_in }} / {{ row.tokens_out }}</template>
          </el-table-column>
          <el-table-column prop="cost" label="实扣代币" width="100" sortable />
          <el-table-column label="平均延迟" width="100">
            <template #default="{ row }">{{ row.avg_latency ? row.avg_latency + ' ms' : '—' }}</template>
          </el-table-column>
        </el-table>
        <StateBlock
          v-else-if="!usageLoading"
          state="empty"
          title="近 30 天没有调用记录"
          description="模型被实际调用后这里才会有数据；试算与失败重试不计入。"
        />
      </div>

      <!-- ============ 模型列表 ============ -->
      <div v-show="at('config', 'model')" class="admin-panel" style="margin-top:16px">
        <SectionHeader title="模型列表" subtitle="可配置多个上游，按优先级取用" :icon="'🤖'" gradient="purple" />
        <div class="table-toolbar">
          <span class="muted small">共 {{ models.length }} 个模型（启用 {{ enabledCount }} 个）</span>
          <el-button @click="openPull"><span class="btn-emoji">🔌</span>从渠道拉取</el-button>
          <el-button :loading="chLoading" @click="openChannels">渠道余额</el-button>
          <el-button type="primary" :icon="Plus" @click="openCreate">添加模型</el-button>
        </div>

        <!-- 空态：直接给可点的预设卡片。
             原来只有一行「暂无模型，点击右上角添加」，而"添加模型"弹窗里有 11 个字段要填
             （上游地址 / 模型标识 / API 协议 / 三档价格…）—— 多数站长在这里就放弃了，
             这也是「AI 开了但啥也没有」最常见的原因。 -->
        <div v-if="!loading && models.length === 0" class="pool-empty">
          <p class="pool-empty-tip">
            还没有模型。选一个预设<strong>一键创建</strong>，创建后到「密钥池」补上游密钥即可使用：
          </p>
          <div class="pool-preset-grid">
            <button
              v-for="p in quickPresets"
              :key="p.key"
              type="button"
              class="pool-preset"
              :disabled="quickAdding === p.key"
              @click="quickAdd(p)"
            >
              <span class="pool-preset-name">{{ p.name }}</span>
              <span class="pool-preset-meta">
                <el-tag size="small" :type="kindTag(p.kind)">{{ kindLabel(p.kind) }}</el-tag>
                <span v-if="p.vendor" class="pool-preset-vendor">{{ p.vendor }}</span>
              </span>
              <span class="pool-preset-price">
                {{ p.kind === 'chat' ? '按 tokens 计费' : `${p.price_call || 0} 代币 / 次` }}
              </span>
              <span v-if="!p.base_url" class="pool-preset-warn">需填中转地址</span>
              <span v-else class="pool-preset-ready">✓ 可一键创建</span>
            </button>
          </div>
          <div class="pool-empty-foot">
            <el-button text type="primary" @click="openCreate">手工添加（自定义上游）</el-button>
          </div>
        </div>

        <el-table v-else :data="models" v-loading="loading" style="width:100%">
          <el-table-column prop="name" label="展示名" min-width="110" show-overflow-tooltip />
          <el-table-column label="类型" width="80">
            <template #default="{ row }">
              <el-tag size="small" :type="kindTag(row.kind)">{{ kindLabel(row.kind) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="model_id" label="上游模型 ID" min-width="140" show-overflow-tooltip />
          <el-table-column prop="base_url" label="上游地址" min-width="180" show-overflow-tooltip />
          <el-table-column label="密钥" width="90">
            <template #default="{ row }">
              <el-tag size="small" :type="row.has_api_key ? 'success' : 'info'">
                {{ row.has_api_key ? '已配置' : '未配置' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="价格" min-width="150">
            <template #default="{ row }">
              <span v-if="row.kind === 'chat'" class="mono">{{ row.price_in || 0 }} / {{ row.price_out || 0 }}</span>
              <span v-else class="mono">{{ row.price_call || 0 }}</span>
              <span class="muted small">{{ row.kind === 'chat' ? '每 1K tokens 入/出' : '每次调用' }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="max_tokens" label="默认输出上限" width="120" />
          <el-table-column label="启用" width="80">
            <template #default="{ row }">
              <el-switch v-model="row.enabled" size="small" @change="toggleEnabled(row)" />
            </template>
          </el-table-column>
          <el-table-column prop="priority" label="优先级" width="80" />
          <el-table-column label="操作" width="268" fixed="right">
            <template #default="{ row }">
              <el-tooltip content="按模型的旧密钥池（兼容保留）。新配置请到「按渠道 → 卡密池」，同一渠道下的模型共用。" placement="top">
                <el-button size="small" text @click="openKeys(row)">旧密钥池</el-button>
              </el-tooltip>
              <el-button size="small" text type="warning" @click="openChannel(row)">对账</el-button>
              <el-button size="small" text type="primary" @click="openEdit(row)">编辑</el-button>
              <el-button size="small" text type="danger" @click="removeModel(row)">删除</el-button>
            </template>
          </el-table-column>
          <template #empty><EmptyState icon="🤖" text="暂无模型，点击右上角添加" /></template>
        </el-table>
      </div>
    </div>

    <!-- ============ 厂商管理 ============ -->
    <div v-show="at('config', 'vendor')" class="admin-panel" style="margin-top:16px">
      <SectionHeader title="厂商管理" subtitle="厂商 = 模型 ID 前缀（如 deepseek），按厂商聚合与筛选；导入时按前缀归类" :icon="'🏷️'" gradient="teal" />
      <div class="table-toolbar">
        <span class="muted small">共 {{ vendors.length }} 个厂商（启用 {{ enabledVendorCount }} 个）</span>
        <el-button type="primary" :icon="Plus" @click="openVendorCreate">添加厂商</el-button>
      </div>
      <el-table :data="vendors" v-loading="vendorLoading">
        <el-table-column prop="name" label="厂商名" min-width="120" show-overflow-tooltip />
        <el-table-column label="ID 前缀" min-width="130" show-overflow-tooltip>
          <template #default="{ row }"><code v-if="row.prefix">{{ row.prefix }}</code><span v-else class="muted">—</span></template>
        </el-table-column>
        <el-table-column prop="models" label="模型数" width="90" />
        <el-table-column label="启用" width="80">
          <template #default="{ row }"><el-switch v-model="row.enabled" size="small" @change="toggleVendor(row)" /></template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="openVendorEdit(row)">编辑</el-button>
            <el-button size="small" text type="danger" @click="removeVendor(row)">删除</el-button>
          </template>
        </el-table-column>
        <template #empty><EmptyState icon="🏷️" text="暂无厂商，点击右上角添加" /></template>
      </el-table>
    </div>

    <!-- ============ 厂商版本矩阵（管理端专用） ============ -->
    <div v-show="at('config', 'vendor')" class="admin-panel" style="margin-top:16px">
      <SectionHeader title="厂商版本矩阵" subtitle="按 厂商→版本→亚版本 聚合；同一版本下高亮最新亚版本（仅管理端可见，用户侧只看主名字）" :icon="'🧬'" gradient="indigo" />
      <div class="matrix-wrap" v-loading="matrixLoading">
        <div v-for="v in matrix" :key="v.id" class="matrix-vendor">
          <div class="matrix-vendor-head">
            <strong>{{ v.name }}</strong>
            <code v-if="v.prefix" class="matrix-prefix">{{ v.prefix }}</code>
            <span class="muted small">（{{ v.models }} 个模型）</span>
          </div>
          <div v-if="!v.versions.length" class="muted small" style="padding:6px 0">无版本信息</div>
          <div v-for="ver in v.versions" :key="ver.version" class="matrix-version">
            <div class="matrix-version-head">
              <span class="matrix-version-tag">{{ ver.version }}</span>
              <el-tag v-if="ver.newest_sub_version" size="small" type="success">最新亚版本：{{ ver.newest_sub_version }}</el-tag>
              <span class="muted small">{{ ver.count }} 个</span>
            </div>
            <div class="matrix-subs">
              <span
                v-for="it in ver.items"
                :key="it.id"
                class="matrix-sub"
                :class="{ 'matrix-sub-newest': it.newest }"
              >{{ it.sub_version || '（无亚版本）' }}<em v-if="it.newest">★</em></span>
            </div>
          </div>
        </div>
        <EmptyState v-if="!matrixLoading && !matrix.length" icon="🧬" text="暂无厂商版本数据" />
      </div>
    </div>

    <!-- ============ 聚合分类定价 ============ -->
    <div v-show="at('policy', 'pricing')" class="admin-panel" style="margin-top:16px">
      <SectionHeader title="聚合分类定价" subtitle="按 chat / image / video 三类设默认单价；导入模型未显式给价时自动套用" :icon="'💰'" gradient="amber" />
      <div class="price-grid">
        <div v-for="p in categoryPrices" :key="p.kind" class="price-card">
          <div class="price-card-head">
            <el-tag size="small" :type="kindTag(p.kind)">{{ kindLabel(p.kind) }}</el-tag>
            <span class="muted small">{{ p.kind }}</span>
          </div>
          <template v-if="p.kind === 'chat'">
            <div class="price-row">
              <span class="price-label">输入价</span>
              <el-input-number v-model="p.price_in" :min="0" :max="1000000" :step="1" controls-position="right" size="small" />
              <span class="muted small">每 1K tokens</span>
            </div>
            <div class="price-row">
              <span class="price-label">输出价</span>
              <el-input-number v-model="p.price_out" :min="0" :max="1000000" :step="1" controls-position="right" size="small" />
              <span class="muted small">每 1K tokens</span>
            </div>
          </template>
          <div v-else class="price-row">
            <span class="price-label">单次价</span>
            <el-input-number v-model="p.price_call" :min="0" :max="1000000" :step="1" controls-position="right" size="small" />
            <span class="muted small">每次调用</span>
          </div>
          <el-button size="small" type="primary" :loading="p._saving" @click="saveCategoryPrice(p)">保存</el-button>
        </div>
      </div>
    </div>

    <!-- ============ 从渠道拉取模型对话框（L5：输入渠道地址 + key 拉出模型清单） ============ -->
    <el-dialog v-model="pullDialogVisible" title="从渠道拉取模型" width="820px" :close-on-click-modal="false">
      <div class="pull-step">
        <el-alert type="info" :closable="false" show-icon
          style="margin-bottom:12px"
          title="输入上游渠道地址与密钥，一键拉出该渠道全部模型；勾选需要导入的，导入后默认对用户隐藏，到密钥池补密钥并手动开放可见性即可。"
        />
        <el-form label-width="110px" inline>
          <el-form-item label="上游地址" required style="width:420px">
            <el-input v-model="pullForm.base_url" placeholder="如 https://api.openai.com/v1（不含 /chat/completions）" />
          </el-form-item>
          <el-form-item label="API Key">
            <el-input v-model="pullForm.api_key" placeholder="可留空（部分渠道无需密钥）" style="width:240px" type="password" show-password />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="pulling" @click="doPull">拉取模型</el-button>
          </el-form-item>
        </el-form>

        <div v-if="pullError" class="pull-error">{{ pullError }}</div>

        <div v-if="pullList.length" class="pull-result">
          <div class="pull-result-head">
            <span class="muted small">共 {{ pullList.length }} 个模型，已选 {{ pullSelected.length }} 个</span>
            <span class="pull-result-ops">
              <el-popover placement="bottom-end" :width="230" trigger="click">
                <template #reference>
                  <el-button size="small">列显示</el-button>
                </template>
                <div class="pull-col-toggles">
                  <div class="pull-col-title">按需显示拆解字段</div>
                  <el-checkbox v-for="c in PULL_COLS" :key="c.key" v-model="pullCols[c.key]">{{ c.label }}</el-checkbox>
                  <div class="pull-col-tip">模型名会被拆成「家族 / 版本 / 类型 / 子号」四段，便于按档位与快照筛选</div>
                </div>
              </el-popover>
              <el-button size="small" @click="toggleAllPull(true)">全选</el-button>
              <el-button size="small" @click="toggleAllPull(false)">取消全选</el-button>
            </span>
          </div>
          <el-table ref="pullTableRef" :data="pullList" max-height="360" @selection-change="onPullSelection">
            <el-table-column type="selection" width="46" />
            <el-table-column prop="alias" label="展示别名" min-width="150" show-overflow-tooltip />
            <el-table-column prop="raw_id" label="上游模型 ID" min-width="180" show-overflow-tooltip />
            <el-table-column v-if="pullCols.family" prop="prefix" label="模型大名" width="110" show-overflow-tooltip />
            <el-table-column v-if="pullCols.version" prop="version" label="版本" width="80" />
            <el-table-column v-if="pullCols.variant" prop="variant" label="类型" width="120" show-overflow-tooltip />
            <el-table-column v-if="pullCols.build" prop="build" label="子号" width="110" show-overflow-tooltip />
            <el-table-column v-if="pullCols.vendor" prop="vendor" label="厂商" width="100" />
            <el-table-column v-if="pullCols.channel" prop="channel" label="渠道" width="100" />
            <el-table-column label="类别" width="100">
              <template #default="{ row }"><el-tag size="small" :type="kindTag(row.category)">{{ kindLabel(row.category) }}</el-tag></template>
            </el-table-column>
            <el-table-column label="导入后对用户可见" width="140">
              <template #default="{ row }"><el-switch v-model="row.user_visible" size="small" /></template>
            </el-table-column>
          </el-table>
          <div class="pull-import-bar">
            <el-input v-model="pullChannel" placeholder="渠道分组名（如 openai / 我的中转），留空按推断分组" style="width:260px" />
            <el-button type="success" :loading="importing" :disabled="!pullSelected.length" @click="doImport">导入选中（{{ pullSelected.length }}）</el-button>
          </div>
        </div>
      </div>
    </el-dialog>

    <!-- ============ 渠道卡密池（按渠道，模型共用） ============ -->
    <el-dialog v-model="ckDialog" title="渠道卡密池" width="780px">
      <div class="ch-overview-tip">
        卡密池归属<b>渠道</b>而不是模型：同一条渠道下的所有模型共用这个池，按权重轮询、连续失败自动摘除。
        模型只记录「走哪条渠道」。
        <br />当前渠道：<code class="mono">{{ ckBase }}</code>
      </div>

      <div class="key-add">
        <el-input v-model="ckForm.keys" type="textarea" :rows="3"
          placeholder="粘贴卡密，一行一把（也支持逗号分隔）；同渠道重复的会自动跳过" />
        <div class="key-add-row">
          <el-input v-model="ckForm.label" placeholder="标签（可选，留空自动编号）" style="width:200px" />
          <el-input-number v-model="ckForm.weight" :min="1" :max="100" style="width:120px" />
          <el-button type="primary" :loading="ckSaving" @click="ckAdd">添加</el-button>
          <span class="muted small">权重越大被选中概率越高</span>
        </div>
      </div>

      <el-table :data="ckList" size="small" max-height="300" stripe>
        <template #empty>
          <StateBlock state="empty" compact title="这个渠道还没有卡密"
            description="老站点升级会自动把模型里已存的密钥迁移过来；也可在上方粘贴新增。" />
        </template>
        <el-table-column prop="label" label="标签" min-width="130" show-overflow-tooltip />
        <el-table-column prop="hint" label="卡密" width="110">
          <template #default="{ row }"><span class="mono">{{ row.hint || '—' }}</span></template>
        </el-table-column>
        <el-table-column prop="weight" label="权重" width="70" />
        <el-table-column label="状态" width="150">
          <template #default="{ row }">
            <el-tag size="small" :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</el-tag>
            <span v-if="row.fail_count > 0" class="key-fail">连败 {{ row.fail_count }}</span>
          </template>
        </el-table-column>
        <el-table-column label="累计 成功/失败" width="130">
          <template #default="{ row }">
            <span class="muted small">{{ row.success_total }} / {{ row.fail_total }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="190">
          <template #default="{ row }">
            <el-button link size="small" :loading="row._testing" @click="ckTest(row)">测试</el-button>
            <el-button link size="small" @click="ckToggle(row)">{{ row.enabled ? '停用' : '启用' }}</el-button>
            <el-button link size="small" type="danger" @click="ckRemove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="openMonitor(ckBase)">监控这个渠道（临时令牌，不落库）</el-button>
        <el-button type="primary" @click="ckDialog = false">关闭</el-button>
      </template>
    </el-dialog>

    <!-- ============ 渠道监控（临时令牌，不落库） ============ -->
    <el-dialog v-model="monDialog" :title="monTitle" width="680px">
      <div class="ch-overview-tip">
        <template v-if="mon.locked">
          从渠道 <b>{{ mon.base_url }}</b> 进入 —— 地址已按该渠道填好，只需填令牌与用户 ID。
        </template>
        <template v-else>
          想先看看一条渠道「通不通 / 还剩多少 / 最近花了多少」，又不想把它存进模型池时用这里。
          也可以直接在「渠道余额一览」里点某条渠道的「监控」，地址会自动带过来。
        </template>
        <b>令牌只在本次查询里用，不写库、不写日志。</b>
      </div>
      <el-form label-width="96px" class="mon-form">
        <el-form-item label="渠道地址">
          <el-input v-model="mon.base_url" :disabled="mon.locked" placeholder="填站点根地址，如 https://api.example.com" />
          <span class="muted small">粘成 /api/user/self、/v1/models 这类具体接口也没关系，会自动还原成站点根</span>
        </el-form-item>
        <el-form-item label="令牌">
          <el-input v-model="mon.token" type="password" show-password
            placeholder="模型密钥 sk_ 或中转站系统访问令牌 nap_" />
          <span class="muted small">sk_ 只能查该密钥自身用量；账户余额要 nap_ 令牌 + 用户 ID</span>
        </el-form-item>
        <el-form-item label="用户 ID">
          <el-input v-model="mon.user_id" placeholder="中转站的数字用户 ID（New-Api-User），官方渠道留空" />
        </el-form-item>
        <el-form-item label="模型密钥">
          <el-input v-model="mon.model_key" type="password" show-password
            placeholder="可选，sk_ 开头；用来探测「这条渠道能拉到多少模型」" />
          <span class="muted small">账户令牌 nap_ 不能调模型接口，探模型列表要用 sk_</span>
        </el-form-item>
        <el-form-item label="查询方式">
          <el-select v-model="mon.kind" style="width:220px">
            <el-option label="自动判断" value="" />
            <el-option label="New API 中转站" value="newapi" />
            <el-option label="DeepSeek 官方" value="deepseek" />
            <el-option label="通用 / 自建" value="generic" />
          </el-select>
        </el-form-item>
      </el-form>

      <div v-if="monResult" class="mon-result">
        <!-- 自动纠正过什么必须说出来，否则站长不知道自己的填法有问题 -->
        <div v-if="monResult.warnings && monResult.warnings.length" class="mon-warn-list">
          <div v-for="(w, i) in monResult.warnings" :key="i" class="mon-warn-item">已自动纠正：{{ w }}</div>
        </div>
        <div class="mon-row">
          <span class="mon-k">连通性</span>
          <el-tag :type="monResult.probe_ok ? 'success' : (monResult.probe_skipped ? 'info' : 'danger')" size="small">
            {{ monResult.probe_ok ? '正常' : (monResult.probe_skipped ? '已跳过' : '失败') }}
          </el-tag>
          <span class="muted small" v-if="monResult.probe_ok">可用模型 {{ monResult.model_count }} 个</span>
          <span class="mon-bad small" v-else>{{ monResult.probe_error }}</span>
        </div>
        <div class="mon-row">
          <span class="mon-k">账户余额</span>
          <template v-if="monResult.balance_ok">
            <b class="ch-est">{{ fmtMoney(monResult.remaining) }} <em class="unit">{{ monResult.currency || '' }}</em></b>
            <span class="muted small">已消费 {{ fmtMoney(monResult.used) }}</span>
          </template>
          <span v-else class="mon-bad small">{{ monResult.balance_error }}</span>
        </div>
        <div class="mon-row" v-if="monResult.recent_logs && monResult.recent_logs.length">
          <span class="mon-k">最近调用</span>
          <div class="mon-logs">
            <div v-for="(l, i) in monResult.recent_logs.slice(0, 3)" :key="i" class="mon-log">
              <span class="mon-model">{{ l.model_name || '—' }}</span>
              <span class="ch-est">≈{{ fmtMoney(l.quota) }} <em class="unit">{{ monResult.currency || 'USD' }}</em></span>
              <span class="muted small">{{ fmtTok(l.tokens_in) }} / {{ fmtTok(l.tokens_out) }}</span>
            </div>
          </div>
        </div>
        <div class="mon-row" v-else-if="monResult.logs_error">
          <span class="mon-k">最近调用</span>
          <span class="muted small">{{ monResult.logs_error }}</span>
        </div>
      </div>

      <template #footer>
        <el-button @click="monDialog = false">关闭</el-button>
        <el-button type="primary" :loading="monLoading" @click="runMonitor">查询</el-button>
      </template>
    </el-dialog>

    <!-- ============ 渠道余额一览（按渠道聚合） ============ -->
    <el-dialog v-model="chDialog" title="渠道余额一览" width="980px">
      <!-- 换算比例条：中转站的「美元」只是站内代币，不说清比例就没有成本含义 -->
      <div class="ch-rate-bar" :class="{ unset: !rateValid }">
        <span class="ch-rate-icon">{{ rateValid ? '💱' : '⚠️' }}</span>
        <div class="ch-rate-text">
          <template v-if="rateValid">
            <b>换算比例</b> {{ rateDescribe }}
            <span class="muted small">（虚拟金额 × {{ rateRatio }} = 真实金额）</span>
          </template>
          <template v-else>
            <b>未设置换算比例</b>
            <span class="muted small">
              中转站的「美元」只是站内代币：充 100 元可能到账 1000 虚拟美元。
              不填比例，下面的金额只能算「游戏厅代币数」，代表不了你真花了多少钱。
            </span>
          </template>
        </div>
        <el-button size="small" type="primary" plain @click="openRateEdit">设置比例</el-button>
      </div>

      <div class="ch-overview-tip">
        按上游渠道（base_url）聚合，每条渠道查一次账户余额 —— 回答「这条渠道还剩多少钱」。
        同一渠道下的多个模型共用一份余额，所以不逐模型重复查询。
        带「实」的小字是按上述比例折算出的**真实成本**。
        <br />
        <b>最近一次</b> = 中转站使用日志里最新一条的实际扣费（精确，需系统访问令牌含「查看日志」权限）；
        <b>估算单次</b> = 最近两次采样的余额消耗 ÷ 窗口内该渠道的请求数；<b>单次 token</b> 与
        <b>每千 token 单价</b> 用来区分「单价贵」和「用得多」。上游不按请求返回金额，
        所以金额是估算值（并发与重试会摊进去）；多点几次「渠道余额」或等后台每 10 分钟自动采样即可攒出窗口。
        token 数取自本站调用日志，是真实值。
      </div>
      <el-table v-if="chRows.length" :data="chRows" size="small" stripe max-height="420">
        <el-table-column prop="base_url" label="渠道" min-width="220" show-overflow-tooltip />
        <el-table-column prop="models" label="模型数" width="80" />
        <el-table-column label="剩余" width="132">
          <template #default="{ row }">
            <div class="ch-dual">
              <span v-if="row.supported" :class="['ch-bal', { zero: !row.total_remaining }]">{{ fmtMoney(row.total_remaining) }}</span>
              <span v-else class="muted small">查不到</span>
              <span v-if="row.real_remaining !== undefined" class="ch-real">
                实 {{ fmtMoney(row.real_remaining) }} <em class="unit">{{ row.rate?.real_currency }}</em>
              </span>
              <span v-else-if="!row.rate" class="ch-noreal">未设比例</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="已消费" width="132">
          <template #default="{ row }">
            <div class="ch-dual">
              <span v-if="row.supported">{{ fmtMoney(row.total_used) }}</span>
              <span v-else class="muted small">—</span>
              <span v-if="row.real_used !== undefined" class="ch-real">
                实 {{ fmtMoney(row.real_used) }} <em class="unit">{{ row.rate?.real_currency }}</em>
              </span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="币种" width="66">
          <template #default="{ row }">{{ row.currency || '—' }}</template>
        </el-table-column>
        <el-table-column label="期间消耗" width="126">
          <template #default="{ row }">
            <div v-if="row.window?.has_data" class="ch-dual">
              <span>{{ fmtMoney(row.window.cost_used) }} <em class="unit">{{ row.window.currency || '' }}</em></span>
              <span v-if="row.real_window_cost !== undefined" class="ch-real">
                实 {{ fmtMoney(row.real_window_cost) }} <em class="unit">{{ row.rate?.real_currency }}</em>
              </span>
            </div>
            <span v-else class="muted small">需≥2次采样</span>
          </template>
        </el-table-column>
        <el-table-column label="期间请求" width="84">
          <template #default="{ row }">
            <span v-if="row.window?.has_data">{{ row.window.requests }}</span>
            <span v-else class="muted small">—</span>
          </template>
        </el-table-column>
        <el-table-column label="估算单次" width="126">
          <template #default="{ row }">
            <div v-if="row.window?.per_request" class="ch-dual">
              <span class="ch-est">≈{{ fmtMoney(row.window.per_request) }} <em class="unit">{{ row.window.currency || '' }}</em></span>
              <span v-if="row.real_per_request !== undefined" class="ch-real">
                实≈{{ fmtMoney(row.real_per_request) }} <em class="unit">{{ row.rate?.real_currency }}</em>
              </span>
            </div>
            <span v-else class="muted small">—</span>
          </template>
        </el-table-column>
        <el-table-column label="最近一次" width="136">
          <template #default="{ row }">
            <div v-if="row.last_cost !== undefined && row.last_cost !== null" class="ch-dual">
              <span class="ch-est">{{ fmtMoney(row.last_cost) }} <em class="unit">{{ row.currency || 'USD' }}</em></span>
              <span v-if="row.real_last_cost !== undefined" class="ch-real">
                实 {{ fmtMoney(row.real_last_cost) }} <em class="unit">{{ row.rate?.real_currency }}</em>
              </span>
            </div>
            <span v-else-if="row.logs_error" class="muted small" :title="row.logs_error">无日志权限</span>
            <span v-else class="muted small">—</span>
          </template>
        </el-table-column>
        <el-table-column label="期间 token" width="150">
          <template #default="{ row }">
            <span v-if="row.window?.has_data" class="ch-tok">
              {{ fmtTok(row.window.tokens_in) }} / {{ fmtTok(row.window.tokens_out) }}
            </span>
            <span v-else class="muted small">—</span>
          </template>
        </el-table-column>
        <el-table-column label="单次 token" width="100">
          <template #default="{ row }">
            <span v-if="row.window?.per_request_tok">{{ fmtTok(row.window.per_request_tok) }}</span>
            <span v-else class="muted small">—</span>
          </template>
        </el-table-column>
        <el-table-column label="每千 token 单价" width="140">
          <template #default="{ row }">
            <span v-if="row.window?.cost_per_1k_tok" class="ch-est">
              ≈{{ fmtMoney(row.window.cost_per_1k_tok) }} <em class="unit">{{ row.window.currency || '' }}</em>
            </span>
            <span v-else class="muted small">—</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" min-width="150">
          <template #default="{ row }">
            <el-tag v-if="row.supported" size="small" type="success">可查</el-tag>
            <span v-else class="muted small">{{ row.error || '该渠道无余额端点' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="86" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" size="small" @click="openMonitor(row.base_url)">监控</el-button>
          </template>
        </el-table-column>
      </el-table>
      <StateBlock v-else-if="!chLoading" state="empty" title="没有可查询的渠道" description="先在模型里配置上游地址与密钥" />
      <template #footer>
        <el-button :loading="chLoading" @click="openChannels">重新查询</el-button>
        <el-button type="primary" @click="chDialog = false">关闭</el-button>
      </template>
    </el-dialog>

    <!-- ============ 真实成本换算比例 ============ -->
    <el-dialog v-model="rateDialog" title="真实成本换算比例" width="560px" append-to-body>
      <div class="ch-rate-help">
        中转站后台显示的「美元」只是<b>站内代币</b>，和你实际充的钱不是一回事。
        请按你的真实充值单填写，例如：<b>充 100 元人民币 → 到账 1000 虚拟美元</b>，
        就填 <code>100 CNY = 1000 虚拟美元</code>。官方渠道（按真实美元计费）填 1 = 1 即可。
      </div>
      <el-form label-width="118px" class="ch-rate-form">
        <el-form-item label="换算比例">
          <el-input-number v-model="rateForm.real_amount" :min="0" :step="1" :precision="4" :controls="false" style="width:130px" />
          <el-select v-model="rateForm.real_currency" style="width:96px;margin:0 6px">
            <el-option v-for="c in realCurrencies" :key="c" :label="c" :value="c" />
          </el-select>
          <span class="ch-rate-eq">=</span>
          <el-input-number v-model="rateForm.virtual_amount" :min="0" :step="1" :precision="4" :controls="false" style="width:130px" />
          <el-input v-model="rateForm.virtual_currency" placeholder="虚拟美元" style="width:120px;margin-left:6px" />
        </el-form-item>
        <el-form-item label="试算">
          <el-input-number v-model="ratePreviewIn" :min="0" :step="100" :controls="false" style="width:130px" />
          <span class="muted small" style="margin:0 8px">虚拟单位 →</span>
          <span v-if="ratePreviewOut !== null" class="ch-real" style="font-size:0.9rem">
            {{ fmtMoney(ratePreviewOut) }} <em class="unit">{{ rateForm.real_currency }}</em>
          </span>
          <span v-else class="ch-noreal">比例不完整，无法换算</span>
        </el-form-item>
        <el-form-item label=" ">
          <span class="muted small">
            留空 / 两个都填 0 = 不换算，界面上就不会出现「实」字金额（宁可不显示，也不给你一个假成本）。
          </span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="rateDialog = false">取消</el-button>
        <el-button type="primary" :loading="rateSaving" @click="saveRate">保存</el-button>
      </template>
    </el-dialog>

    <!-- ============ L5 渠道对账：余额 / 已消费 / 剩余 + 连通性 ============ -->
    <el-dialog v-model="channelDialog" title="渠道对账（余额 · 消费 · 连通性）" width="640px">
      <div v-if="channelLoading" class="ch-loading">正在向该渠道发起真实请求…</div>
      <template v-else-if="channelReport">
        <div class="ch-head">
          <div>
            <div class="ch-name">{{ channelReport.model_name }}</div>
            <div class="ch-meta">{{ channelReport.base_url }}</div>
          </div>
          <el-tag :type="channelReport.balance.supported ? 'success' : 'info'" size="small">
            {{ channelReport.balance.supported ? '支持余额查询' : '该渠道无余额端点' }}
          </el-tag>
        </div>

        <div class="ch-cards">
          <div class="ch-card">
            <div class="ch-card-k">剩余</div>
            <div class="ch-card-v">{{ fmtMoney(channelReport.balance.total_remaining) }}</div>
            <div class="ch-card-u">{{ channelReport.balance.currency || '—' }}</div>
          </div>
          <div class="ch-card">
            <div class="ch-card-k">已消费</div>
            <div class="ch-card-v used">{{ fmtMoney(channelReport.balance.total_used) }}</div>
            <div class="ch-card-u">{{ channelReport.balance.currency || '—' }}</div>
          </div>
          <div class="ch-card">
            <div class="ch-card-k">累计充值 / 授予</div>
            <div class="ch-card-v">{{ fmtMoney(channelReport.balance.total_granted) }}</div>
            <div class="ch-card-u">{{ channelReport.balance.currency || '—' }}</div>
          </div>
        </div>

        <el-descriptions :column="2" size="small" border style="margin-top:12px">
          <el-descriptions-item label="渠道类型">{{ channelReport.provider || '—' }}</el-descriptions-item>
          <el-descriptions-item label="查询时间">{{ channelReport.balance.fetched_at || '—' }}</el-descriptions-item>
          <el-descriptions-item label="连通性">
            <el-tag :type="channelReport.probe.reachable ? 'success' : 'danger'" size="small">
              {{ channelReport.probe.reachable ? '可达' : '不可达' }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="延迟">{{ channelReport.probe.latency_ms }} ms</el-descriptions-item>
          <el-descriptions-item label="HTTP 状态">{{ channelReport.probe.status_code || '—' }}</el-descriptions-item>
          <el-descriptions-item label="可用模型数">{{ channelReport.probe.models ?? '—' }}</el-descriptions-item>
        </el-descriptions>

        <el-alert
          v-if="channelReport.balance.error || channelReport.probe.error"
          type="warning" :closable="false" show-icon style="margin-top:10px"
          :title="channelReport.balance.error || channelReport.probe.error"
        />
        <div class="ch-tip">余额查询仅对 OpenAI 兼容渠道有效；部分中转渠道不提供该端点属正常现象，连通性仍可验证。</div>
      </template>
      <template #footer>
        <el-button :loading="channelLoading" @click="refreshChannel">重新查询</el-button>
        <el-button type="primary" @click="channelDialog = false">关闭</el-button>
      </template>
    </el-dialog>

    <!-- ============ 创建 / 编辑对话框 ============ -->
    <el-dialog v-model="dialogVisible" :title="editing ? '编辑模型' : '添加模型'" width="560px">
      <el-form :model="form" label-width="110px">
        <el-form-item label="类型" required>
          <el-radio-group v-model="form.kind">
            <el-radio-button value="chat">对话</el-radio-button>
            <el-radio-button value="image">绘画</el-radio-button>
            <el-radio-button value="video">视频</el-radio-button>
          </el-radio-group>
          <div class="kind-hint">{{ kindHint(form.kind) }}</div>
        </el-form-item>
        <el-form-item label="展示名" required>
          <el-input v-model="form.name" placeholder="作者可见的名称，如：默认模型" maxlength="64" />
        </el-form-item>
        <el-form-item label="上游模型 ID" required>
          <el-input v-model="form.model_id" placeholder="如 gpt-4o-mini / deepseek-chat" maxlength="128" />
        </el-form-item>
        <el-form-item label="上游地址" required>
          <el-input v-model="form.base_url" placeholder="如 https://api.openai.com/v1（不含 /chat/completions）" />
        </el-form-item>
        <el-form-item label="快速预设">
          <el-select v-model="presetKey" clearable filterable placeholder="选择厂商预设，一键填充上游参数" style="width:100%" @change="applyPreset">
            <el-option v-for="p in presets" :key="p.key" :label="`${p.name} · ${kindLabel(p.kind)}`" :value="p.key" />
          </el-select>
          <div v-if="presetNote" class="kind-hint">{{ presetNote }}</div>
        </el-form-item>
        <el-form-item label="上游协议">
          <el-radio-group v-model="form.api_kind">
            <el-radio-button value="openai">OpenAI 兼容</el-radio-button>
            <el-radio-button value="claude">Claude</el-radio-button>
            <el-radio-button v-if="form.kind !== 'chat'" value="mj">MJ 代理</el-radio-button>
            <el-radio-button v-if="form.kind !== 'chat'" value="gemini">Gemini</el-radio-button>
          </el-radio-group>
          <div class="kind-hint">{{ apiKindHint(form.api_kind) }}</div>
        </el-form-item>
        <el-form-item label="厂商">
          <el-input v-model="form.vendor" placeholder="如 OpenAI / Anthropic / Google / 字节跳动（用于前台厂商筛选）" maxlength="32" />
        </el-form-item>
        <el-form-item label="上游归属">
          <el-select v-model="form.provider_hint" clearable placeholder="未标注" style="width:200px">
            <el-option label="境内（domestic）" value="domestic" />
            <el-option label="境外（overseas）" value="overseas" />
          </el-select>
          <span class="muted small">用于合规提示：境外端点会提示数据出境风险</span>
        </el-form-item>
        <el-form-item label="API Key">
          <el-input
            v-model="form.api_key"
            type="password"
            show-password
            :placeholder="editing ? '留空表示不更换（当前已加密存储）' : '上游密钥，加密后存储'"
          />
        </el-form-item>
        <el-form-item label="余额查询方式">
          <el-select v-model="form.balance_provider" style="width:220px">
            <el-option label="自动（按域名推断）" value="" />
            <el-option label="DeepSeek 官方（/user/balance）" value="deepseek" />
            <el-option label="New API 中转站（nap_ + 用户ID）" value="newapi" />
            <el-option label="自定义规则（自己填 URL 与字段）" value="custom" />
            <el-option label="不查询余额" value="none" />
          </el-select>
          <span class="muted small">官方与中转站逻辑完全不同，建议显式选择，别依赖猜</span>
        </el-form-item>
        <template v-if="form.balance_provider === 'custom'">
          <el-form-item label="余额接口 URL">
            <el-input v-model="form.balance_url" placeholder="如 https://api.example.com/stat/credit" />
          </el-form-item>
          <el-form-item label="取哪个字段">
            <el-input v-model="form.balance_field" placeholder="如 data.credit.available；用点号逐层，支持数组下标" />
            <span class="muted small">最终金额 =（原始值 ÷ 除数）× 乘数</span>
          </el-form-item>
          <el-form-item label="除数 / 乘数">
            <el-input-number v-model="form.balance_divisor" :min="0" :controls="false" placeholder="除数，如 500000" style="width:150px" />
            <span class="muted small" style="margin:0 6px">×</span>
            <el-input-number v-model="form.balance_multiplier" :min="0" :controls="false" placeholder="乘数，通常 1" style="width:150px" />
            <el-input v-model="form.balance_currency" placeholder="单位，如 USD" style="width:110px;margin-left:8px" />
          </el-form-item>
        </template>
        <el-form-item label="系统访问令牌">
          <el-input v-model="form.account_token" type="password" show-password
            :placeholder="editing ? '留空表示不更换（nap_ 开头，查账户余额用）' : 'nap_ 开头的系统访问令牌；sk_ 查不到账户余额'" />
          <span class="muted small">New API 体系里 sk_ 只能查该令牌自身用量；账户剩余需 nap_ 令牌 + 用户 ID 调 /api/user/self</span>
        </el-form-item>
        <el-form-item label="令牌用户 ID">
          <el-input v-model="form.token_user_id" placeholder="中转站（one-api/new-api）的用户 ID，如 56347；官方渠道留空" />
          <span class="muted small">中转站查额度要用「访问令牌 + 用户 ID」成对认证（New-Api-User 头）</span>
        </el-form-item>
        <el-form-item label="本渠道成本比例">
          <el-input-number v-model="form.rate_real_amount" :min="0" :precision="4" :controls="false" style="width:120px" />
          <el-select v-model="form.rate_real_currency" style="width:92px;margin:0 6px">
            <el-option v-for="c in realCurrencies" :key="c" :label="c" :value="c" />
          </el-select>
          <span class="ch-rate-eq">=</span>
          <el-input-number v-model="form.rate_virtual_amount" :min="0" :precision="4" :controls="false" style="width:120px" />
          <span class="muted small" style="margin-left:6px">虚拟单位</span>
          <span class="muted small">
            两数都填 0 = 跟随站点全局比例。例：充 100 元得 1000 虚拟美元 → 100 = 1000
          </span>
        </el-form-item>
        <template v-if="form.kind === 'chat'">
          <el-form-item label="输入价格">
            <el-input-number v-model="form.price_in" :min="0" :max="1000000" :step="1" controls-position="right" />
            <span class="muted small">每 1K 输入 tokens 代币数，0 = 免费</span>
          </el-form-item>
          <el-form-item label="输出价格">
            <el-input-number v-model="form.price_out" :min="0" :max="1000000" :step="1" controls-position="right" />
            <span class="muted small">每 1K 输出 tokens 代币数，0 = 免费</span>
          </el-form-item>
          <el-form-item label="默认输出上限">
            <el-input-number v-model="form.max_tokens" :min="256" :max="8192" :step="256" controls-position="right" />
            <span class="muted small">tokens，单次调用上限 8192</span>
          </el-form-item>
          <el-form-item label="温度">
            <el-input-number v-model="form.temperature" :min="0" :max="2" :step="0.1" controls-position="right" />
          </el-form-item>
        </template>
        <el-form-item v-else label="单次价格">
          <el-input-number v-model="form.price_call" :min="0" :max="1000000" :step="1" controls-position="right" />
          <span class="muted small">每次生成收取代币数，0 = 免费（{{ form.kind === 'image' ? '绘画' : '视频' }}按次计费）</span>
        </el-form-item>
        <el-form-item label="优先级">
          <el-input-number v-model="form.priority" :min="0" :max="999" :step="1" controls-position="right" />
          <span class="muted small">大者优先（备用池预留）</span>
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>
        <el-form-item v-if="form.kind === 'chat'" label="思考型">
          <el-switch v-model="form.thinking" />
          <span class="muted small">
            先输出推理过程再给正文（DeepSeek-R1 / GLM / Qwen thinking 等）。
            勾选后前端会把思考过程折叠展示，不会因为「正文来晚了」被当成卡死
          </span>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.note" placeholder="选填" maxlength="256" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveModel">保存</el-button>
      </template>
    </el-dialog>

    <!-- ============ API 卡池（多密钥轮询） ============ -->
    <el-drawer v-model="keysVisible" :title="`密钥池 · ${keysModel?.name || ''}`" size="min(720px, 94vw)">
      <div class="kp-tip">
        多把密钥按权重随机轮询；单把密钥连续失败 {{ maxFail }} 次自动停用（可手动重置）。
        池为空时自动回退到模型自带的单一密钥。
      </div>

      <div class="kp-add">
        <el-input v-model="keyForm.label" size="small" placeholder="备注（可选）" style="width:150px" />
        <el-input-number v-model="keyForm.weight" :min="1" :max="100" size="small" style="width:110px" />
        <el-input
          v-model="keyForm.keys"
          type="textarea"
          :rows="2"
          size="small"
          placeholder="粘贴密钥；支持一次多行批量导入（每行一把，最多 50 行）"
          style="flex:1"
        />
        <el-button type="primary" size="small" :loading="keySaving" @click="addKeys">添加</el-button>
      </div>

      <el-table :data="keys" size="small" v-loading="keyLoading">
        <el-table-column prop="label" label="备注" width="120" show-overflow-tooltip />
        <el-table-column prop="hint" label="密钥" width="110">
          <template #default="{ row }"><code>{{ row.hint || '—' }}</code></template>
        </el-table-column>
        <el-table-column label="权重" width="110">
          <template #default="{ row }">
            <el-input-number
              :model-value="row.weight"
              :min="1"
              :max="100"
              size="small"
              controls-position="right"
              style="width:80px"
              @change="(v: number) => updateKey(row, { weight: v })"
            />
          </template>
        </el-table-column>
        <el-table-column label="失败次数" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="row.fail_count >= maxFail ? 'danger' : row.fail_count > 0 ? 'warning' : 'success'">
              {{ row.fail_count }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="启用" width="80">
          <template #default="{ row }">
            <el-switch :model-value="row.enabled" size="small" @change="(v: boolean) => updateKey(row, { enabled: v })" />
          </template>
        </el-table-column>
        <el-table-column prop="last_used_at" label="最近使用" width="150">
          <template #default="{ row }">{{ row.last_used_at ? new Date(row.last_used_at).toLocaleString() : '—' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text type="primary" :loading="testingId === row.id" @click="testKey(row)">测试</el-button>
            <el-button size="small" text @click="updateKey(row, { reset: true })">重置</el-button>
            <el-button size="small" text type="danger" @click="removeKey(row)">删除</el-button>
          </template>
        </el-table-column>
        <template #empty><EmptyState icon="🔑" text="尚未添加密钥（当前使用模型自带密钥）" /></template>
      </el-table>
    </el-drawer>

    <!-- ============ 厂商创建 / 编辑对话框 ============ -->
    <el-dialog v-model="vendorDialog" :title="vendorEditing ? '编辑厂商' : '添加厂商'" width="440px">
      <el-form label-width="90px">
        <el-form-item label="厂商名" required>
          <el-input v-model="vendorForm.name" placeholder="如 DeepSeek / 阿里通义 / 字节跳动" maxlength="32" />
        </el-form-item>
        <el-form-item label="ID 前缀" required>
          <el-input v-model="vendorForm.prefix" placeholder="如 deepseek（模型 ID 约定：前缀-版本-亚版本）" maxlength="64" />
          <div class="kind-hint">模型 ID 约定：前缀-版本-亚版本（如 deepseek-v3.1-turbo），导入时按此前缀归类，用户侧只看厂商主名字</div>
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="vendorForm.sort_order" :min="0" :max="999" controls-position="right" />
        </el-form-item>
        <el-form-item label="启用" v-if="vendorEditing">
          <el-switch v-model="vendorForm.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="vendorDialog = false">取消</el-button>
        <el-button type="primary" :loading="vendorSaving" @click="saveVendor">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { ElMessage, ElMessageBox } from 'element-plus';
import { MagicStick, Plus } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import SectionHeader from '@/components/common/SectionHeader.vue';
import EmptyState from '@/components/common/EmptyState.vue';
import { aiApi, type AdminAiModel, type AiModelPreset, type AiVendor, type AiCategoryPrice, type AiVendorMatrixEntry, type ProviderModelSuggestion, type AdminChannelReport, type CostRateSetting } from '@/api/ai';
import { adminApi } from '@/api/admin';
import StateBlock from '@/components/common/StateBlock.vue';

// ==================== 两级导航（URL query 参数） ====================
// 一级决定"做什么"，二级决定"从哪个角度看"。两层都用 query 参数表达：
//   /admin/ai-pool?tab=config&view=channel   —— 模型配置 · 按渠道
//   /admin/ai-pool?tab=stats&view=model      —— 模型统计 · 按模型
// 用 query 而不是 #hash：参数能随路由一起被 vue-router 管理，
// 浏览器前进/后退、直接粘贴链接、外部跳转都能正确复位；
// 也与后台已有的 ?item= 深链约定一致，不会出现两套地址风格。
type L1 = 'config' | 'stats' | 'policy';
type L2 = 'free' | 'channel' | 'model' | 'vendor' | 'route' | 'pricing';

// 一级 → 可用二级（顺序即按钮顺序）
const LEVEL2_MAP: Record<L1, { key: L2; label: string }[]> = {
  config: [
    { key: 'free', label: '自由添加' },
    { key: 'channel', label: '按渠道' },
    { key: 'model', label: '按模型' },
    { key: 'vendor', label: '按厂商' },
  ],
  stats: [
    { key: 'channel', label: '按渠道' },
    { key: 'model', label: '按模型' },
    { key: 'vendor', label: '按厂商' },
  ],
  policy: [
    { key: 'route', label: '路由策略' },
    { key: 'pricing', label: '分类定价' },
  ],
};

// 注意：本页另有一个 route = reactive({...}) 表示「模型路由策略」，
// 所以路由对象改名为 navRoute/navRouter，避免标识符冲突（构建期直接报重复声明）。
const navRoute = useRoute();
const navRouter = useRouter();

function parseQuery(): { l1: L1; l2: L2 } {
  const a = String(navRoute.query.tab || '');
  const b = String(navRoute.query.view || '');
  if (a === 'config' || a === 'stats' || a === 'policy') {
    const list = LEVEL2_MAP[a as L1];
    const hit = list.find((x) => x.key === b);
    // 二级缺失或不匹配时取该项的第一个，避免"一级选中但下面空白"
    return { l1: a as L1, l2: (hit ? hit.key : list[0].key) };
  }
  return { l1: 'config', l2: 'model' };
}

const initialRoute = parseQuery();
const level1 = ref<L1>(initialRoute.l1);
const level2 = ref<L2>(initialRoute.l2);

const level1Tabs = [
  { key: 'config' as L1, label: '模型配置', icon: '⚙️' },
  { key: 'stats' as L1, label: '模型统计', icon: '📊' },
  { key: 'policy' as L1, label: '路由与定价', icon: '🧭' },
];

// 二级按钮带计数，省得逐个点开确认
const level2Tabs = computed(() => {
  const base = LEVEL2_MAP[level1.value];
  const counts: Partial<Record<L2, number | null>> = {
    free: null,
    channel: channelGroups.value.length,
    model: models.value.length,
    vendor: vendors.value.length,
    route: null,
    pricing: categoryPrices.value.length,
  };
  return base.map((t) => ({ ...t, count: counts[t.key] ?? null }));
});

/** 当前显示的是否为「一级/二级」这一格 */
function at(l1: L1, l2: L2) {
  return level1.value === l1 && level2.value === l2;
}

function syncQuery() {
  // 保留其它 query（如侧栏深链用的 ?item=），只覆盖本页这两个参数；
  // replace 而非 push：切 tab 不该在浏览历史里堆一串中间态
  navRouter.replace({ query: { ...navRoute.query, tab: level1.value, view: level2.value } });
}

function switchLevel1(k: L1) {
  level1.value = k;
  // 切一级时回到它的第一个二级，避免停留在上一级的二级项上（比如 policy/route）
  const list = LEVEL2_MAP[k];
  if (!list.some((x) => x.key === level2.value)) level2.value = list[0].key;
  syncQuery();
  if (k === 'stats') loadUsageStats();
}

function switchLevel2(k: L2) {
  level2.value = k;
  syncQuery();
  if (level1.value === 'stats') loadUsageStats();
}

// 地址栏被外部改动（粘贴链接、前进/后退）时同步视图
watch(
  () => [navRoute.query.tab, navRoute.query.view],
  () => {
    const { l1, l2 } = parseQuery();
    if (l1 === level1.value && l2 === level2.value) return;
    level1.value = l1;
    level2.value = l2;
    if (l1 === 'stats') loadUsageStats();
  },
);

// ---------- 按渠道视图（从已有模型聚合出渠道列表，不额外请求） ----------
const channelGroups = computed(() => {
  const map = new Map<string, { base_url: string; models: number; hasKey: boolean }>();
  for (const m of models.value) {
    const k = (m.base_url || '').trim() || '(未配置上游)';
    if (!map.has(k)) map.set(k, { base_url: k, models: 0, hasKey: false });
    const g = map.get(k)!;
    g.models += 1;
    if ((m as any).has_api_key) g.hasKey = true;
  }
  return [...map.values()].sort((a, b) => b.models - a.models);
});

// ---------- 模型统计 ----------
const statsDim = computed<L2>(() => (level2.value === 'model' || level2.value === 'vendor' ? level2.value : 'channel'));
const usageLoading = ref(false);
const usageRows = ref<any[]>([]);
const usageTotals = reactive({ req: 0, tok: 0, cost: 0 });

async function loadUsageStats() {
  if (level1.value !== 'stats') return;
  usageLoading.value = true;
  try {
    const res = await aiApi.usageStats(statsDim.value, 30);
    if (res.data.code === 0) {
      const d = res.data.data;
      usageRows.value = d.rows || [];
      usageTotals.req = d.total_req || 0;
      usageTotals.tok = d.total_tok || 0;
      usageTotals.cost = d.total_cost || 0;
    }
  } catch {
    ElMessage.error('统计加载失败');
  } finally {
    usageLoading.value = false;
  }
}

// ---------- 总开关 ----------
const config = reactive<{ enabled: boolean }>({ enabled: false });

async function loadConfig() {
  try {
    const res = await aiApi.getConfig();
    if (res.data.code === 0) config.enabled = res.data.data.enabled;
  } catch { /* ignore */ }
}

async function saveConfig(v: boolean) {
  try {
    const res = await aiApi.setConfig({ enabled: v });
    if (res.data.code === 0) ElMessage.success(v ? 'AI 已开启' : 'AI 已关闭');
    else config.enabled = !v; // 失败回滚开关
  } catch {
    config.enabled = !v;
    ElMessage.error('保存失败');
  }
}

// ---------- 模型列表 ----------
const models = ref<AdminAiModel[]>([]);
const loading = ref(false);
const enabledCount = computed(() => models.value.filter((m) => m.enabled).length);

async function loadModels() {
  loading.value = true;
  try {
    const res = await aiApi.listModels();
    if (res.data.code === 0) models.value = res.data.data.list || [];
  } catch { /* ignore */ }
  loading.value = false;
}

async function toggleEnabled(row: AdminAiModel) {
  try {
    const res = await aiApi.updateModel(row.id, { enabled: row.enabled } as any);
    if (res.data.code !== 0) row.enabled = !row.enabled;
  } catch {
    row.enabled = !row.enabled;
    ElMessage.error('操作失败');
  }
}

// ---------- 创建 / 编辑 ----------
const dialogVisible = ref(false);
const saving = ref(false);
const editing = ref<AdminAiModel | null>(null);

function blankForm() {
  return {
    name: '', kind: 'chat', model_id: '', base_url: '', api_key: '',
    price_in: 0, price_out: 0, price_call: 0, max_tokens: 2048,
    temperature: 0.7, priority: 0, enabled: true, note: '',
    api_kind: 'openai', provider_hint: '', vendor: '', thinking: false, token_user_id: '', account_token: '',
    balance_provider: '', balance_url: '', balance_field: '',
    balance_divisor: 0, balance_multiplier: 0, balance_currency: '',
    rate_real_amount: 0, rate_virtual_amount: 0, rate_real_currency: 'CNY',
  };
}
const form = reactive(blankForm());

function kindLabel(kind: string) {
  const map: Record<string, string> = { chat: '对话', image: '绘画', video: '视频' };
  return map[kind] || kind;
}
function kindTag(kind: string) {
  const map: Record<string, string> = { chat: 'primary', image: 'success', video: 'warning' };
  return (map[kind] as any) || 'info';
}
function kindHint(kind: string) {
  const map: Record<string, string> = {
    chat: '按 tokens 计费：流式对话 / 编辑区 AI 助手（/chat/completions）',
    image: '按次计费：同步出图（/images/generations），成功才扣款',
    video: '按次计费：异步任务（/videos 提交 + 查询），提交即扣款、失败退款',
  };
  return map[kind] || '';
}

// ---------- 拉取结果：按需显示的列 ----------
// 模型名会被拆成「模型大名 / 版本 / 类型 / 子号」四段（后端 aiapi/vendor.ParseModelID），
// 站长可按需勾选显示哪几列，避免默认表格过宽。
const PULL_COLS = [
  { key: 'family', label: '模型大名（如 deepseek）' },
  { key: 'version', label: '版本（如 v4.1）' },
  { key: 'variant', label: '类型（如 flash / pro）' },
  { key: 'build', label: '子号（如 1017）' },
  { key: 'vendor', label: '厂商' },
  { key: 'channel', label: '渠道' },
] as const;
const pullCols = reactive<Record<string, boolean>>({
  family: true, version: true, variant: true, build: true, vendor: false, channel: true,
});

// ---------- 渠道卡密池 ----------
const ckDialog = ref(false);
const ckBase = ref('');
const ckSaving = ref(false);
const ckList = ref<any[]>([]);
const ckStats = ref<Record<string, { total: number; enabled: number; disabled: number }>>({});
const ckForm = reactive({ keys: '', label: '', weight: 1 });

async function ckOpen(baseURL: string) {
  ckBase.value = baseURL;
  ckDialog.value = true;
  ckForm.keys = '';
  ckForm.label = '';
  ckForm.weight = 1;
  await ckReload();
}

async function ckReload() {
  if (!ckBase.value) return;
  try {
    const res = await aiApi.channelKeys(ckBase.value);
    if (res.data.code === 0) {
      ckList.value = res.data.data.list || [];
      const enabled = ckList.value.filter((k) => k.enabled).length;
      ckStats.value = { ...ckStats.value, [ckBase.value]: { total: ckList.value.length, enabled, disabled: ckList.value.length - enabled } };
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '读取卡密失败');
  }
}

async function ckAdd() {
  if (!ckForm.keys.trim()) {
    ElMessage.warning('请粘贴至少一把卡密');
    return;
  }
  ckSaving.value = true;
  try {
    const res = await aiApi.addChannelKeys({
      base_url: ckBase.value, keys: ckForm.keys, label: ckForm.label, weight: ckForm.weight,
    });
    if (res.data.code === 0) {
      ElMessage.success(res.data.data.message);
      ckForm.keys = '';
      await ckReload();
      loadModels();
    } else {
      ElMessage.error(res.data.message || '添加失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '添加失败');
  } finally {
    ckSaving.value = false;
  }
}

async function ckToggle(row: any) {
  try {
    await aiApi.updateChannelKey(row.id, { base_url: ckBase.value, enabled: !row.enabled });
    await ckReload();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function ckRemove(row: any) {
  try {
    await ElMessageBox.confirm(`删除卡密「${row.label || row.hint}」？此操作不可恢复。`, '确认删除', { type: 'warning' });
  } catch { return; }
  try {
    await aiApi.deleteChannelKey(row.id);
    ElMessage.success('已删除');
    await ckReload();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

async function ckTest(row: any) {
  row._testing = true;
  try {
    const res = await aiApi.testChannelKey(row.id, ckBase.value);
    const d = res.data?.data;
    if (d?.ok) ElMessage.success(`连通正常，可用模型 ${d.model_count ?? '?'} 个`);
    else ElMessage.error(d?.error || '连通失败');
    await ckReload();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '测试失败');
  } finally {
    row._testing = false;
  }
}

// ---------- 渠道监控（临时令牌，不落库） ----------
const monDialog = ref(false);
const monLoading = ref(false);
const mon = reactive({ base_url: '', token: '', user_id: '', model_key: '', kind: '', locked: false });
const monResult = ref<any>(null);

/** 从某个渠道进来：地址预填且锁住（站长只该填令牌和用户 ID，别再让他在地址上出错） */
function openMonitor(baseUrl?: string) {
  monResult.value = null;
  mon.base_url = (baseUrl || '').trim();
  mon.locked = !!(baseUrl && baseUrl.trim());
  mon.token = '';
  mon.user_id = '';
  mon.model_key = '';
  mon.kind = '';
  monDialog.value = true;
}

const monTitle = computed(() =>
  mon.locked ? `渠道监控 · ${mon.base_url.replace(/^https?:\/\//, '')}` : '渠道监控（临时查询，不落库）',
);

async function runMonitor() {
  if (!mon.base_url.trim() || !mon.token.trim()) {
    ElMessage.warning('请填写渠道地址与令牌');
    return;
  }
  monLoading.value = true;
  monResult.value = null;
  try {
    const res = await aiApi.channelMonitor({
      base_url: mon.base_url.trim(),
      token: mon.token.trim(),
      user_id: mon.user_id.trim(),
      model_key: mon.model_key.trim(),
      kind: mon.kind,
    });
    if (res.data.code === 0) monResult.value = res.data.data;
    else ElMessage.error(res.data.message || '查询失败');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '查询失败');
  } finally {
    monLoading.value = false;
  }
}

// ---------- 渠道余额一览 ----------
const chDialog = ref(false);
const chLoading = ref(false);
const chRows = ref<any[]>([]);

async function openChannels() {
  chDialog.value = true;
  chLoading.value = true;
  try {
    const res = await aiApi.channelsBalance();
    if (res.data.code === 0) chRows.value = res.data.data.channels || [];
    else ElMessage.error(res.data.message || '查询失败');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '查询失败');
  } finally {
    chLoading.value = false;
  }
}

// ---------- 真实成本换算比例 ----------
// 中转站的「美元」只是站内代币，必须由站长声明「多少真实货币 = 多少虚拟单位」，
// 平台才能把「还剩 5274」折算成真金白银。
const rateDialog = ref(false);
const rateSaving = ref(false);
const rateValid = ref(false);
const rateRatio = ref(0);
const rateDescribe = ref('');
const realCurrencies = ref<string[]>(['CNY', 'USD', 'EUR', 'JPY', 'HKD']);
const rateForm = reactive<CostRateSetting>({
  real_amount: 0, virtual_amount: 0, real_currency: 'CNY', virtual_currency: '虚拟美元',
});
const ratePreviewIn = ref(1000);

// 试算完全按本地同一套算法（虚拟 × 真实数额 ÷ 虚拟数额），不额外打接口
const ratePreviewOut = computed(() => {
  const { real_amount: a, virtual_amount: b } = rateForm;
  if (!(a > 0) || !(b > 0)) return null;
  return Math.round((ratePreviewIn.value * a / b) * 10000) / 10000;
});

async function loadRate() {
  try {
    const res = await aiApi.getCostRate();
    if (res.data.code !== 0) return;
    const d = res.data.data;
    rateValid.value = d.valid;
    rateRatio.value = d.ratio;
    rateDescribe.value = d.describe;
    if (d.rate) Object.assign(rateForm, d.rate);
    if (Array.isArray(d.real_currencies) && d.real_currencies.length) realCurrencies.value = d.real_currencies;
  } catch {
    /* 读不到就不显示比例，不打断运营操作 */
  }
}

function openRateEdit() {
  rateDialog.value = true;
}

async function saveRate() {
  rateSaving.value = true;
  try {
    const res = await aiApi.saveCostRate({ ...rateForm });
    if (res.data.code !== 0) {
      ElMessage.error(res.data.message || '保存失败');
      return;
    }
    rateValid.value = res.data.data.valid;
    rateRatio.value = res.data.data.ratio;
    rateDescribe.value = res.data.data.describe;
    ElMessage.success('已保存，重新查询余额即可看到真实成本');
    rateDialog.value = false;
    // 换算在前端展示，重新拉一次让「实」字金额立刻出现
    if (chDialog.value) await openChannels();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    rateSaving.value = false;
  }
}

// ---------- L5 渠道对账 ----------
const channelDialog = ref(false);
const channelLoading = ref(false);
const channelReport = ref<AdminChannelReport | null>(null);
const channelTarget = ref<AdminAiModel | null>(null);

async function openChannel(row: AdminAiModel) {
  channelTarget.value = row;
  channelDialog.value = true;
  channelLoading.value = true;
  channelReport.value = null;
  try {
    const res = await aiApi.modelChannel(row.id);
    if (res.data.code === 0) channelReport.value = res.data.data;
    else ElMessage.error(res.data.message || '对账查询失败');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '对账查询失败');
  }
  channelLoading.value = false;
}

// 重新查询（弹窗内按钮，复用已选中的模型，不必再传参）
function refreshChannel() {
  if (channelTarget.value) openChannel(channelTarget.value);
}

// token 展示：千进位，避免一长串数字
function fmtTok(n?: number) {
  if (!n) return '0';
  if (n >= 1000000) return (n / 1000000).toFixed(2) + 'M';
  if (n >= 1000) return (n / 1000).toFixed(1) + 'k';
  return String(n);
}

// 金额展示：整数不带小数位，避免 10.0000 这种噪声
function fmtMoney(n?: number) {  if (n === undefined || n === null) return '—';
  return Number.isInteger(n) ? String(n) : n.toFixed(4).replace(/0+$/, '').replace(/\.$/, '');
}

// ---------- API 卡池 ----------
const keysVisible = ref(false);
const keyLoading = ref(false);
const keySaving = ref(false);
const testingId = ref(0);
const maxFail = ref(3);
const keysModel = ref<AdminAiModel | null>(null);
const keys = ref<any[]>([]);
const keyForm = reactive({ label: '', keys: '', weight: 1 });

async function loadKeys() {
  if (!keysModel.value) return;
  keyLoading.value = true;
  try {
    const res = await aiApi.aiModelKeys(keysModel.value.id);
    if (res.data.code === 0) {
      keys.value = res.data.data.list || [];
      maxFail.value = res.data.data.max_fail || 3;
    }
  } catch { /* ignore */ }
  keyLoading.value = false;
}

function openKeys(row: AdminAiModel) {
  keysModel.value = row;
  keys.value = [];
  keyForm.label = '';
  keyForm.keys = '';
  keyForm.weight = 1;
  keysVisible.value = true;
  loadKeys();
}

async function addKeys() {
  if (!keysModel.value || !keyForm.keys.trim()) {
    ElMessage.warning('请粘贴密钥');
    return;
  }
  keySaving.value = true;
  try {
    const res = await aiApi.aiModelKeyCreate(keysModel.value.id, {
      label: keyForm.label.trim(),
      keys: keyForm.keys,
      weight: keyForm.weight,
    });
    if (res.data.code === 0) {
      ElMessage.success((res.data as any).data?.message || '已添加');
      keyForm.keys = '';
      await loadKeys();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '添加失败');
  }
  keySaving.value = false;
}

async function updateKey(row: any, patch: any) {
  if (!keysModel.value) return;
  try {
    const res = await aiApi.aiModelKeyUpdate(keysModel.value.id, row.id, patch);
    if (res.data.code === 0) await loadKeys();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function removeKey(row: any) {
  if (!keysModel.value) return;
  try {
    await ElMessageBox.confirm(`删除该密钥（${row.hint || row.id}）？`, '删除确认', { type: 'warning' });
  } catch { return; }
  try {
    const res = await aiApi.aiModelKeyDelete(keysModel.value.id, row.id);
    if (res.data.code === 0) {
      ElMessage.success('已删除');
      await loadKeys();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

async function testKey(row: any) {
  if (!keysModel.value) return;
  testingId.value = row.id;
  try {
    const res = await aiApi.aiModelKeyTest(keysModel.value.id, { key_id: row.id });
    if (res.data.code === 0) {
      const r = res.data.data;
      ElMessage[r.ok ? 'success' : 'error'](r.message);
      if (r.ok) await loadKeys();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '测试失败');
  }
  testingId.value = 0;
}

// ---------- 厂商预设 ----------
const presets = ref<AiModelPreset[]>([]);
const presetKey = ref('');
const presetNote = ref('');

async function loadPresets() {
  try {
    const res = await aiApi.aiModelPresets();
    if (res.data.code === 0) presets.value = res.data.data.list || [];
  } catch { /* ignore */ }
}

function applyPreset(key: string) {
  const p = presets.value.find((x) => x.key === key);
  if (!p) { presetNote.value = ''; return; }
  form.kind = p.kind;
  form.name = p.name;
  form.model_id = p.model_id;
  form.base_url = p.base_url || form.base_url;
  form.api_kind = p.api_kind;
  form.provider_hint = p.provider_hint;
  form.vendor = p.vendor || '';
  if (p.price_call) form.price_call = p.price_call;
  form.note = p.note;
  presetNote.value = `${p.note}${p.base_url ? '' : '（该厂商无统一公共端点，请填你自己中转的地址）'}`;
}

// ---------- 空态「一键创建」----------
// 只展示 8 个，并且把「开箱可用」（base_url 非空）的排前面：
// MJ / Kling / Sora 这类没有统一公共端点，必须填中转地址，直接创建会得到一个连不上的模型。
const quickPresets = computed(() => {
  const kindOrder = ['chat', 'image', 'video'];
  return [...presets.value]
    .sort((a, b) => {
      const ua = a.base_url ? 0 : 1;
      const ub = b.base_url ? 0 : 1;
      if (ua !== ub) return ua - ub;
      return kindOrder.indexOf(a.kind) - kindOrder.indexOf(b.kind);
    })
    .slice(0, 8);
});
const quickAdding = ref('');

async function quickAdd(p: AiModelPreset) {
  // base_url 空 → 不能直接落地，改为打开弹窗并预填，让站长补自己的中转地址
  if (!p.base_url) {
    openCreate();
    presetKey.value = p.key;
    applyPreset(p.key);
    ElMessage.info('该厂商没有统一公共端点，请填写你自己的中转地址');
    return;
  }
  quickAdding.value = p.key;
  try {
    const res = await aiApi.createModel({
      name: p.name,
      kind: p.kind,
      model_id: p.model_id,
      base_url: p.base_url,
      price_in: 0,
      price_out: 0,
      price_call: p.price_call || 0,
      max_tokens: 2048,
      temperature: 0.7,
      priority: 0,
      enabled: true,
      api_kind: p.api_kind,
      provider_hint: p.provider_hint || '',
      vendor: p.vendor || '',
      note: p.note || '',
    });
    if (res.data.code === 0) {
      // 明确说清"还没完"：少了密钥，用户侧仍然调不通，是最容易卡住的一步
      ElMessage.success(`已添加「${p.name}」，下一步：点该行「密钥池」填入上游密钥`);
      await loadModels();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '添加失败');
  }
  quickAdding.value = '';
}

function apiKindHint(k: string) {
  const map: Record<string, string> = {
    openai: '标准 /v1/images/generations（豆包 Seedream、gpt-image-2、多数中转）',
    mj: 'Midjourney 代理协议：/mj/submit/imagine + 任务轮询；模型标识含 niji 即走 Niji 机器人',
    gemini: 'Google generateContent（responseModalities=IMAGE），如 gemini-2.5-flash-image（Nano Banana）',
    claude: 'Anthropic Messages API：POST /v1/messages，x-api-key + anthropic-version；system 走顶层字段，响应为 content block 数组',
  };
  return map[k] || '';
}

function openCreate() {
  editing.value = null;
  Object.assign(form, blankForm());
  dialogVisible.value = true;
}

function openEdit(row: AdminAiModel) {
  editing.value = row;
  Object.assign(form, {
    name: row.name, kind: row.kind || 'chat', model_id: row.model_id, base_url: row.base_url, api_key: '',
    price_in: row.price_in, price_out: row.price_out, price_call: row.price_call || 0,
    max_tokens: row.max_tokens,
    temperature: row.temperature, priority: row.priority, enabled: row.enabled, note: row.note,
    api_kind: row.api_kind || 'openai', provider_hint: row.provider_hint || '', token_user_id: row.token_user_id || '',
    balance_provider: row.balance_provider || '', balance_url: row.balance_url || '',
    balance_field: row.balance_field || '', balance_divisor: row.balance_divisor || 0,
    balance_multiplier: row.balance_multiplier || 0, balance_currency: row.balance_currency || '',
    rate_real_amount: row.rate_real_amount || 0, rate_virtual_amount: row.rate_virtual_amount || 0,
    rate_real_currency: row.rate_real_currency || 'CNY',
    vendor: row.vendor || '', thinking: !!row.thinking,
  });
  dialogVisible.value = true;
}

async function saveModel() {
  if (!form.name.trim() || !form.model_id.trim() || !form.base_url.trim()) {
    ElMessage.warning('展示名、上游模型 ID、上游地址均为必填');
    return;
  }
  saving.value = true;
  try {
    const payload: any = {
      name: form.name.trim(),
      kind: form.kind,
      model_id: form.model_id.trim(),
      base_url: form.base_url.trim(),
      price_in: form.price_in,
      price_out: form.price_out,
      price_call: form.price_call,
      max_tokens: form.max_tokens,
      temperature: form.temperature,
      priority: form.priority,
      enabled: form.enabled,
      api_kind: form.api_kind,
      provider_hint: form.provider_hint,
      vendor: form.vendor,
      thinking: !!form.thinking,
      note: form.note.trim(),
      // 中转站用户 ID（非机密，允许清空）
      token_user_id: form.token_user_id.trim(),
      // 余额查询方式：官方 / 中转站 / 自定义规则 / 不查
      balance_provider: form.balance_provider,
      balance_url: form.balance_url.trim(),
      balance_field: form.balance_field.trim(),
      balance_divisor: form.balance_divisor || 0,
      balance_multiplier: form.balance_multiplier || 0,
      balance_currency: form.balance_currency.trim(),
      // 渠道级真实成本比例（两数都 0 = 跟随站点全局）
      rate_real_amount: form.rate_real_amount || 0,
      rate_virtual_amount: form.rate_virtual_amount || 0,
      rate_real_currency: form.rate_real_currency,
    };
    if (form.api_key.trim()) payload.api_key = form.api_key.trim();
    if (form.account_token.trim()) payload.account_token = form.account_token.trim();
    const res = editing.value
      ? await aiApi.updateModel(editing.value.id, payload)
      : await aiApi.createModel(payload);
    if (res.data.code === 0) {
      ElMessage.success('已保存');
      dialogVisible.value = false;
      await loadModels();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  saving.value = false;
}

async function removeModel(row: AdminAiModel) {
  try {
    await ElMessageBox.confirm(`确定删除模型「${row.name}」？`, '删除确认', { type: 'warning' });
  } catch { return; }
  try {
    const res = await aiApi.deleteModel(row.id);
    if (res.data.code === 0) {
      ElMessage.success('已删除');
      await loadModels();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

// ---------- 从渠道拉取模型（L5） ----------
const pullDialogVisible = ref(false);
const pullForm = reactive({ base_url: '', api_key: '' });
const pullList = ref<ProviderModelSuggestion[]>([]);
const pullSelected = ref<ProviderModelSuggestion[]>([]);
const pulling = ref(false);
const importing = ref(false);
const pullError = ref('');
const pullChannel = ref('');
const pullTableRef = ref<any>(null);

function openPull() {
  pullForm.base_url = '';
  pullForm.api_key = '';
  pullList.value = [];
  pullSelected.value = [];
  pullError.value = '';
  pullChannel.value = '';
  pullDialogVisible.value = true;
}

async function doPull() {
  if (!pullForm.base_url.trim()) {
    ElMessage.warning('请填写上游地址');
    return;
  }
  pulling.value = true;
  pullError.value = '';
  pullList.value = [];
  pullSelected.value = [];
  try {
    const res = await aiApi.pullModels({ base_url: pullForm.base_url.trim(), api_key: pullForm.api_key.trim() });
    if (res.data.code === 0) {
      pullList.value = (res.data.data.items || []).map((it) => ({ ...it, user_visible: false }));
      if (!pullList.value.length) {
        pullError.value = '该渠道未返回任何模型（可能是地址错误、密钥无效，或上游不支持 /models 端点）';
      }
    }
  } catch (e: any) {
    pullError.value = e?.response?.data?.message || '拉取失败';
  }
  pulling.value = false;
}

function onPullSelection(rows: any[]) {
  pullSelected.value = rows as ProviderModelSuggestion[];
}

function toggleAllPull(v: boolean) {
  // Element Plus 表格多选由内部维护，这里通过 ref 触发：
  const tbl = pullTableRef.value;
  if (!tbl) return;
  if (v) pullList.value.forEach((r) => tbl.toggleRowSelection(r, true));
  else tbl.clearSelection();
}

async function doImport() {
  if (!pullSelected.value.length) {
    ElMessage.warning('请先勾选要导入的模型');
    return;
  }
  importing.value = true;
  try {
    const res = await aiApi.importModels({
      base_url: pullForm.base_url.trim(),
      api_key: pullForm.api_key.trim(),
      channel: pullChannel.value.trim(),
      items: pullSelected.value.map((it) => ({
        raw_id: it.raw_id,
        alias: it.alias,
        vendor: it.vendor,
        category: it.category,
        user_visible: it.user_visible,
      })),
    });
    if (res.data.code === 0) {
      ElMessage.success(`已导入 ${res.data.data.created} 个模型（默认对用户隐藏，到密钥池补密钥并开放可见性）`);
      pullDialogVisible.value = false;
      await loadModels();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '导入失败');
  }
  importing.value = false;
}

const route = reactive({
  strategy: 'manual',
  scene_strategy: {} as Record<string, string>,
  prefer_domestic: false,
  fallback: true,
  max_fallback: 1,
});
const routeSaving = ref(false);
const routeHint = ref('');

async function loadRoute() {
  try {
    const res = await adminApi.getAiRoute();
    const cfg = res.data.data?.config;
    if (cfg) {
      route.strategy = cfg.strategy;
      route.scene_strategy = cfg.scene_strategy || {};
      route.prefer_domestic = !!cfg.prefer_domestic;
      route.fallback = !!cfg.fallback;
      route.max_fallback = cfg.max_fallback || 1;
    }
    const scenes = res.data.data?.preview?.scenes || {};
    routeHint.value = Object.entries(scenes).map(([k, v]) => `${k}=${v}`).join(' · ');
  } catch { /* 策略不可用不阻断模型管理 */ }
}

async function saveRoute() {
  routeSaving.value = true;
  try {
    await adminApi.updateAiRoute({ ...route });
    ElMessage.success('路由策略已保存并即时生效');
    await loadRoute();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    routeSaving.value = false;
  }
}

onMounted(() => {
  loadConfig();
  loadModels();
  loadPresets();
  loadRoute();
  loadVendors();
  loadMatrix();
  loadCategoryPrices();
  loadRate();
  // 直接落在 ?tab=stats 时先把统计拉回来，否则会看到空表
  if (level1.value === 'stats') loadUsageStats();
});


// ---------- 厂商管理 ----------
const vendors = ref<AiVendor[]>([]);
const vendorLoading = ref(false);
const vendorDialog = ref(false);
const vendorSaving = ref(false);
const vendorEditing = ref<AiVendor | null>(null);
const vendorForm = reactive({ name: '', prefix: '', sort_order: 0, enabled: true });
const enabledVendorCount = computed(() => vendors.value.filter((v) => v.enabled).length);

async function loadVendors() {
  vendorLoading.value = true;
  try {
    const res = await aiApi.listAiVendors();
    if (res.data.code === 0) vendors.value = res.data.data.list || [];
  } catch { /* ignore */ }
  vendorLoading.value = false;
}

async function toggleVendor(row: AiVendor) {
  try {
    const res = await aiApi.updateAiVendor(row.id, { name: row.name, sort_order: row.sort_order, enabled: row.enabled, prefix: row.prefix });
    if (res.data.code !== 0) row.enabled = !row.enabled;
  } catch {
    row.enabled = !row.enabled;
    ElMessage.error('操作失败');
  }
}

function openVendorCreate() {
  vendorEditing.value = null;
  Object.assign(vendorForm, { name: '', prefix: '', sort_order: vendors.value.length + 1, enabled: true });
  vendorDialog.value = true;
}

function openVendorEdit(row: AiVendor) {
  vendorEditing.value = row;
  Object.assign(vendorForm, { name: row.name, prefix: row.prefix, sort_order: row.sort_order, enabled: row.enabled });
  vendorDialog.value = true;
}

async function saveVendor() {
  if (!vendorForm.name.trim() || !vendorForm.prefix.trim()) {
    ElMessage.warning('厂商名与 ID 前缀均为必填');
    return;
  }
  vendorSaving.value = true;
  try {
    const payload = {
      name: vendorForm.name.trim(),
      prefix: vendorForm.prefix.trim(),
      sort_order: vendorForm.sort_order,
      enabled: vendorForm.enabled,
    };
    const res = vendorEditing.value
      ? await aiApi.updateAiVendor(vendorEditing.value.id, payload)
      : await aiApi.createAiVendor(payload);
    if (res.data.code === 0) {
      ElMessage.success('已保存');
      vendorDialog.value = false;
      await loadVendors();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  vendorSaving.value = false;
}

async function removeVendor(row: AiVendor) {
  if (row.models > 0) {
    ElMessage.warning(`仍有 ${row.models} 个模型属于「${row.name}」，请先改走其他厂商`);
    return;
  }
  try {
    await ElMessageBox.confirm(`删除厂商「${row.name}」？`, '删除确认', { type: 'warning' });
  } catch { return; }
  try {
    const res = await aiApi.deleteAiVendor(row.id);
    if (res.data.code === 0) {
      ElMessage.success('已删除');
      await loadVendors();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

// ---------- 厂商版本矩阵（管理端专用，含 newest 标记） ----------
const matrix = ref<AiVendorMatrixEntry[]>([]);
const matrixLoading = ref(false);

async function loadMatrix() {
  matrixLoading.value = true;
  try {
    const res = await aiApi.getVendorMatrix();
    if (res.data.code === 0) matrix.value = res.data.data.vendors || [];
  } catch { /* ignore */ }
  matrixLoading.value = false;
}

// ---------- 聚合分类定价（chat/image/video 默认单价） ----------
const categoryPrices = ref<(AiCategoryPrice & { _saving?: boolean })[]>([]);

async function loadCategoryPrices() {
  try {
    const res = await aiApi.getCategoryPrices();
    if (res.data.code === 0) {
      categoryPrices.value = (res.data.data.list || []).map((p: AiCategoryPrice) => ({ ...p, _saving: false }));
    }
  } catch { /* ignore */ }
}

async function saveCategoryPrice(p: AiCategoryPrice & { _saving?: boolean }) {
  p._saving = true;
  try {
    const res = await aiApi.setCategoryPrice({
      kind: p.kind,
      price_in: p.price_in || 0,
      price_out: p.price_out || 0,
      price_call: p.price_call || 0,
    });
    if (res.data.code === 0) ElMessage.success('已保存');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  p._saving = false;
}
</script>

<style scoped>
.admin-page { --admin-content-max: 1100px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }

/* 分块切换条：与 AdminHeader / body 共用同一条居中列，否则会通栏贴左错位 */
.admin-page > .pool-nav {
  max-width: var(--admin-content-max, 1100px);
  margin-left: auto;
  margin-right: auto;
}
.pool-tabs {
  display: flex; gap: 6px; flex-wrap: wrap;
  margin-bottom: 14px; padding: 5px;
  border-radius: 12px;
  background: var(--bg-card, #fff);
  border: 1px solid var(--border-color, #e8eaf0);
}
.pool-tab {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 7px 14px; border-radius: 9px;
  border: none; background: transparent; cursor: pointer;
  font-size: 0.86rem; color: var(--text-light, #64748b);
  transition: background 0.15s, color 0.15s;
}
.pool-tab:hover { background: var(--bg-subtle, #f1f5f9); }
.pool-tab.active {
  background: var(--gradient-primary, linear-gradient(135deg, #6366f1, #8b5cf6));
  color: #fff; font-weight: 600;
}
.pool-tab-ico { font-size: 0.95rem; line-height: 1; }
.pool-tab-count {
  font-size: 0.72rem; padding: 0 6px; border-radius: 999px;
  background: rgba(15, 23, 42, 0.08); color: inherit;
  font-variant-numeric: tabular-nums;
}
.pool-tab.active .pool-tab-count { background: rgba(255, 255, 255, 0.24); }

/* 两级导航：一级是主色胶囊，二级是浅底小胶囊，视觉上区分层级 */
.pool-nav { display: flex; flex-direction: column; gap: 8px; }
.pool-tabs-l1 { background: transparent; border: none; padding: 0; }
.pool-tabs-l1 .pool-tab {
  padding: 8px 18px; border-radius: 10px;
  background: var(--bg-card, #fff);
  border: 1px solid var(--border-color, #e8eaf0);
  font-weight: 600; color: var(--text-color, #334155);
}
.pool-tabs-l1 .pool-tab.active {
  background: var(--gradient-primary, linear-gradient(135deg, #6366f1, #8b5cf6));
  border-color: transparent; color: #fff;
}
.pool-tabs-l2 {
  background: var(--bg-subtle, #f8fafc);
  border: 1px solid var(--border-color, #e8eaf0);
  padding: 4px;
  width: fit-content;
}
.pool-tab-sm {
  padding: 5px 12px; font-size: 0.82rem; border-radius: 8px;
  color: var(--text-light, #64748b);
}
.pool-tab-sm.active {
  background: var(--bg-card, #fff); color: var(--el-color-primary, #6366f1);
  font-weight: 600; box-shadow: var(--shadow-xs, 0 1px 3px rgba(0, 0, 0, 0.08));
}

/* 自由添加：三张步骤卡 */
.free-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 14px; margin-top: 12px; }
.free-card {
  padding: 16px; border-radius: 12px;
  border: 1px solid var(--border-color, #e8eaf0);
  background: var(--bg-subtle, #f8fafc);
  display: flex; flex-direction: column; gap: 8px;
}
.free-step {
  width: 24px; height: 24px; border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  font-size: 0.76rem; font-weight: 700; color: #fff;
  background: var(--gradient-primary, linear-gradient(135deg, #6366f1, #8b5cf6));
}
.free-title { margin: 0; font-size: 0.92rem; font-weight: 700; color: var(--text-main, #1e293b); }
.free-desc { margin: 0; font-size: 0.8rem; line-height: 1.7; color: var(--text-light, #64748b); flex: 1; }
.free-desc code { padding: 1px 4px; border-radius: 4px; background: rgba(15, 23, 42, 0.06); }

/* 统计顶部汇总 */
.stats-sum {
  display: flex; flex-wrap: wrap; gap: 18px;
  margin: 4px 0 12px; font-size: 0.82rem; color: var(--text-light, #64748b);
}
.stats-sum b { color: var(--text-main, #1e293b); font-variant-numeric: tabular-nums; font-size: 0.95rem; }
.stat-bad { color: var(--el-color-danger, #f56c6c); font-weight: 600; }

.hint-text { color: var(--text-light, #666); font-size: 0.85rem; margin-bottom: 12px; line-height: 1.6; }
.enable-row { display: flex; align-items: center; justify-content: space-between; padding: 8px 4px; font-weight: 600; }
.table-toolbar { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
/* 按钮内的 emoji 前缀：不能走 :icon（Element Plus 会把字符串当组件名 createElement） */
.btn-emoji { margin-right: 4px; }
.kp-tip {
  font-size: 0.78rem; color: var(--text-light, #777); line-height: 1.6;
  background: var(--el-fill-color-light, #f8f9fa); border-radius: 8px; padding: 8px 12px; margin-bottom: 12px;
}
.kp-add { display: flex; align-items: flex-start; gap: 8px; margin-bottom: 12px; }
.muted { color: var(--text-light, #999); }
.small { font-size: 0.78rem; margin-left: 6px; }
.mono { font-family: Consolas, 'Courier New', monospace; }
.kind-hint { font-size: 0.75rem; color: var(--text-light, #999); margin-top: 2px; }
.pull-step { padding: 2px 4px; }
.pull-error { color: #f56c6c; font-size: 0.82rem; margin: 8px 0; }
.pull-result { margin-top: 12px; border-top: 1px solid var(--border-color, #ebeef5); padding-top: 12px; }
.pull-result-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
.pull-import-bar { display: flex; align-items: center; justify-content: flex-end; gap: 10px; margin-top: 12px; }
[data-theme="dark"] .admin-page { background: #0f172a; }

/* 厂商版本矩阵 */
.matrix-wrap { display: flex; flex-direction: column; gap: 12px; }
.matrix-vendor { border: 1px solid var(--border-color, #ebeef5); border-radius: 10px; padding: 10px 14px; background: var(--bg-card, #fff); }
.matrix-vendor-head { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; }
.matrix-prefix { background: var(--el-fill-color-light, #f4f4f5); border-radius: 4px; padding: 1px 6px; font-size: 0.78rem; color: var(--text-light, #888); }
.matrix-version { padding: 6px 0; border-top: 1px dashed var(--border-color, #eee); }
.matrix-version:first-of-type { border-top: none; }
.matrix-version-head { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
.matrix-version-tag { font-family: Consolas, 'Courier New', monospace; font-weight: 600; color: var(--text-main, #333); }
.matrix-subs { display: flex; flex-wrap: wrap; gap: 6px; }
.matrix-sub { font-size: 0.78rem; padding: 2px 8px; border-radius: 12px; background: var(--el-fill-color-light, #f4f4f5); color: var(--text-light, #888); }
.matrix-sub-newest { background: var(--gradient-success, #67c23a); color: #fff; font-weight: 600; }
.matrix-sub-newest em { font-style: normal; margin-left: 2px; }

/* 聚合分类定价 */
.price-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 14px; }
.price-card { border: 1px solid var(--border-color, #ebeef5); border-radius: 10px; padding: 14px; background: var(--bg-card, #fff); }
.price-card-head { display: flex; align-items: center; gap: 8px; margin-bottom: 10px; }
.price-row { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.price-label { width: 56px; font-size: 0.82rem; color: var(--text-light, #666); }

/* 拉取结果的列显示开关 */
.pull-result-ops { display: inline-flex; align-items: center; gap: 6px; }
.pull-col-toggles { display: flex; flex-direction: column; gap: 4px; }
.pull-col-title { font-size: 0.8rem; font-weight: 600; margin-bottom: 4px; color: var(--text-main, #1f2937); }
.pull-col-tip { margin-top: 6px; font-size: 0.72rem; line-height: 1.5; color: var(--text-light, #999); }

/* 渠道监控：自动纠正提示（不显眼但必须能看见） */
.mon-warn-list {
  display: flex; flex-direction: column; gap: 4px;
  margin-bottom: 8px; padding: 8px 10px; border-radius: 6px;
  border: 1px solid var(--el-color-warning-light-5, #f5dab1);
  background: var(--el-color-warning-light-9, #fdf6ec);
}
.mon-warn-item { font-size: 0.74rem; color: var(--el-color-warning, #b88230); line-height: 1.6; }

/* 渠道卡密池 */
.key-add { margin-bottom: 12px; display: flex; flex-direction: column; gap: 8px; }
.key-add-row { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.key-fail { margin-left: 6px; font-size: 0.72rem; color: var(--el-color-warning, #e6a23c); }
.mono { font-family: var(--font-mono, ui-monospace, Menlo, monospace); font-size: 0.78rem; }

/* 渠道监控 */
.mon-form { margin-top: 4px; }
.mon-result { border-top: 1px dashed var(--border-color, #e8eaf0); padding-top: 12px; display: flex; flex-direction: column; gap: 10px; }
.mon-row { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; font-size: 0.82rem; }
.mon-k { width: 72px; flex: none; color: var(--text-light, #94a3b8); }
.mon-bad { color: var(--el-color-danger, #f56c6c); line-height: 1.6; }
.mon-logs { display: flex; flex-direction: column; gap: 4px; flex: 1; }
.mon-log { display: flex; align-items: center; gap: 12px; font-size: 0.78rem; }
.mon-model { min-width: 150px; color: var(--text-main, #334155); }

/* 渠道余额一览 */
.unit { font-style: normal; font-size: 0.72rem; color: var(--text-light, #999); }
.ch-est { font-weight: 600; color: var(--el-color-warning, #e6a23c); font-variant-numeric: tabular-nums; }
.ch-tok { font-variant-numeric: tabular-nums; font-size: 0.78rem; color: var(--text-main, #475569); }
.ch-overview-tip { margin-bottom: 10px; font-size: 0.78rem; line-height: 1.6; color: var(--text-light, #888); }
.ch-bal { font-weight: 700; color: var(--el-color-primary, #409eff); font-variant-numeric: tabular-nums; }
.ch-bal.zero { color: var(--text-light, #bbb); }

/* 换算比例条：未设置时用暖色警示，避免站长把「虚拟美元」当真实成本看 */
.ch-rate-bar {
  display: flex; align-items: center; gap: 10px;
  padding: 10px 12px; margin-bottom: 10px; border-radius: 8px;
  background: var(--el-color-primary-light-9, #ecf5ff);
  border: 1px solid var(--el-color-primary-light-7, #b3d8ff);
}
.ch-rate-bar.unset {
  background: var(--el-color-warning-light-9, #fdf6ec);
  border-color: var(--el-color-warning-light-7, #f5dab1);
}
.ch-rate-icon { font-size: 1.1rem; line-height: 1; }
.ch-rate-text { flex: 1; font-size: 0.8rem; line-height: 1.6; color: var(--text-main, #475569); }
.ch-rate-text .muted { display: block; }

/* 虚拟值 + 真实值双行（不额外加列，避免表过宽） */
.ch-dual { display: flex; flex-direction: column; gap: 1px; font-variant-numeric: tabular-nums; }
.ch-real { font-size: 0.72rem; font-weight: 600; color: var(--el-color-danger, #f56c6c); }
.ch-noreal { font-size: 0.7rem; color: var(--text-light, #c0c4cc); }

.ch-rate-help {
  margin-bottom: 14px; padding: 10px 12px; border-radius: 8px;
  background: var(--bg-subtle, #f8fafc); font-size: 0.78rem; line-height: 1.7;
  color: var(--text-main, #475569);
}
.ch-rate-help code {
  padding: 1px 5px; border-radius: 4px; background: var(--bg-page, #eef2f7);
  font-size: 0.76rem;
}
.ch-rate-eq { margin: 0 8px; color: var(--text-light, #999); }
.ch-rate-form :deep(.el-input-number) { width: 100%; }

/* L5 渠道对账 */
.ch-loading { padding: 28px 0; text-align: center; color: var(--text-light, #888); font-size: 0.88rem; }
.ch-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; margin-bottom: 14px; }
.ch-name { font-weight: 600; font-size: 0.98rem; color: var(--text-main, #1f2937); }
.ch-meta { font-size: 0.76rem; color: var(--text-light, #888); word-break: break-all; margin-top: 2px; }
.ch-cards { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; }
.ch-card {
  border: 1px solid var(--border-color, #ebeef5); border-radius: 10px;
  padding: 12px 14px; background: var(--bg-soft, #fafbfc);
}
.ch-card-k { font-size: 0.75rem; color: var(--text-light, #888); margin-bottom: 6px; }
.ch-card-v { font-size: 1.4rem; font-weight: 700; color: var(--el-color-primary, #409eff); line-height: 1.15; font-variant-numeric: tabular-nums; }
.ch-card-v.used { color: var(--el-color-warning, #e6a23c); }
.ch-card-u { font-size: 0.72rem; color: var(--text-light, #aaa); margin-top: 2px; }
.ch-tip { margin-top: 10px; font-size: 0.76rem; color: var(--text-light, #999); }
</style>
