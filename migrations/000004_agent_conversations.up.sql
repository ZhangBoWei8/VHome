-- 8V 对话历史：把原本只存在于进程内存里的 history 落到数据库。
--
-- 在这之前每个 HTTP 请求都会新建一个 Agent，上下文用完即弃，
-- 追问（“把刚才那条改成 8 点”）无法工作。
SET NAMES utf8mb4;
SET time_zone = '+08:00';

CREATE TABLE IF NOT EXISTS agent_conversations (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    household_id    BIGINT UNSIGNED NOT NULL,
    member_id       BIGINT UNSIGNED NOT NULL,

    -- 由第一条用户消息截断生成，只用于列表展示。
    title           VARCHAR(128) NOT NULL DEFAULT '',

    last_message_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    message_count   INT UNSIGNED NOT NULL DEFAULT 0,
    version         BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at      DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at      DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
                        ON UPDATE CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),

    -- 列表页固定按成员 + 最近活跃排序。
    KEY idx_agent_conversations_member (member_id, last_message_at DESC),
    KEY idx_agent_conversations_household (household_id),

    -- 清理任务按空闲时间批量删除。
    KEY idx_agent_conversations_idle (last_message_at),

    CONSTRAINT fk_agent_conversations_household
        FOREIGN KEY (household_id)
        REFERENCES households (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_agent_conversations_member
        FOREIGN KEY (member_id)
        REFERENCES members (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT chk_agent_conversations_version
        CHECK (version > 0)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS agent_messages (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    conversation_id BIGINT UNSIGNED NOT NULL,
    household_id    BIGINT UNSIGNED NOT NULL,

    -- 会话内自增序号，决定回放顺序。用它而不是 id，
    -- 是为了让同一轮里成组写入的消息顺序稳定可读。
    seq             INT UNSIGNED NOT NULL,

    -- system 角色刻意不入库：它每轮都由实时快照（当前时间、谁在家）
    -- 加记忆库重新拼装，存下来的副本一定会过期。
    role            VARCHAR(16) NOT NULL,

    content         MEDIUMTEXT NOT NULL,

    -- assistant 消息发起的工具调用，原样保存成 OpenAI 的 tool_calls 结构；
    -- 回放时必须和它对应的 tool 结果一起交给模型，否则接口会报错。
    tool_calls      JSON NULL,

    -- tool 角色专用：这条结果回应的是哪一次调用。
    tool_call_id    VARCHAR(64) NULL,
    tool_name       VARCHAR(64) NULL,

    created_at      DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),
    UNIQUE KEY uk_agent_messages_seq (conversation_id, seq),

    CONSTRAINT fk_agent_messages_conversation
        FOREIGN KEY (conversation_id)
        REFERENCES agent_conversations (id)
        ON UPDATE RESTRICT
        ON DELETE CASCADE,

    CONSTRAINT chk_agent_messages_role
        CHECK (role IN ('user', 'assistant', 'tool'))
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;
