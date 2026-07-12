<template>
  <div class="bookshelf-page" v-loading="loading">
    <div class="page-container">
      <div class="shelf-header">
        <h1 class="section-title">
          📚 我的书架
          <span class="shelf-count" v-if="total > 0">（{{ total }} 部）</span>
        </h1>
      </div>

      <!-- 空状态 -->
      <el-empty
        v-if="!loading && items.length === 0"
        description="书架空空如也，去发现好作品吧"
        :image-size="160"
      >
        <el-button type="primary" @click="$router.push('/')">去首页看看</el-button>
      </el-empty>

      <!-- 书架列表 -->
      <div v-else class="shelf-grid">
        <div
          v-for="item in items"
          :key="item.id"
          class="shelf-card"
          @click="goRead(item)"
        >
          <!-- 封面 -->
          <div class="shelf-cover">
            <img
              v-if="item.novel.cover_url"
              :src="item.novel.cover_url"
              :alt="item.novel.title"
            />
            <div v-else class="cover-placeholder">
              {{ item.novel.title[0] }}
            </div>
          </div>

          <!-- 信息 -->
          <div class="shelf-info">
            <h3 class="shelf-title">{{ item.novel.title }}</h3>
            <p class="shelf-author">作者：{{ item.novel.author?.nickname || item.novel.author?.username || '未知' }}</p>
            <div class="shelf-meta">
              <span class="meta-tag">{{ item.novel.category }}</span>
              <span>{{ item.novel.total_chapters }} 章</span>
              <span>{{ formatWords(item.novel.total_words) }}</span>
            </div>
            <!-- 阅读进度 -->
            <div class="shelf-progress" v-if="item.last_read_chapter > 0">
              <el-progress
                :percentage="
                  item.novel.total_chapters > 0
                    ? Math.round((item.last_read_chapter / item.novel.total_chapters) * 100)
                    : 0
                "
                :stroke-width="6"
                :show-text="false"
              />
              <span class="progress-text">
                已读至第 {{ item.last_read_chapter }} 章
                <template v-if="item.novel.total_chapters > 0">
                  （{{ item.last_read_chapter }}/{{ item.novel.total_chapters }}）
                </template>
              </span>
            </div>
            <div class="shelf-progress" v-else>
              <span class="progress-text new-text">新添加到书架</span>
            </div>
            <!-- 状态 -->
            <div class="shelf-status">
              <el-tag :type="item.novel.status === 'completed' ? 'success' : ''" size="small">
                {{ item.novel.status === 'completed' ? '已完结' : '连载中' }}
              </el-tag>
            </div>
          </div>

          <!-- 移除按钮 -->
          <el-popconfirm
            title="确认从书架移除？"
            confirm-button-text="移除"
            cancel-button-text="取消"
            @confirm.stop="handleRemove(item.novel_id)"
          >
            <template #reference>
              <el-button
                class="shelf-remove-btn"
                size="small"
                circle
                :icon="Delete"
                @click.stop
              />
            </template>
          </el-popconfirm>
        </div>
      </div>

      <!-- 分页 -->
      <div class="shelf-pagination" v-if="total > pageSize">
        <el-pagination
          v-model:current-page="currentPage"
          :page-size="pageSize"
          :total="total"
          layout="prev, pager, next"
          @current-change="loadBookshelf"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { bookshelfApi, type ShelfItem } from '@/api/bookshelf';
import { ElMessage } from 'element-plus';
import { Delete } from '@element-plus/icons-vue';

const router = useRouter();
const items = ref<ShelfItem[]>([]);
const total = ref(0);
const loading = ref(false);
const currentPage = ref(1);
const pageSize = 50;

function formatWords(n: number): string {
  if (n >= 10000) return (n / 10000).toFixed(1) + '万字';
  if (n >= 1000) return (n / 1000).toFixed(1) + '千字';
  return n + '字';
}

