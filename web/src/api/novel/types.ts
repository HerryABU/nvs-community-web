export interface Novel {
  id: number;
  author_id: number;
  title: string;
  category: string;
  categories?: string[];
  tags: string[];
  summary: string;
  cover_url: string;
  price_per_chapter: number;
  status: string;
  source_type?: string;       // original / reprint
  creation_method?: string;   // human / ai / human_ai_assisted
  total_words: number;
  total_chapters: number;
  created_at: string;
  updated_at: string;
  wall_enabled?: boolean;
  wall_warning?: string;
  author_name?: string;
  author?: {
    id: number;
    username: string;
    nickname: string;
    avatar_url: string;
  };
}

export interface Chapter {
  id: number;
  novel_id: number;
  chapter_number: number;
  title: string;
  content?: string;
  content_hash?: string;
  word_count: number;
  status: string;
  created_at: string;
}

export interface ChapterDetail {
  chapter: Chapter;
  html_content?: string;
  hash: string;
  hash_match: boolean;
  signature_verified?: boolean;
  file_size: number;
  modified_at: number;
}

export interface ChapterVerify {
  novel_id: number;
  chapter_number: number;
  title: string;
  hash_db: string;
  hash_current: string;
  hash_verified: boolean;
  file_size: number;
  modified_at: number;
  message: string;
}

export interface Comment {
  id: number;
  user_id: number;
  novel_id: number;
  blog_id?: number;
  chapter_number: number;
  content: string;
  quote_text: string;
  parent_id: number | null;
  // 楼层模型：root_id = 所属楼的顶层评论 id（顶层为 null）；floor_no = 楼层号（楼内回复为 0）
  root_id?: number | null;
  floor_no?: number;
  reply_to_name?: string; // 被回复者昵称（展示「X 回复 Y」）
  has_password?: boolean; // 楼加密（入口限制）：该楼是否设有访问密码
  locked?: boolean;       // 运行时脱敏标记：当前请求未解锁（前端渲染 🔒 占位）
  username?: string;
  user?: { id: number; username?: string; nickname?: string; avatar_url?: string };
  created_at: string;
}

export interface NovelListParams {
  page?: number;
  page_size?: number;
  category?: string;
  search?: string;
  status?: string;
}
