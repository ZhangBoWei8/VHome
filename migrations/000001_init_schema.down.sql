-- Rolling this back destroys the entire household database. It exists so that
-- `migrate down` is well defined and so a throwaway test database can be reset;
-- never run it against the real deployment.
SET FOREIGN_KEY_CHECKS = 0;

DROP TABLE IF EXISTS material_reminder_reads;
DROP TABLE IF EXISTS inventory_items;
DROP TABLE IF EXISTS material_templates;
DROP TABLE IF EXISTS storage_locations;
DROP TABLE IF EXISTS meal_records;
DROP TABLE IF EXISTS foods;
DROP TABLE IF EXISTS expense_records;
DROP TABLE IF EXISTS expense_categories;
DROP TABLE IF EXISTS calendar_day_overrides;
DROP TABLE IF EXISTS memos;
DROP TABLE IF EXISTS household_notification_settings;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS members;
DROP TABLE IF EXISTS households;

SET FOREIGN_KEY_CHECKS = 1;
