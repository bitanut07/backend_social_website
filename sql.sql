-- Artly Social - schema và dữ liệu mẫu cho PostgreSQL 17.
-- Đây là SQL thuần, chạy trực tiếp được trong Supabase SQL Editor, psql
-- hoặc Docker. Hãy chọn đúng database trước khi chạy file.
-- File có thể chạy lại an toàn nhờ IF NOT EXISTS và ON CONFLICT.

SET client_encoding = 'UTF8';
SET TIME ZONE 'UTC';
SET search_path TO public;

BEGIN;

-- CREATE TABLE IF NOT EXISTS không thể tự đổi schema ID số đã tồn tại sang UUID.
-- Dừng sớm với thông báo rõ ràng thay vì để seed lỗi kiểu dữ liệu ở cuối file.
DO $$
DECLARE
  incompatible_table TEXT;
  incompatible_column TEXT;
  incompatible_type TEXT;
BEGIN
  WITH expected(table_name, column_name) AS (
    VALUES
      ('users', 'id'),
      ('topics', 'id'),
      ('topic_aliases', 'id'),
      ('topic_aliases', 'topic_id'),
      ('posts', 'id'),
      ('posts', 'user_id'),
      ('post_topics', 'post_id'),
      ('post_topics', 'topic_id'),
      ('reactions', 'id'),
      ('reactions', 'post_id'),
      ('reactions', 'user_id'),
      ('messages', 'id'),
      ('messages', 'sender_id'),
      ('messages', 'receiver_id')
  )
  SELECT
    expected.table_name,
    expected.column_name,
    COALESCE(columns.udt_name, 'missing')
  INTO incompatible_table, incompatible_column, incompatible_type
  FROM expected
  JOIN information_schema.tables AS tables
    ON tables.table_schema = current_schema()
    AND tables.table_name = expected.table_name
    AND tables.table_type = 'BASE TABLE'
  LEFT JOIN information_schema.columns AS columns
    ON columns.table_schema = current_schema()
    AND columns.table_name = expected.table_name
    AND columns.column_name = expected.column_name
  WHERE columns.udt_name IS DISTINCT FROM 'uuid'
  ORDER BY expected.table_name, expected.column_name
  LIMIT 1;

  IF incompatible_table IS NOT NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = format(
        'Schema Artly hiện có không tương thích: %s.%s có kiểu %s, phải là UUID.',
        incompatible_table,
        incompatible_column,
        incompatible_type
      ),
      HINT = 'Hãy sao lưu dữ liệu, xóa các bảng Artly demo cũ rồi chạy lại toàn bộ sql.sql.';
  END IF;
END;
$$;

