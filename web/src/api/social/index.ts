import api from '@/api/index';

// 关注作者
export const followApi = {
  follow(authorId: number) {
    return api.post(`/follow/${authorId}`);
  },
  unfollow(authorId: number) {
    return api.delete(`/follow/${authorId}`);
  },
  check(authorId: number) {
    return api.get(`/follow/check/${authorId}`);
  },
  listFollowing(page: number = 1) {
    return api.get('/following', { params: { page } });
  },
  listFollowers(page: number = 1) {
    return api.get('/followers', { params: { page } });
  },
  stats() {
    return api.get('/follow/stats');
  },
};

// 作者博客
export const blogApi = {
  listPublic(page: number = 1, pageSize: number = 12, categoryId?: number, tag?: string) {
    return api.get('/blogs', { params: { page, page_size: pageSize, category_id: categoryId || undefined, tag: tag || undefined } });
  },
  getBlog(id: number) {
    return api.get(`/blogs/${id}`);
  },
  listByAuthor(authorId: number, page: number = 1, categoryId?: number, search?: string, excludeCategoryId?: number) {
    return api.get(`/author/${authorId}/blogs`, {
      params: {
        page,
        category_id: categoryId !== undefined ? categoryId : undefined,
        search: search || undefined,
        exclude_category_id: excludeCategoryId || undefined,
      },
    });
  },
  // 批量设置博客分类（CSDN 式批量操作；category_id=0 移出到未分类）
  batchSetCategory(ids: number[], categoryId: number) {
    return api.post('/blogs/batch-category', { ids, category_id: categoryId });
  },
  getAuthorBlog(authorId: number, blogId: number) {
    return api.get(`/author/${authorId}/blogs/${blogId}`);
  },
  create(data: { title: string; content: string; summary?: string; category_id?: number; tags?: string[]; topics?: string[] }) {
    return api.post('/blogs', data);
  },
  // 批量导入博客：每个文件一篇，标题取文件名（去扩展名）；categoryId 可选=导入到指定分类（CSDN 式）
  importBatch(files: File[], categoryId?: number) {
    const formData = new FormData();
    for (const f of files) formData.append('files', f);
    if (categoryId) formData.append('category_id', String(categoryId));
    return api.post<{ code: number; data: { count: number; skipped: number; message: string; created_ids?: number[] } }>(
      '/blogs/import/batch',
      formData,
    );
  },
  update(id: number, data: { title?: string; content?: string; summary?: string; is_pinned?: boolean; category_id?: number; tags?: string[] | null; topics?: string[] | null }) {
    return api.put(`/blogs/${id}`, data);
  },
  delete(id: number) {
    return api.delete(`/blogs/${id}`);
  },
  // ===== 标签（#tag，CSDN 式）=====
  getTags() {
    return api.get<{ code: number; data: { name: string; count: number }[] }>('/blog-tags');
  },
  // ===== 话题（#话题，CSDN 式内容聚合）=====
  getTopics() {
    return api.get<{ code: number; data: { id: number; name: string; description: string; blog_count: number }[] }>('/topics');
  },
  getTopic(id: number) {
    return api.get<{ code: number; data: { topic: any; blog_count: number } }>(`/topics/${id}`);
  },
  getTopicBlogs(id: number, page: number = 1, pageSize: number = 12) {
    return api.get(`/topics/${id}/blogs`, { params: { page, page_size: pageSize } });
  },
  createTopic(data: { name: string; description?: string }) {
    return api.post('/topics', data);
  },

  // ===== 分类（树）=====
  getCategories() {
    return api.get<{ code: number; data: BlogCategoryNode[] }>('/blog-categories');
  },
  // 某作者的已公开分类树（个人专栏）
  getAuthorCategories(authorKey: number | string) {
    return api.get<{ code: number; data: BlogCategoryNode[] }>(`/author/${authorKey}/blog-categories`);
  },
  // 分类管理视图（登录）：mine=自己创建的分类（个人专栏，含待审核状态）
  getManageCategories() {
    return api.get<{ code: number; data: { platform: BlogCategoryNode[]; mine: any[] } }>('/blog-categories/manage');
  },
  // 分类管理（登录作者/管理员可用：新增/编辑/删除/挂靠，支持三级；普通作者创建=待审核）
  createCategory(data: { name: string; parent_id?: number; sort_order?: number }) {
    return api.post('/blog-categories', data);
  },
  updateCategory(id: number, data: { name?: string; parent_id?: number; sort_order?: number }) {
    return api.put(`/blog-categories/${id}`, data);
  },
  deleteCategory(id: number) {
    return api.delete(`/blog-categories/${id}`);
  },
  // ===== 点赞 =====
  getLikes(id: number) {
    return api.get<{ code: number; data: { count: number; liked: boolean } }>(`/blogs/${id}/likes`);
  },
  toggleLike(id: number) {
    return api.post<{ code: number; data: { liked: boolean; count: number } }>(`/blogs/${id}/like`);
  },
};

// 通用文件上传（博客图片/音视频，CSDN 式编辑器）
export const uploadApi = {
  upload(file: File) {
    const fd = new FormData();
    fd.append('file', file);
    return api.post<{ code: number; data: { url: string; size: number; name: string; ext: string } }>('/upload', fd, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 120000,
    });
  },
};

export interface BlogCategoryNode {
  id: number;
  parent_id: number;
  name: string;
  sort_order: number;
  blog_count: number;
  children?: BlogCategoryNode[];
}

export interface AuthorBlog {
  id: number;
  author_id: number;
  title: string;
  content: string;
  summary: string;
  is_pinned: boolean;
  view_count: number;
  created_at: string;
  updated_at: string;
  author?: {
    id: number;
    username: string;
    nickname: string;
    avatar_url: string;
  };
}