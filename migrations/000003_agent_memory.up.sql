-- 8V 记忆库：Agent 跨对话保留的长期事实。
--
-- 这里只存"耐久"的偏好和约束（"爸爸不吃辣"、"周日采购"），
-- 会变化的数据（库存、账单、提醒）仍然通过工具实时查询，不进记忆库。
SET NAMES utf8mb4;
SET time_zone = '+08:00';

CREATE TABLE IF NOT EXISTS agent_memories (
    id                     BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    household_id           BIGINT UNSIGNED NOT NULL,

    -- scope = 'HOUSEHOLD' 时 member_id 必须为 NULL，全家可见；
    -- scope = 'MEMBER'    时 member_id 为归属成员，只进该成员的提示词。
    scope                  VARCHAR(16) NOT NULL,
    member_id              BIGINT UNSIGNED NULL,

    content                VARCHAR(500) NOT NULL,

    -- content 的 SHA-256。唯一键放在哈希上而不是 content 上：
    -- utf8mb4 的 VARCHAR(500) 超过 InnoDB 3072 字节的索引上限。
    content_hash           BINARY(32) NOT NULL,

    -- MySQL 的唯一键把多个 NULL 视为互不相同，家庭级记忆（member_id IS NULL）
    -- 因此无法靠 member_id 去重。这个生成列把 NULL 折叠成 0，
    -- 让两种作用域都能命中同一个唯一键。
    member_key             BIGINT UNSIGNED AS (COALESCE(member_id, 0)) STORED NOT NULL,

    source_conversation_id BIGINT UNSIGNED NULL,
    created_by             BIGINT UNSIGNED NOT NULL,
    version                BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at             DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at             DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
                               ON UPDATE CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),

    -- 重复"记住"同一件事时命中唯一键，改为幂等更新，
    -- 而不是让提示词里堆满同义条目。
    UNIQUE KEY uk_agent_memories_content (household_id, member_key, content_hash),

    KEY idx_agent_memories_scope (household_id, scope, member_id),

    CONSTRAINT fk_agent_memories_household
        FOREIGN KEY (household_id)
        REFERENCES households (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_agent_memories_member
        FOREIGN KEY (member_id)
        REFERENCES members (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_agent_memories_created_by
        FOREIGN KEY (created_by)
        REFERENCES members (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT chk_agent_memories_scope
        CHECK (scope IN ('HOUSEHOLD', 'MEMBER')),

    -- 作用域和归属成员必须一致，否则家庭级记忆会被当成某个成员的私有记忆
    -- （或者反过来）漏进别人的提示词。
    CONSTRAINT chk_agent_memories_member_scope
        CHECK (
            (scope = 'HOUSEHOLD' AND member_id IS NULL)
            OR (scope = 'MEMBER' AND member_id IS NOT NULL)
        ),

    CONSTRAINT chk_agent_memories_content
        CHECK (CHAR_LENGTH(TRIM(content)) > 0),
    CONSTRAINT chk_agent_memories_version
        CHECK (version > 0)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;
