-- Artly Social - schema và dữ liệu mẫu cho MySQL 8.
-- Có thể chạy lại file này; DDL dùng IF NOT EXISTS và seed dùng upsert.

CREATE DATABASE IF NOT EXISTS `artly_social`
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_0900_ai_ci;

USE `artly_social`;

SET NAMES utf8mb4 COLLATE utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `users` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `username` VARCHAR(50) NOT NULL,
  `display_name` VARCHAR(100) NOT NULL,
  `role` ENUM('STUDENT', 'TEACHER') NOT NULL,
  `avatar_url` VARCHAR(2048) NULL,
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `users_username_unique` (`username`),
  KEY `users_role_index` (`role`),
  CONSTRAINT `chk_users_username_format`
    CHECK (`username` REGEXP '^[a-z0-9._-]{3,50}$'),
  CONSTRAINT `chk_users_display_name_length`
    CHECK (CHAR_LENGTH(TRIM(`display_name`)) BETWEEN 1 AND 100),
  CONSTRAINT `chk_users_avatar_url`
    CHECK (
      `avatar_url` IS NULL
      OR `avatar_url` LIKE 'http://%'
      OR `avatar_url` LIKE 'https://%'
    )
) ENGINE=InnoDB
  DEFAULT CHARACTER SET utf8mb4
  COLLATE utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `topics` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `slug` VARCHAR(100) NOT NULL,
  `name` VARCHAR(100) NOT NULL,
  `normalized_name` VARCHAR(100) NOT NULL,
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `topics_slug_unique` (`slug`),
  UNIQUE KEY `topics_normalized_name_unique` (`normalized_name`),
  KEY `topics_name_index` (`name`),
  CONSTRAINT `chk_topics_name_length`
    CHECK (CHAR_LENGTH(TRIM(`name`)) BETWEEN 1 AND 100),
  CONSTRAINT `chk_topics_normalized_name_length`
    CHECK (CHAR_LENGTH(TRIM(`normalized_name`)) BETWEEN 1 AND 100),
  CONSTRAINT `chk_topics_slug_format`
    CHECK (`slug` REGEXP '^[a-z0-9]+(-[a-z0-9]+)*$')
) ENGINE=InnoDB
  DEFAULT CHARACTER SET utf8mb4
  COLLATE utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `topic_aliases` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `topic_id` BIGINT UNSIGNED NOT NULL,
  `alias` VARCHAR(100) NOT NULL,
  `normalized_alias` VARCHAR(100) NOT NULL,
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `topic_aliases_normalized_alias_unique` (`normalized_alias`),
  KEY `topic_aliases_topic_index` (`topic_id`),
  CONSTRAINT `fk_topic_aliases_topic`
    FOREIGN KEY (`topic_id`) REFERENCES `topics` (`id`)
    ON UPDATE CASCADE ON DELETE CASCADE,
  CONSTRAINT `chk_topic_aliases_length`
    CHECK (
      CHAR_LENGTH(TRIM(`alias`)) BETWEEN 1 AND 100
      AND CHAR_LENGTH(TRIM(`normalized_alias`)) BETWEEN 1 AND 100
    )
) ENGINE=InnoDB
  DEFAULT CHARACTER SET utf8mb4
  COLLATE utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `posts` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `title` VARCHAR(120) NOT NULL,
  `caption` TEXT NOT NULL,
  `image_url` VARCHAR(2048) NOT NULL,
  `exam_name` VARCHAR(160) NULL,
  `status` ENUM('PUBLISHED', 'ARCHIVED') NOT NULL DEFAULT 'PUBLISHED',
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `posts_status_created_id_index` (`status`, `created_at`, `id`),
  KEY `posts_user_created_id_index` (`user_id`, `created_at`, `id`),
  CONSTRAINT `fk_posts_user`
    FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
    ON UPDATE CASCADE ON DELETE RESTRICT,
  CONSTRAINT `chk_posts_title_length`
    CHECK (CHAR_LENGTH(TRIM(`title`)) BETWEEN 1 AND 120),
  CONSTRAINT `chk_posts_caption_length`
    CHECK (CHAR_LENGTH(TRIM(`caption`)) BETWEEN 1 AND 2000),
  CONSTRAINT `chk_posts_image_url`
    CHECK (`image_url` LIKE 'http://%' OR `image_url` LIKE 'https://%'),
  CONSTRAINT `chk_posts_exam_name_length`
    CHECK (
      `exam_name` IS NULL
      OR CHAR_LENGTH(TRIM(`exam_name`)) BETWEEN 1 AND 160
    )
) ENGINE=InnoDB
  DEFAULT CHARACTER SET utf8mb4
  COLLATE utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `post_topics` (
  `post_id` BIGINT UNSIGNED NOT NULL,
  `topic_id` BIGINT UNSIGNED NOT NULL,
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`post_id`, `topic_id`),
  KEY `post_topics_topic_post_index` (`topic_id`, `post_id`),
  CONSTRAINT `fk_post_topics_post`
    FOREIGN KEY (`post_id`) REFERENCES `posts` (`id`)
    ON UPDATE CASCADE ON DELETE CASCADE,
  CONSTRAINT `fk_post_topics_topic`
    FOREIGN KEY (`topic_id`) REFERENCES `topics` (`id`)
    ON UPDATE CASCADE ON DELETE CASCADE
) ENGINE=InnoDB
  DEFAULT CHARACTER SET utf8mb4
  COLLATE utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `reactions` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `post_id` BIGINT UNSIGNED NOT NULL,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `type` ENUM('LIKE', 'LOVE', 'CLAP') NOT NULL DEFAULT 'LIKE',
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `reactions_post_user_unique` (`post_id`, `user_id`),
  KEY `reactions_user_created_index` (`user_id`, `created_at`),
  CONSTRAINT `fk_reactions_post`
    FOREIGN KEY (`post_id`) REFERENCES `posts` (`id`)
    ON UPDATE CASCADE ON DELETE CASCADE,
  CONSTRAINT `fk_reactions_user`
    FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
    ON UPDATE CASCADE ON DELETE CASCADE
) ENGINE=InnoDB
  DEFAULT CHARACTER SET utf8mb4
  COLLATE utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `messages` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `sender_id` BIGINT UNSIGNED NOT NULL,
  `receiver_id` BIGINT UNSIGNED NOT NULL,
  `body` TEXT NOT NULL,
  `is_read` TINYINT(1) NOT NULL DEFAULT 0,
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `messages_sender_receiver_created_index`
    (`sender_id`, `receiver_id`, `created_at`, `id`),
  KEY `messages_receiver_sender_created_index`
    (`receiver_id`, `sender_id`, `created_at`, `id`),
  KEY `messages_receiver_unread_index`
    (`receiver_id`, `is_read`, `created_at`),
  CONSTRAINT `fk_messages_sender`
    FOREIGN KEY (`sender_id`) REFERENCES `users` (`id`)
    ON UPDATE CASCADE ON DELETE RESTRICT,
  CONSTRAINT `fk_messages_receiver`
    FOREIGN KEY (`receiver_id`) REFERENCES `users` (`id`)
    ON UPDATE CASCADE ON DELETE RESTRICT,
  CONSTRAINT `chk_messages_body_length`
    CHECK (CHAR_LENGTH(TRIM(`body`)) BETWEEN 1 AND 2000),
  CONSTRAINT `chk_messages_is_read`
    CHECK (`is_read` IN (0, 1))
) ENGINE=InnoDB
  DEFAULT CHARACTER SET utf8mb4
  COLLATE utf8mb4_0900_ai_ci;