CREATE TABLE IF NOT EXISTS users (
  id UUID NOT NULL DEFAULT gen_random_uuid(),
  username VARCHAR(50) NOT NULL,
  display_name VARCHAR(100) NOT NULL,
  role VARCHAR(20) NOT NULL,
  avatar_url VARCHAR(2048),
  created_at TIMESTAMPTZ(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  CONSTRAINT users_username_unique UNIQUE (username),
  CONSTRAINT chk_users_role
    CHECK (role IN ('STUDENT', 'TEACHER')),
  CONSTRAINT chk_users_username_format
    CHECK (username ~ '^[a-z0-9._-]{3,50}$'),
  CONSTRAINT chk_users_display_name_length
    CHECK (CHAR_LENGTH(TRIM(display_name)) BETWEEN 1 AND 100),
  CONSTRAINT chk_users_avatar_url
    CHECK (
      avatar_url IS NULL
      OR avatar_url LIKE 'http://%'
      OR avatar_url LIKE 'https://%'
    )
);

CREATE TABLE IF NOT EXISTS topics (
  id UUID NOT NULL DEFAULT gen_random_uuid(),
  slug VARCHAR(100) NOT NULL,
  name VARCHAR(100) NOT NULL,
  normalized_name VARCHAR(100) NOT NULL,
  created_at TIMESTAMPTZ(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  CONSTRAINT topics_slug_unique UNIQUE (slug),
  CONSTRAINT topics_normalized_name_unique UNIQUE (normalized_name),
  CONSTRAINT chk_topics_name_length
    CHECK (CHAR_LENGTH(TRIM(name)) BETWEEN 1 AND 100),
  CONSTRAINT chk_topics_normalized_name_length
    CHECK (CHAR_LENGTH(TRIM(normalized_name)) BETWEEN 1 AND 100),
  CONSTRAINT chk_topics_slug_format
    CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$')
);

CREATE TABLE IF NOT EXISTS topic_aliases (
  id UUID NOT NULL DEFAULT gen_random_uuid(),
  topic_id UUID NOT NULL,
  alias VARCHAR(100) NOT NULL,
  normalized_alias VARCHAR(100) NOT NULL,
  created_at TIMESTAMPTZ(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  CONSTRAINT topic_aliases_normalized_alias_unique UNIQUE (normalized_alias),
  CONSTRAINT fk_topic_aliases_topic
    FOREIGN KEY (topic_id) REFERENCES topics (id)
    ON UPDATE CASCADE ON DELETE CASCADE,
  CONSTRAINT chk_topic_aliases_length
    CHECK (
      CHAR_LENGTH(TRIM(alias)) BETWEEN 1 AND 100
      AND CHAR_LENGTH(TRIM(normalized_alias)) BETWEEN 1 AND 100
    )
);

CREATE TABLE IF NOT EXISTS posts (
  id UUID NOT NULL DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL,
  title VARCHAR(120) NOT NULL,
  caption TEXT NOT NULL,
  image_url VARCHAR(2048) NOT NULL,
  exam_name VARCHAR(160),
  status VARCHAR(20) NOT NULL DEFAULT 'PUBLISHED',
  created_at TIMESTAMPTZ(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  CONSTRAINT fk_posts_user
    FOREIGN KEY (user_id) REFERENCES users (id)
    ON UPDATE CASCADE ON DELETE RESTRICT,
  CONSTRAINT chk_posts_status
    CHECK (status IN ('PUBLISHED', 'ARCHIVED')),
  CONSTRAINT chk_posts_title_length
    CHECK (CHAR_LENGTH(TRIM(title)) BETWEEN 1 AND 120),
  CONSTRAINT chk_posts_caption_length
    CHECK (CHAR_LENGTH(TRIM(caption)) BETWEEN 1 AND 2000),
  CONSTRAINT chk_posts_image_url
    CHECK (image_url LIKE 'http://%' OR image_url LIKE 'https://%'),
  CONSTRAINT chk_posts_exam_name_length
    CHECK (
      exam_name IS NULL
      OR CHAR_LENGTH(TRIM(exam_name)) BETWEEN 1 AND 160
    )
);

CREATE TABLE IF NOT EXISTS post_topics (
  post_id UUID NOT NULL,
  topic_id UUID NOT NULL,
  created_at TIMESTAMPTZ(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (post_id, topic_id),
  CONSTRAINT fk_post_topics_post
    FOREIGN KEY (post_id) REFERENCES posts (id)
    ON UPDATE CASCADE ON DELETE CASCADE,
  CONSTRAINT fk_post_topics_topic
    FOREIGN KEY (topic_id) REFERENCES topics (id)
    ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS reactions (
  id UUID NOT NULL DEFAULT gen_random_uuid(),
  post_id UUID NOT NULL,
  user_id UUID NOT NULL,
  type VARCHAR(20) NOT NULL DEFAULT 'LIKE',
  created_at TIMESTAMPTZ(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  CONSTRAINT reactions_post_user_unique UNIQUE (post_id, user_id),
  CONSTRAINT fk_reactions_post
    FOREIGN KEY (post_id) REFERENCES posts (id)
    ON UPDATE CASCADE ON DELETE CASCADE,
  CONSTRAINT fk_reactions_user
    FOREIGN KEY (user_id) REFERENCES users (id)
    ON UPDATE CASCADE ON DELETE CASCADE,
  CONSTRAINT chk_reactions_type
    CHECK (type IN ('LIKE', 'LOVE', 'CLAP'))
);

CREATE TABLE IF NOT EXISTS messages (
  id UUID NOT NULL DEFAULT gen_random_uuid(),
  sender_id UUID NOT NULL,
  receiver_id UUID NOT NULL,
  body TEXT NOT NULL,
  is_read BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  CONSTRAINT fk_messages_sender
    FOREIGN KEY (sender_id) REFERENCES users (id)
    ON UPDATE CASCADE ON DELETE RESTRICT,
  CONSTRAINT fk_messages_receiver
    FOREIGN KEY (receiver_id) REFERENCES users (id)
    ON UPDATE CASCADE ON DELETE RESTRICT,
  CONSTRAINT chk_messages_body_length
    CHECK (CHAR_LENGTH(TRIM(body)) BETWEEN 1 AND 2000),
  CONSTRAINT chk_messages_is_read
    CHECK (is_read IN (FALSE, TRUE))
);

-- Các index phục vụ bảng tin, tra cứu chủ đề, reaction và hội thoại.
CREATE INDEX IF NOT EXISTS users_role_index
  ON users (role);

CREATE INDEX IF NOT EXISTS topics_name_index
  ON topics (name);

CREATE INDEX IF NOT EXISTS topic_aliases_topic_index
  ON topic_aliases (topic_id);

CREATE INDEX IF NOT EXISTS posts_status_created_id_index
  ON posts (status, created_at, id);

CREATE INDEX IF NOT EXISTS posts_user_created_id_index
  ON posts (user_id, created_at, id);

CREATE INDEX IF NOT EXISTS post_topics_topic_post_index
  ON post_topics (topic_id, post_id);

CREATE INDEX IF NOT EXISTS reactions_user_created_index
  ON reactions (user_id, created_at);

CREATE INDEX IF NOT EXISTS messages_sender_receiver_created_index
  ON messages (sender_id, receiver_id, created_at, id);

CREATE INDEX IF NOT EXISTS messages_receiver_sender_created_index
  ON messages (receiver_id, sender_id, created_at, id);

CREATE INDEX IF NOT EXISTS messages_receiver_unread_index
  ON messages (receiver_id, is_read, created_at);

-- PostgreSQL không có ON UPDATE CURRENT_TIMESTAMP ở định nghĩa cột.
-- Trigger dùng chung này duy trì updated_at cho mọi bảng có thể cập nhật.
CREATE OR REPLACE FUNCTION artly_set_updated_at()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  NEW.updated_at := CURRENT_TIMESTAMP;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS users_set_updated_at ON users;
CREATE TRIGGER users_set_updated_at
  BEFORE UPDATE ON users
  FOR EACH ROW
  EXECUTE FUNCTION artly_set_updated_at();

DROP TRIGGER IF EXISTS topics_set_updated_at ON topics;
CREATE TRIGGER topics_set_updated_at
  BEFORE UPDATE ON topics
  FOR EACH ROW
  EXECUTE FUNCTION artly_set_updated_at();

DROP TRIGGER IF EXISTS topic_aliases_set_updated_at ON topic_aliases;
CREATE TRIGGER topic_aliases_set_updated_at
  BEFORE UPDATE ON topic_aliases
  FOR EACH ROW
  EXECUTE FUNCTION artly_set_updated_at();

DROP TRIGGER IF EXISTS posts_set_updated_at ON posts;
CREATE TRIGGER posts_set_updated_at
  BEFORE UPDATE ON posts
  FOR EACH ROW
  EXECUTE FUNCTION artly_set_updated_at();

DROP TRIGGER IF EXISTS post_topics_set_updated_at ON post_topics;
CREATE TRIGGER post_topics_set_updated_at
  BEFORE UPDATE ON post_topics
  FOR EACH ROW
  EXECUTE FUNCTION artly_set_updated_at();

DROP TRIGGER IF EXISTS reactions_set_updated_at ON reactions;
CREATE TRIGGER reactions_set_updated_at
  BEFORE UPDATE ON reactions
  FOR EACH ROW
  EXECUTE FUNCTION artly_set_updated_at();

DROP TRIGGER IF EXISTS messages_set_updated_at ON messages;
CREATE TRIGGER messages_set_updated_at
  BEFORE UPDATE ON messages
  FOR EACH ROW
  EXECUTE FUNCTION artly_set_updated_at();

-- Tài khoản demo: dùng một trong ba UUID dưới đây trong header X-User-ID.
INSERT INTO users
  (id, username, display_name, role, avatar_url)
VALUES
  (
    '00000000-0000-4000-8000-000000000001',
    'minh.an',
    'Trần Minh An',
    'STUDENT',
    'https://i.pravatar.cc/256?img=12'
  ),
  (
    '00000000-0000-4000-8000-000000000002',
    'co.lan',
    'Cô Nguyễn Hoài Lan',
    'TEACHER',
    'https://i.pravatar.cc/256?img=47'
  ),
  (
    '00000000-0000-4000-8000-000000000003',
    'phuong.thao',
    'Nguyễn Phương Thảo',
    'STUDENT',
    'https://i.pravatar.cc/256?img=32'
  )
ON CONFLICT (id) DO UPDATE SET
  username = EXCLUDED.username,
  display_name = EXCLUDED.display_name,
  role = EXCLUDED.role,
  avatar_url = EXCLUDED.avatar_url;

INSERT INTO topics
  (id, slug, name, normalized_name)
VALUES
  (
    '10000000-0000-4000-8000-000000000001',
    'phong-canh',
    'Phong cảnh',
    'phong canh'
  ),
  (
    '10000000-0000-4000-8000-000000000002',
    'chan-dung',
    'Chân dung',
    'chan dung'
  ),
  (
    '10000000-0000-4000-8000-000000000003',
    'moi-truong',
    'Môi trường',
    'moi truong'
  ),
  (
    '10000000-0000-4000-8000-000000000004',
    'hoa-binh',
    'Hòa bình',
    'hoa binh'
  ),
  (
    '10000000-0000-4000-8000-000000000005',
    'di-san-van-hoa',
    'Di sản văn hóa',
    'di san van hoa'
  ),
  (
    '10000000-0000-4000-8000-000000000006',
    'uoc-mo',
    'Ước mơ',
    'uoc mo'
  )
ON CONFLICT (id) DO UPDATE SET
  slug = EXCLUDED.slug,
  name = EXCLUDED.name,
  normalized_name = EXCLUDED.normalized_name;

INSERT INTO topic_aliases
  (id, topic_id, alias, normalized_alias)
VALUES
  (
    '30000000-0000-4000-8000-000000000001',
    '10000000-0000-4000-8000-000000000001',
    'Cảnh vật',
    'canh vat'
  ),
  (
    '30000000-0000-4000-8000-000000000002',
    '10000000-0000-4000-8000-000000000001',
    'Landscape',
    'landscape'
  ),
  (
    '30000000-0000-4000-8000-000000000003',
    '10000000-0000-4000-8000-000000000002',
    'Vẽ người',
    've nguoi'
  ),
  (
    '30000000-0000-4000-8000-000000000004',
    '10000000-0000-4000-8000-000000000002',
    'Portrait',
    'portrait'
  ),
  (
    '30000000-0000-4000-8000-000000000005',
    '10000000-0000-4000-8000-000000000003',
    'Bảo vệ môi trường',
    'bao ve moi truong'
  ),
  (
    '30000000-0000-4000-8000-000000000006',
    '10000000-0000-4000-8000-000000000003',
    'Environment',
    'environment'
  ),
  (
    '30000000-0000-4000-8000-000000000007',
    '10000000-0000-4000-8000-000000000004',
    'Tình bạn',
    'tinh ban'
  ),
  (
    '30000000-0000-4000-8000-000000000008',
    '10000000-0000-4000-8000-000000000004',
    'Peace',
    'peace'
  ),
  (
    '30000000-0000-4000-8000-000000000009',
    '10000000-0000-4000-8000-000000000005',
    'Di sản',
    'di san'
  ),
  (
    '30000000-0000-4000-8000-000000000010',
    '10000000-0000-4000-8000-000000000005',
    'Heritage',
    'heritage'
  ),
  (
    '30000000-0000-4000-8000-000000000011',
    '10000000-0000-4000-8000-000000000006',
    'Giấc mơ',
    'giac mo'
  ),
  (
    '30000000-0000-4000-8000-000000000012',
    '10000000-0000-4000-8000-000000000006',
    'Dream',
    'dream'
  )
ON CONFLICT (id) DO UPDATE SET
  topic_id = EXCLUDED.topic_id,
  alias = EXCLUDED.alias,
  normalized_alias = EXCLUDED.normalized_alias;

INSERT INTO posts
  (
    id,
    user_id,
    title,
    caption,
    image_url,
    exam_name,
    status
  )
VALUES
  (
    '20000000-0000-4000-8000-000000000001',
    '00000000-0000-4000-8000-000000000001',
    'Mầm xanh tương lai',
    'Bài vẽ màu nước về những bàn tay cùng nâng niu một mầm cây xanh.',
    'http://localhost:5173/demo-art/mam-xanh-tuong-lai.png',
    'Sắc màu xanh 2026',
    'PUBLISHED'
  ),
  (
    '20000000-0000-4000-8000-000000000002',
    '00000000-0000-4000-8000-000000000003',
    'Hòa bình trong em',
    'Chim bồ câu và những bàn tay nhiều sắc màu thể hiện tình bạn học đường.',
    'http://localhost:5173/demo-art/hoa-binh-trong-em.png',
    'Thiếu nhi Việt Nam vẽ hòa bình',
    'PUBLISHED'
  ),
  (
    '20000000-0000-4000-8000-000000000003',
    '00000000-0000-4000-8000-000000000001',
    'Di sản quê em',
    'Em chọn phố cổ Hội An, đèn lồng và dòng sông làm cảm hứng cho bài thi.',
    'http://localhost:5173/demo-art/di-san-que-em.png',
    'Di sản trong mắt em',
    'PUBLISHED'
  ),
  (
    '20000000-0000-4000-8000-000000000004',
    '00000000-0000-4000-8000-000000000003',
    'Chân dung người truyền cảm hứng',
    'Bài chân dung cô giáo mỹ thuật đã giúp em tự tin hơn với màu sắc.',
    'https://images.unsplash.com/photo-1549490349-8643362247b5?auto=format&fit=crop&w=1200&q=80',
    'Người thầy trong em',
    'PUBLISHED'
  ),
  (
    '20000000-0000-4000-8000-000000000005',
    '00000000-0000-4000-8000-000000000001',
    'Thành phố trên mây',
    'Em tưởng tượng một thành phố xanh có những khu vườn bay giữa bầu trời.',
    'https://images.unsplash.com/photo-1541961017774-22349e4a1262?auto=format&fit=crop&w=1200&q=80',
    'Ước mơ của em',
    'PUBLISHED'
  )
ON CONFLICT (id) DO UPDATE SET
  user_id = EXCLUDED.user_id,
  title = EXCLUDED.title,
  caption = EXCLUDED.caption,
  image_url = EXCLUDED.image_url,
  exam_name = EXCLUDED.exam_name,
  status = EXCLUDED.status;

INSERT INTO post_topics (post_id, topic_id)
VALUES
  (
    '20000000-0000-4000-8000-000000000001',
    '10000000-0000-4000-8000-000000000001'
  ),
  (
    '20000000-0000-4000-8000-000000000001',
    '10000000-0000-4000-8000-000000000003'
  ),
  (
    '20000000-0000-4000-8000-000000000002',
    '10000000-0000-4000-8000-000000000004'
  ),
  (
    '20000000-0000-4000-8000-000000000003',
    '10000000-0000-4000-8000-000000000005'
  ),
  (
    '20000000-0000-4000-8000-000000000004',
    '10000000-0000-4000-8000-000000000002'
  ),
  (
    '20000000-0000-4000-8000-000000000005',
    '10000000-0000-4000-8000-000000000001'
  ),
  (
    '20000000-0000-4000-8000-000000000005',
    '10000000-0000-4000-8000-000000000006'
  )
ON CONFLICT (post_id, topic_id) DO NOTHING;

INSERT INTO reactions
  (id, post_id, user_id, type)
VALUES
  (
    '40000000-0000-4000-8000-000000000001',
    '20000000-0000-4000-8000-000000000001',
    '00000000-0000-4000-8000-000000000002',
    'LOVE'
  ),
  (
    '40000000-0000-4000-8000-000000000002',
    '20000000-0000-4000-8000-000000000001',
    '00000000-0000-4000-8000-000000000003',
    'CLAP'
  ),
  (
    '40000000-0000-4000-8000-000000000003',
    '20000000-0000-4000-8000-000000000002',
    '00000000-0000-4000-8000-000000000001',
    'LIKE'
  ),
  (
    '40000000-0000-4000-8000-000000000004',
    '20000000-0000-4000-8000-000000000003',
    '00000000-0000-4000-8000-000000000002',
    'LOVE'
  ),
  (
    '40000000-0000-4000-8000-000000000005',
    '20000000-0000-4000-8000-000000000004',
    '00000000-0000-4000-8000-000000000001',
    'CLAP'
  ),
  (
    '40000000-0000-4000-8000-000000000006',
    '20000000-0000-4000-8000-000000000005',
    '00000000-0000-4000-8000-000000000002',
    'LIKE'
  )
ON CONFLICT (post_id, user_id) DO UPDATE SET
  type = EXCLUDED.type;

INSERT INTO messages
  (id, sender_id, receiver_id, body, is_read)
VALUES
  (
    '50000000-0000-4000-8000-000000000001',
    '00000000-0000-4000-8000-000000000001',
    '00000000-0000-4000-8000-000000000002',
    'Cô xem giúp em phần phối màu của bài Mầm xanh tương lai với ạ.',
    TRUE
  ),
  (
    '50000000-0000-4000-8000-000000000002',
    '00000000-0000-4000-8000-000000000002',
    '00000000-0000-4000-8000-000000000001',
    'Em thử tăng độ tương phản ở vùng tiền cảnh nhé, bố cục đang rất tốt.',
    TRUE
  ),
  (
    '50000000-0000-4000-8000-000000000003',
    '00000000-0000-4000-8000-000000000003',
    '00000000-0000-4000-8000-000000000002',
    'Cô ơi, bài Hòa bình trong em của em đã đủ rõ chủ đề chưa ạ?',
    TRUE
  ),
  (
    '50000000-0000-4000-8000-000000000004',
    '00000000-0000-4000-8000-000000000002',
    '00000000-0000-4000-8000-000000000003',
    'Chủ đề rất rõ rồi em. Em có thể làm nổi bật chim bồ câu thêm một chút.',
    FALSE
  )
ON CONFLICT (id) DO UPDATE SET
  sender_id = EXCLUDED.sender_id,
  receiver_id = EXCLUDED.receiver_id,
  body = EXCLUDED.body,
  is_read = EXCLUDED.is_read;

COMMIT;
