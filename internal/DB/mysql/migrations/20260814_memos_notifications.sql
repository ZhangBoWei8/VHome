-- Upgrade an existing VHome database once before deploying the memo version.
-- Fresh installations should continue to use ../init.sql instead.
SET NAMES utf8mb4;
SET time_zone = '+08:00';

SET @add_member_email = IF(
    (SELECT COUNT(*) FROM information_schema.columns
     WHERE table_schema = DATABASE() AND table_name = 'members' AND column_name = 'email') = 0,
    'ALTER TABLE members ADD COLUMN email VARCHAR(254) NULL AFTER avatar_key',
    'DO 0'
);
PREPARE migration_statement FROM @add_member_email;
EXECUTE migration_statement;
DEALLOCATE PREPARE migration_statement;

SET @add_member_phone = IF(
    (SELECT COUNT(*) FROM information_schema.columns
     WHERE table_schema = DATABASE() AND table_name = 'members' AND column_name = 'phone_e164') = 0,
    'ALTER TABLE members ADD COLUMN phone_e164 VARCHAR(20) NULL AFTER email',
    'DO 0'
);
PREPARE migration_statement FROM @add_member_phone;
EXECUTE migration_statement;
DEALLOCATE PREPARE migration_statement;

CREATE TABLE IF NOT EXISTS household_notification_settings (
    household_id BIGINT UNSIGNED NOT NULL,
    email_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    smtp_host VARCHAR(255) NOT NULL DEFAULT '',
    smtp_port SMALLINT UNSIGNED NOT NULL DEFAULT 465,
    smtp_security VARCHAR(16) NOT NULL DEFAULT 'TLS',
    smtp_username VARCHAR(254) NOT NULL DEFAULT '',
    smtp_password_ciphertext VARBINARY(1024) NULL,
    smtp_from_email VARCHAR(254) NOT NULL DEFAULT '',
    smtp_from_name VARCHAR(128) NOT NULL DEFAULT 'VHome',
    sms_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    sms_provider VARCHAR(32) NOT NULL DEFAULT 'TENCENT_CLOUD',
    sms_secret_id VARCHAR(255) NOT NULL DEFAULT '',
    sms_secret_key_ciphertext VARBINARY(1024) NULL,
    sms_sdk_app_id VARCHAR(64) NOT NULL DEFAULT '',
    sms_sign_name VARCHAR(128) NOT NULL DEFAULT '',
    sms_template_id VARCHAR(64) NOT NULL DEFAULT '',
    version BIGINT UNSIGNED NOT NULL DEFAULT 1,
    updated_by BIGINT UNSIGNED NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (household_id),
    KEY idx_notification_settings_updated_by (updated_by),
    CONSTRAINT fk_notification_settings_household FOREIGN KEY (household_id)
        REFERENCES households (id) ON UPDATE RESTRICT ON DELETE CASCADE,
    CONSTRAINT fk_notification_settings_updated_by FOREIGN KEY (updated_by)
        REFERENCES members (id) ON UPDATE RESTRICT ON DELETE SET NULL,
    CONSTRAINT chk_notification_settings_smtp_port CHECK (smtp_port BETWEEN 1 AND 65535),
    CONSTRAINT chk_notification_settings_smtp_security CHECK (smtp_security IN ('TLS', 'STARTTLS')),
    CONSTRAINT chk_notification_settings_sms_provider CHECK (sms_provider = 'TENCENT_CLOUD'),
    CONSTRAINT chk_notification_settings_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS memos (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    household_id BIGINT UNSIGNED NOT NULL,
    title VARCHAR(128) NOT NULL,
    description TEXT NULL,
    remind_at DATETIME(6) NOT NULL,
    recipient_ids JSON NOT NULL,
    created_by BIGINT UNSIGNED NOT NULL,
    email_sent_at DATETIME(6) NULL,
    sms_sent_at DATETIME(6) NULL,
    version BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    KEY idx_memos_household_remind_at (household_id, remind_at, id),
    KEY idx_memos_created_by_remind_at (created_by, remind_at, id),
    KEY idx_memos_pending_email (email_sent_at, remind_at, id),
    KEY idx_memos_pending_sms (sms_sent_at, remind_at, id),
    CONSTRAINT fk_memos_household FOREIGN KEY (household_id)
        REFERENCES households (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT fk_memos_created_by FOREIGN KEY (created_by)
        REFERENCES members (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT chk_memos_title CHECK (CHAR_LENGTH(TRIM(title)) > 0),
    CONSTRAINT chk_memos_recipient_ids CHECK (JSON_TYPE(recipient_ids) = 'ARRAY'),
    CONSTRAINT chk_memos_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS calendar_day_overrides (
    calendar_date DATE NOT NULL,
    day_type VARCHAR(24) NOT NULL,
    holiday_name VARCHAR(64) NOT NULL DEFAULT '',
    source_url VARCHAR(512) NOT NULL DEFAULT '',
    synced_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (calendar_date),
    KEY idx_calendar_day_overrides_type (day_type, calendar_date),
    CONSTRAINT chk_calendar_day_overrides_type
        CHECK (day_type IN ('HOLIDAY', 'TRANSFER_WORKDAY'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO calendar_day_overrides (calendar_date, day_type, holiday_name, source_url)
VALUES
    ('2026-01-01', 'HOLIDAY', '元旦', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-01-02', 'HOLIDAY', '元旦', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-01-03', 'HOLIDAY', '元旦', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-01-04', 'TRANSFER_WORKDAY', '', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-02-14', 'TRANSFER_WORKDAY', '', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-02-15', 'HOLIDAY', '春节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-02-16', 'HOLIDAY', '春节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-02-17', 'HOLIDAY', '春节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-02-18', 'HOLIDAY', '春节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-02-19', 'HOLIDAY', '春节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-02-20', 'HOLIDAY', '春节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-02-21', 'HOLIDAY', '春节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-02-22', 'HOLIDAY', '春节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-02-23', 'HOLIDAY', '春节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-02-28', 'TRANSFER_WORKDAY', '', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-04-04', 'HOLIDAY', '清明节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-04-05', 'HOLIDAY', '清明节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-04-06', 'HOLIDAY', '清明节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-05-01', 'HOLIDAY', '劳动节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-05-02', 'HOLIDAY', '劳动节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-05-03', 'HOLIDAY', '劳动节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-05-04', 'HOLIDAY', '劳动节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-05-05', 'HOLIDAY', '劳动节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-05-09', 'TRANSFER_WORKDAY', '', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-06-19', 'HOLIDAY', '端午节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-06-20', 'HOLIDAY', '端午节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-06-21', 'HOLIDAY', '端午节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-09-20', 'TRANSFER_WORKDAY', '', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-09-25', 'HOLIDAY', '中秋节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-09-26', 'HOLIDAY', '中秋节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-09-27', 'HOLIDAY', '中秋节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-10-01', 'HOLIDAY', '国庆节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-10-02', 'HOLIDAY', '国庆节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-10-03', 'HOLIDAY', '国庆节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-10-04', 'HOLIDAY', '国庆节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-10-05', 'HOLIDAY', '国庆节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-10-06', 'HOLIDAY', '国庆节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-10-07', 'HOLIDAY', '国庆节', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm'),
    ('2026-10-10', 'TRANSFER_WORKDAY', '', 'https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm')
ON DUPLICATE KEY UPDATE
    day_type = VALUES(day_type),
    holiday_name = VALUES(holiday_name),
    source_url = VALUES(source_url);