-- Tài khoản demo: dùng id 1, 2 hoặc 3 trong header X-User-ID.
INSERT INTO `users`
  (`id`, `username`, `display_name`, `role`, `avatar_url`)
VALUES
  (
    1,
    'minh.an',
    'Trần Minh An',
    'STUDENT',
    'https://i.pravatar.cc/256?img=12'
  ),
  (
    2,
    'co.lan',
    'Cô Nguyễn Hoài Lan',
    'TEACHER',
    'https://i.pravatar.cc/256?img=47'
  ),
  (
    3,
    'phuong.thao',
    'Nguyễn Phương Thảo',
    'STUDENT',
    'https://i.pravatar.cc/256?img=32'
  )
ON DUPLICATE KEY UPDATE
  `username` = VALUES(`username`),
  `display_name` = VALUES(`display_name`),
  `role` = VALUES(`role`),
  `avatar_url` = VALUES(`avatar_url`);

INSERT INTO `topics`
  (`id`, `slug`, `name`, `normalized_name`)
VALUES
  (1, 'phong-canh', 'Phong cảnh', 'phong canh'),
  (2, 'chan-dung', 'Chân dung', 'chan dung'),
  (3, 'moi-truong', 'Môi trường', 'moi truong'),
  (4, 'hoa-binh', 'Hòa bình', 'hoa binh'),
  (5, 'di-san-van-hoa', 'Di sản văn hóa', 'di san van hoa'),
  (6, 'uoc-mo', 'Ước mơ', 'uoc mo')
ON DUPLICATE KEY UPDATE
  `slug` = VALUES(`slug`),
  `name` = VALUES(`name`),
  `normalized_name` = VALUES(`normalized_name`);

INSERT INTO `topic_aliases`
  (`id`, `topic_id`, `alias`, `normalized_alias`)