function goRead(item: ShelfItem) {
  const ch = item.last_read_chapter > 0 ? item.last_read_chapter : 1;
  router.push(`/novel/${item.novel_id}/read/${ch}`);
}

async function loadBookshelf() {
  loading.value = true;
  try {
    const res = await bookshelfApi.getList(currentPage.value, pageSize);
    if (res.data.code === 0) {
      items.value = res.data.data.list || [];
      total.value = res.data.data.total || 0;
    }
  } catch {
    ElMessage.error('加载书架失败');
  } finally {
    loading.value = false;
  }
}

async function handleRemove(novelId: number) {
  try {
    await bookshelfApi.remove(novelId);
    items.value = items.value.filter(i => i.novel_id !== novelId);
    total.value--;
    ElMessage.success('已从书架移除');
  } catch {
    ElMessage.error('操作失败');
  }
}

onMounted(() => {
  loadBookshelf();
});
</script>

<style scoped>
.bookshelf-page {
  min-height: calc(100vh - 60px);
  background: var(--bg-color);
  padding-top: 20px;
}

.shelf-header {
  margin-bottom: 24px;
}

.shelf-count {
  font-size: 0.9rem;
  color: var(--text-light);
  font-weight: 400;
}

.shelf-grid {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.shelf-card {
  position: relative;
  display: flex;
  gap: 16px;
  padding: 16px;
  background: #fff;
  border-radius: var(--radius-sm);
  box-shadow: var(--shadow-card);
  cursor: pointer;
  transition: box-shadow var(--transition-fast), transform var(--transition-fast);
  align-items: center;
}

.shelf-card:hover {
  box-shadow: var(--shadow-card-hover);
  transform: translateY(-2px);
}

.shelf-cover {
  width: 72px;
  height: 96px;
  border-radius: 6px;
  overflow: hidden;
  flex-shrink: 0;
  background: var(--bg-soft);
}

.shelf-cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.cover-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1.8rem;
  font-weight: 700;
  color: var(--primary-color);
  background: linear-gradient(135deg, #e8e0ff, #dbeafe);
}

.shelf-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.shelf-title {
  font-size: 1.05rem;
  font-weight: 600;
  color: var(--text-color);
  margin: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.shelf-author {
  font-size: 0.82rem;
  color: var(--text-light);
  margin: 0;
}

.shelf-meta {
  display: flex;
  gap: 12px;
  font-size: 0.78rem;
  color: var(--text-muted);
  align-items: center;
}

.meta-tag {
  display: inline-block;
  background: #f0f0ff;
  color: var(--primary-color);
  padding: 1px 8px;
  border-radius: 10px;
  font-size: 0.72rem;
}

.shelf-progress {
  margin-top: 2px;
}

.progress-text {
  font-size: 0.78rem;
  color: var(--primary-color);
}

.new-text {
  color: var(--accent-color);
}

.shelf-status {
  position: absolute;
  top: 12px;
  right: 48px;
}

.shelf-remove-btn {
  position: absolute;
  top: 12px;
  right: 12px;
  opacity: 0;
  transition: opacity var(--transition-fast);
}

.shelf-card:hover .shelf-remove-btn {
  opacity: 1;
}

.shelf-pagination {
  display: flex;
  justify-content: center;
  margin-top: 24px;
}

/* Dark mode */
[data-theme="dark"] .shelf-card {
  background: #1e293b;
}

[data-theme="dark"] .cover-placeholder {
  background: linear-gradient(135deg, #2d2450, #1a3050);
}

[data-theme="dark"] .meta-tag {
  background: rgba(99, 102, 241, 0.15);
}

@media (max-width: 640px) {
  .shelf-cover {
    width: 56px;
    height: 76px;
  }
  .shelf-title {
    font-size: 0.95rem;
  }
  .shelf-meta {
    flex-wrap: wrap;
    gap: 6px;
  }
}
</style>
