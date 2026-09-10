-- Reverses 000002. Dropping these tables deletes every memo, holiday override
-- and notification setting, so only use it on a throwaway database.
SET FOREIGN_KEY_CHECKS = 0;

DROP TABLE IF EXISTS calendar_day_overrides;
DROP TABLE IF EXISTS memos;
DROP TABLE IF EXISTS household_notification_settings;

ALTER TABLE members DROP COLUMN IF EXISTS phone_e164;
ALTER TABLE members DROP COLUMN IF EXISTS email;

SET FOREIGN_KEY_CHECKS = 1;