VALUES
  (1, 1, 'Cảnh vật', 'canh vat'),
  (2, 1, 'Landscape', 'landscape'),
  (3, 2, 'Vẽ người', 've nguoi'),
  (4, 2, 'Portrait', 'portrait'),
  (5, 3, 'Bảo vệ môi trường', 'bao ve moi truong'),
  (6, 3, 'Environment', 'environment'),
  (7, 4, 'Tình bạn', 'tinh ban'),
  (8, 4, 'Peace', 'peace'),
  (9, 5, 'Di sản', 'di san'),
  (10, 5, 'Heritage', 'heritage'),
  (11, 6, 'Giấc mơ', 'giac mo'),
  (12, 6, 'Dream', 'dream')
ON DUPLICATE KEY UPDATE
  `topic_id` = VALUES(`topic_id`),
  `alias` = VALUES(`alias`),
  `normalized_alias` = VALUES(`normalized_alias`);

INSERT INTO `posts`
  (
    `id`,
    `user_id`,
    `title`,
    `caption`,
    `image_url`,
    `exam_name`,
    `status`
  )
VALUES
  (
    1,
    1,
    'Mầm xanh tương lai',
    'Bài vẽ màu nước về những bàn tay cùng nâng niu một mầm cây xanh.',
    'http://localhost:5173/demo-art/mam-xanh-tuong-lai.png',
    'Sắc màu xanh 2026',
    'PUBLISHED'
  ),
  (
    2,
    3,
    'Hòa bình trong em',
    'Chim bồ câu và những bàn tay nhiều sắc màu thể hiện tình bạn học đường.',
    'http://localhost:5173/demo-art/hoa-binh-trong-em.png',
    'Thiếu nhi Việt Nam vẽ hòa bình',
    'PUBLISHED'
  ),
  (
    3,
    1,
    'Di sản quê em',
    'Em chọn phố cổ Hội An, đèn lồng và dòng sông làm cảm hứng cho bài thi.',
    'http://localhost:5173/demo-art/di-san-que-em.png',
    'Di sản trong mắt em',
    'PUBLISHED'
  ),
  (
    4,
    3,
    'Chân dung người truyền cảm hứng',
    'Bài chân dung cô giáo mỹ thuật đã giúp em tự tin hơn với màu sắc.',
    'https://images.unsplash.com/photo-1549490349-8643362247b5?auto=format&fit=crop&w=1200&q=80',
    'Người thầy trong em',
    'PUBLISHED'
  ),
  (
    5,
    1,
    'Thành phố trên mây',
    'Em tưởng tượng một thành phố xanh có những khu vườn bay giữa bầu trời.',
    'https://images.unsplash.com/photo-1541961017774-22349e4a1262?auto=format&fit=crop&w=1200&q=80',
    'Ước mơ của em',
    'PUBLISHED'
  )
ON DUPLICATE KEY UPDATE
  `user_id` = VALUES(`user_id`),
  `title` = VALUES(`title`),
  `caption` = VALUES(`caption`),
  `image_url` = VALUES(`image_url`),
  `exam_name` = VALUES(`exam_name`),
  `status` = VALUES(`status`);

INSERT IGNORE INTO `post_topics` (`post_id`, `topic_id`)
VALUES
  (1, 1),
  (1, 3),
  (2, 4),
  (3, 5),
  (4, 2),
  (5, 1),
  (5, 6);

INSERT INTO `reactions`
  (`id`, `post_id`, `user_id`, `type`)
VALUES
  (1, 1, 2, 'LOVE'),
  (2, 1, 3, 'CLAP'),
  (3, 2, 1, 'LIKE'),
  (4, 3, 2, 'LOVE'),
  (5, 4, 1, 'CLAP'),
  (6, 5, 2, 'LIKE')
ON DUPLICATE KEY UPDATE
  `type` = VALUES(`type`);

INSERT INTO `messages`
  (`id`, `sender_id`, `receiver_id`, `body`, `is_read`)
VALUES
  (
    1,
    1,
    2,
    'Cô xem giúp em phần phối màu của bài Mầm xanh tương lai với ạ.',
    1
  ),
  (
    2,
    2,
    1,
    'Em thử tăng độ tương phản ở vùng tiền cảnh nhé, bố cục đang rất tốt.',
    1
  ),
  (
    3,
    3,
    2,
    'Cô ơi, bài Hòa bình trong em của em đã đủ rõ chủ đề chưa ạ?',
    1
  ),
  (
    4,
    2,
    3,
    'Chủ đề rất rõ rồi em. Em có thể làm nổi bật chim bồ câu thêm một chút.',
    0
  )
ON DUPLICATE KEY UPDATE
  `sender_id` = VALUES(`sender_id`),
  `receiver_id` = VALUES(`receiver_id`),
  `body` = VALUES(`body`),
  `is_read` = VALUES(`is_read`);
