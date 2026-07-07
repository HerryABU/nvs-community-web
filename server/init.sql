-- NVS 网络小说平台 数据库初始化
-- 创建数据库（如不存在）
CREATE DATABASE IF NOT EXISTS nvs CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE nvs;

-- ==================== 用户表 ====================
CREATE TABLE IF NOT EXISTS users (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(64) NOT NULL UNIQUE,
    email VARCHAR(128) NOT NULL UNIQUE,
    password_hash VARCHAR(256) NOT NULL,
    nickname VARCHAR(64) DEFAULT '',
    avatar_url VARCHAR(512) DEFAULT '',
    bio TEXT,
    role ENUM('reader', 'author', 'vip_author', 'arbitrator', 'admin') DEFAULT 'reader',
    real_name_verified BOOLEAN DEFAULT FALSE,
    login_fail_count INT DEFAULT 0,
    locked_until DATETIME NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_email (email),
    INDEX idx_role (role),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB;

-- ==================== 作品表 ====================
CREATE TABLE IF NOT EXISTS novels (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    author_id BIGINT NOT NULL,
    title VARCHAR(256) NOT NULL,
    category VARCHAR(64) NOT NULL DEFAULT '其他',
    tags JSON,
    summary TEXT,
    cover_url VARCHAR(512) DEFAULT '',
    price_per_chapter DECIMAL(10,2) DEFAULT 0.00,
    status ENUM('draft', 'published', 'hidden') DEFAULT 'draft',
    total_words INT DEFAULT 0,
    total_chapters INT DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_author (author_id),
    INDEX idx_category (category),
    INDEX idx_status (status),
    INDEX idx_created_at (created_at),
    FULLTEXT idx_title_summary (title, summary)
) ENGINE=InnoDB;

-- ==================== 章节表 ====================
CREATE TABLE IF NOT EXISTS chapters (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    novel_id BIGINT NOT NULL,
    chapter_number INT NOT NULL,
    title VARCHAR(256) NOT NULL,
    content_path VARCHAR(512) NOT NULL,
    word_count INT DEFAULT 0,
    status ENUM('draft', 'published') DEFAULT 'draft',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (novel_id) REFERENCES novels(id) ON DELETE CASCADE,
    UNIQUE KEY uk_novel_chapter (novel_id, chapter_number),
    INDEX idx_novel (novel_id)
) ENGINE=InnoDB;

-- ==================== 评论表 ====================
CREATE TABLE IF NOT EXISTS comments (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    novel_id BIGINT NOT NULL,
    chapter_number INT DEFAULT 0,
    content TEXT NOT NULL,
    quote_text VARCHAR(1024) DEFAULT '',
    quote_offset INT DEFAULT 0,
    parent_id BIGINT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (novel_id) REFERENCES novels(id) ON DELETE CASCADE,
    FOREIGN KEY (parent_id) REFERENCES comments(id) ON DELETE CASCADE,
    INDEX idx_novel (novel_id),
    INDEX idx_user (user_id),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB;

-- ==================== 打赏表 ====================
CREATE TABLE IF NOT EXISTS tips (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    from_user_id BIGINT NOT NULL,
    to_user_id BIGINT NOT NULL,
    novel_id BIGINT NULL,
    amount DECIMAL(10,2) NOT NULL,
    message VARCHAR(512) DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (from_user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (to_user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (novel_id) REFERENCES novels(id) ON DELETE SET NULL,
    INDEX idx_to_user (to_user_id),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB;

-- ==================== 收藏表 ====================
CREATE TABLE IF NOT EXISTS favorites (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    novel_id BIGINT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (novel_id) REFERENCES novels(id) ON DELETE CASCADE,
    UNIQUE KEY uk_user_novel (user_id, novel_id)
) ENGINE=InnoDB;

-- ==================== 评分表 ====================
CREATE TABLE IF NOT EXISTS ratings (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    novel_id BIGINT NOT NULL,
    type_completion INT DEFAULT 3,
    narrative_quality INT DEFAULT 3,
    thought_depth INT DEFAULT 3,
    community_reputation INT DEFAULT 3,
    update_stability INT DEFAULT 3,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (novel_id) REFERENCES novels(id) ON DELETE CASCADE,
    UNIQUE KEY uk_user_novel_rating (user_id, novel_id)
) ENGINE=InnoDB;

-- ==================== 操作审计日志 ====================
CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    operator_id BIGINT NOT NULL,
    action VARCHAR(64) NOT NULL,
    target_type VARCHAR(32) NOT NULL,
    target_id BIGINT NULL,
    detail JSON,
    ip_address VARCHAR(45) DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (operator_id) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_operator (operator_id),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB;
