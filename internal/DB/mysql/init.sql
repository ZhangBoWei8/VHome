SET NAMES utf8mb4;
SET time_zone = '+00:00';

CREATE TABLE IF NOT EXISTS households (
    id                    BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    singleton_key         TINYINT UNSIGNED NOT NULL DEFAULT 1,
    login_name            VARCHAR(64) NOT NULL,
    display_name          VARCHAR(64) NOT NULL,
    join_secret_hash      VARCHAR(255) NOT NULL,
    registration_enabled  BOOLEAN NOT NULL DEFAULT TRUE,
    avatar_type           VARCHAR(16) NOT NULL DEFAULT 'BUILTIN',
    avatar_value          VARCHAR(64) NOT NULL DEFAULT 'house',
    province              VARCHAR(64) NOT NULL DEFAULT '',
    city                  VARCHAR(64) NOT NULL DEFAULT '',
    version               BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at            DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at            DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
                            ON UPDATE CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),
    UNIQUE KEY uk_households_singleton (singleton_key),
    UNIQUE KEY uk_households_login_name (login_name),

    CONSTRAINT chk_households_singleton
        CHECK (singleton_key = 1),
    CONSTRAINT chk_households_avatar_type
        CHECK (avatar_type = 'BUILTIN')
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;


CREATE TABLE IF NOT EXISTS members (
    id                   BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    household_id         BIGINT UNSIGNED NOT NULL,
    username             VARCHAR(64) NOT NULL,
    username_normalized  VARCHAR(64) NOT NULL,
    display_name         VARCHAR(64) NOT NULL,
    password_hash        VARCHAR(255) NOT NULL,
    role                 VARCHAR(16) NOT NULL DEFAULT 'MEMBER',
    status               VARCHAR(16) NOT NULL DEFAULT 'PENDING',
    reviewed_by         BIGINT UNSIGNED NULL,
    reviewed_at          DATETIME(6) NULL,
    last_login_at        DATETIME(6) NULL,
    presence_status      VARCHAR(16) NULL,
    avatar_key           VARCHAR(32) NOT NULL DEFAULT 'initials',
    version              BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at           DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at           DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
                           ON UPDATE CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),
    UNIQUE KEY uk_members_household_username (
        household_id,
        username_normalized
    ),
    KEY idx_members_household_status (household_id, status),
    KEY idx_members_household_role (household_id, role),
    KEY idx_members_reviewed_by (reviewed_by),

    CONSTRAINT fk_members_household
        FOREIGN KEY (household_id)
        REFERENCES households (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_members_reviewed_by
        FOREIGN KEY (reviewed_by)
        REFERENCES members (id)
        ON UPDATE RESTRICT
        ON DELETE SET NULL,

    CONSTRAINT chk_members_role
        CHECK (role IN ('OWNER', 'ADMIN', 'MEMBER')),

    CONSTRAINT chk_members_status
        CHECK (status IN ('PENDING', 'ACTIVE', 'REJECTED', 'DISABLED')),

    CONSTRAINT chk_members_presence
        CHECK (presence_status IS NULL OR presence_status IN (
            'HOME', 'SCHOOL', 'WORKING', 'OUT',
            'NAPPING', 'RESTING', 'SICK', 'STUDYING'
        )),

    CONSTRAINT chk_members_avatar
        CHECK (avatar_key IN (
            'initials', 'man', 'woman',
            'boy', 'girl', 'dog'
        ))
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;


CREATE TABLE IF NOT EXISTS sessions (
    id               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    household_id     BIGINT UNSIGNED NOT NULL,
    member_id        BIGINT UNSIGNED NOT NULL,
    token_hash       BINARY(32) NOT NULL,
    csrf_token_hash  BINARY(32) NOT NULL,
    expires_at       DATETIME(6) NOT NULL,
    last_seen_at     DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    created_ip       VARBINARY(16) NULL,
    user_agent       VARCHAR(512) NULL,
    revoked_at       DATETIME(6) NULL,
    revoke_reason    VARCHAR(64) NULL,
    created_at       DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),
    UNIQUE KEY uk_sessions_token_hash (token_hash),
    KEY idx_sessions_member_active (
        member_id,
        revoked_at,
        expires_at
    ),
    KEY idx_sessions_household (household_id),

    CONSTRAINT fk_sessions_household
        FOREIGN KEY (household_id)
        REFERENCES households (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_sessions_member
        FOREIGN KEY (member_id)
        REFERENCES members (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;

-- Stable expense categories. V1 exposes these as read-only reference data so
-- historical records always retain a valid, uniquely named classification.
CREATE TABLE IF NOT EXISTS expense_categories (
    id          SMALLINT UNSIGNED NOT NULL AUTO_INCREMENT,
    code        VARCHAR(32) NOT NULL,
    name        VARCHAR(32) NOT NULL,
    icon_key    VARCHAR(64) NOT NULL,
    sort_order  SMALLINT UNSIGNED NOT NULL DEFAULT 0,
    is_builtin  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),
    UNIQUE KEY uk_expense_categories_code (code),
    UNIQUE KEY uk_expense_categories_name (name)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;

-- Every month shares one indexed ledger. Monetary values use integer cents to
-- avoid floating-point rounding, while deleted rows remain available for
-- audit and a future recycle-bin workflow.
CREATE TABLE IF NOT EXISTS expense_records (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    household_id    BIGINT UNSIGNED NOT NULL,
    member_id       BIGINT UNSIGNED NOT NULL,
    category_id     SMALLINT UNSIGNED NOT NULL,
    expense_scope   VARCHAR(16) NOT NULL,
    title           VARCHAR(128) NOT NULL,
    amount_cents    BIGINT UNSIGNED NOT NULL,
    spent_on        DATE NOT NULL,
    note            VARCHAR(500) NULL,
    version         BIGINT UNSIGNED NOT NULL DEFAULT 1,
    deleted_at      DATETIME(6) NULL,
    deleted_by      BIGINT UNSIGNED NULL,
    created_at      DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at      DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
                        ON UPDATE CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),
    KEY idx_expense_records_member_date (
        member_id,
        deleted_at,
        spent_on,
        id
    ),
    KEY idx_expense_records_household_date (
        household_id,
        deleted_at,
        spent_on,
        id
    ),

    CONSTRAINT fk_expense_records_household
        FOREIGN KEY (household_id)
        REFERENCES households (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_expense_records_member
        FOREIGN KEY (member_id)
        REFERENCES members (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_expense_records_category
        FOREIGN KEY (category_id)
        REFERENCES expense_categories (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_expense_records_deleted_by
        FOREIGN KEY (deleted_by)
        REFERENCES members (id)
        ON UPDATE RESTRICT
        ON DELETE SET NULL,

    CONSTRAINT chk_expense_records_scope
        CHECK (expense_scope IN ('PERSONAL', 'COLLECTIVE')),
    CONSTRAINT chk_expense_records_amount
        CHECK (amount_cents > 0),
    CONSTRAINT chk_expense_records_title
        CHECK (CHAR_LENGTH(TRIM(title)) > 0)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;

-- 食品营养主数据。foods 负责“这个食品每 100g 含有什么”，不保存某天吃了多少。
-- 名称使用数据库的 utf8mb4_unicode_ci 排序规则，因此唯一约束同时避免大小写变体重复。
CREATE TABLE IF NOT EXISTS foods (
    id                          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    name                        VARCHAR(128) NOT NULL,
    calories_per_100g           DECIMAL(10,2) NOT NULL,
    carbohydrate_per_100g       DECIMAL(10,2) NULL,
    protein_per_100g            DECIMAL(10,2) NULL,
    fat_per_100g                DECIMAL(10,2) NULL,
    icon_type                   VARCHAR(16) NOT NULL DEFAULT 'BUILTIN',
    icon_value                  VARCHAR(255) NOT NULL DEFAULT 'generic-food',
    source                      VARCHAR(16) NOT NULL DEFAULT 'USER',
    created_by                  BIGINT UNSIGNED NULL,
    deleted_by                  BIGINT UNSIGNED NULL,
    version                     BIGINT UNSIGNED NOT NULL DEFAULT 1,
    deleted_at                  DATETIME(6) NULL,
    created_at                  DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at                  DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
                                    ON UPDATE CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),
    UNIQUE KEY uk_foods_name (name),
    KEY idx_foods_active_name (deleted_at, name),
    KEY idx_foods_created_by (created_by),
    KEY idx_foods_deleted_by (deleted_by),

    CONSTRAINT fk_foods_created_by
        FOREIGN KEY (created_by)
        REFERENCES members (id)
        ON UPDATE RESTRICT
        ON DELETE SET NULL,

    CONSTRAINT fk_foods_deleted_by
        FOREIGN KEY (deleted_by)
        REFERENCES members (id)
        ON UPDATE RESTRICT
        ON DELETE SET NULL,

    CONSTRAINT chk_foods_calories
        CHECK (calories_per_100g BETWEEN 0 AND 1000),
    CONSTRAINT chk_foods_carbohydrate
        CHECK (carbohydrate_per_100g IS NULL OR carbohydrate_per_100g BETWEEN 0 AND 100),
    CONSTRAINT chk_foods_protein
        CHECK (protein_per_100g IS NULL OR protein_per_100g BETWEEN 0 AND 100),
    CONSTRAINT chk_foods_fat
        CHECK (fat_per_100g IS NULL OR fat_per_100g BETWEEN 0 AND 100),
    CONSTRAINT chk_foods_icon_type
        CHECK (icon_type IN ('BUILTIN', 'UPLOAD')),
    CONSTRAINT chk_foods_source
        CHECK (source IN ('BUILTIN', 'USER'))
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;

-- 饮食记录保存食品快照，而不在读取时重新关联 foods 计算营养。
-- 因此修改或逻辑删除食品后，过去的饮食统计仍保持记录创建时的结果。
CREATE TABLE IF NOT EXISTS meal_records (
    id                              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    member_id                       BIGINT UNSIGNED NOT NULL,
    meal_date                       DATE NOT NULL,
    meal_type                       VARCHAR(16) NOT NULL,
    food_id                         BIGINT UNSIGNED NULL,
    food_name_snapshot              VARCHAR(128) NOT NULL,
    icon_type_snapshot              VARCHAR(16) NOT NULL,
    icon_value_snapshot             VARCHAR(255) NOT NULL,
    weight_grams                    DECIMAL(12,2) NOT NULL,
    calories_per_100g_snapshot      DECIMAL(10,2) NOT NULL,
    carbohydrate_per_100g_snapshot  DECIMAL(10,2) NULL,
    protein_per_100g_snapshot       DECIMAL(10,2) NULL,
    fat_per_100g_snapshot           DECIMAL(10,2) NULL,
    created_by                      BIGINT UNSIGNED NOT NULL,
    deleted_by                      BIGINT UNSIGNED NULL,
    version                         BIGINT UNSIGNED NOT NULL DEFAULT 1,
    deleted_at                      DATETIME(6) NULL,
    created_at                      DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at                      DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
                                        ON UPDATE CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),
    KEY idx_meal_records_member_date (member_id, meal_date, deleted_at),
    KEY idx_meal_records_food (food_id),
    KEY idx_meal_records_created_by (created_by),
    KEY idx_meal_records_deleted_by (deleted_by),

    CONSTRAINT fk_meal_records_member
        FOREIGN KEY (member_id)
        REFERENCES members (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_meal_records_food
        FOREIGN KEY (food_id)
        REFERENCES foods (id)
        ON UPDATE RESTRICT
        ON DELETE SET NULL,

    CONSTRAINT fk_meal_records_created_by
        FOREIGN KEY (created_by)
        REFERENCES members (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_meal_records_deleted_by
        FOREIGN KEY (deleted_by)
        REFERENCES members (id)
        ON UPDATE RESTRICT
        ON DELETE SET NULL,

    CONSTRAINT chk_meal_records_type
        CHECK (meal_type IN ('BREAKFAST', 'LUNCH', 'DINNER', 'SNACK')),
    CONSTRAINT chk_meal_records_icon_type
        CHECK (icon_type_snapshot IN ('BUILTIN', 'UPLOAD')),
    CONSTRAINT chk_meal_records_weight
        CHECK (weight_grams > 0 AND weight_grams <= 100000),
    CONSTRAINT chk_meal_records_calories
        CHECK (calories_per_100g_snapshot BETWEEN 0 AND 1000),
    CONSTRAINT chk_meal_records_carbohydrate
        CHECK (carbohydrate_per_100g_snapshot IS NULL OR carbohydrate_per_100g_snapshot BETWEEN 0 AND 100),
    CONSTRAINT chk_meal_records_protein
        CHECK (protein_per_100g_snapshot IS NULL OR protein_per_100g_snapshot BETWEEN 0 AND 100),
    CONSTRAINT chk_meal_records_fat
        CHECK (fat_per_100g_snapshot IS NULL OR fat_per_100g_snapshot BETWEEN 0 AND 100)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS storage_locations (
    id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    name         VARCHAR(64) NOT NULL,
    icon_key     VARCHAR(64) NOT NULL DEFAULT 'generic-storage',
    storage_type VARCHAR(16) NOT NULL,
    is_builtin   BOOLEAN NOT NULL DEFAULT FALSE,
    enabled      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uk_storage_locations_name (name),
    CONSTRAINT chk_storage_locations_type CHECK (storage_type IN ('COLD', 'AMBIENT'))
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS material_templates (
    id                          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    name                        VARCHAR(64) NOT NULL,
    icon_key                    VARCHAR(64) NOT NULL DEFAULT 'generic-food',
    default_unit                VARCHAR(16) NOT NULL DEFAULT 'G',
    cold_shelf_life_days        SMALLINT UNSIGNED NULL,
    ambient_shelf_life_days     SMALLINT UNSIGNED NULL,
    calories_per_100g           DECIMAL(10,2) NULL,
    protein_per_100g            DECIMAL(10,2) NULL,
    fat_per_100g                DECIMAL(10,2) NULL,
    carbohydrate_per_100g       DECIMAL(10,2) NULL,
    source                      VARCHAR(16) NOT NULL DEFAULT 'USER',
    enabled                     BOOLEAN NOT NULL DEFAULT TRUE,
    version                     BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at                  DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at                  DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uk_material_templates_name (name),
    CONSTRAINT chk_material_templates_unit CHECK (default_unit IN ('G','PIECE','PACK','BOX','BOTTLE','CAN')),
    CONSTRAINT chk_material_templates_source CHECK (source IN ('BUILTIN','USER')),
    CONSTRAINT chk_material_templates_storage CHECK (cold_shelf_life_days IS NOT NULL OR ambient_shelf_life_days IS NOT NULL),
    CONSTRAINT chk_material_templates_cold_days CHECK (cold_shelf_life_days IS NULL OR cold_shelf_life_days BETWEEN 1 AND 3650),
    CONSTRAINT chk_material_templates_ambient_days CHECK (ambient_shelf_life_days IS NULL OR ambient_shelf_life_days BETWEEN 1 AND 3650)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS inventory_items (
    id                    BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    template_id           BIGINT UNSIGNED NULL,
    storage_location_id   BIGINT UNSIGNED NOT NULL,
    name                  VARCHAR(128) NOT NULL,
    icon_key              VARCHAR(64) NOT NULL DEFAULT 'generic-food',
    quantity              DECIMAL(12,2) NOT NULL,
    unit                  VARCHAR(16) NOT NULL,
    stocked_on            DATE NOT NULL,
    expires_on            DATE NOT NULL,
    calories_per_100g     DECIMAL(10,2) NULL,
    protein_per_100g      DECIMAL(10,2) NULL,
    fat_per_100g          DECIMAL(10,2) NULL,
    carbohydrate_per_100g DECIMAL(10,2) NULL,
    description           TEXT NULL,
    image_path            VARCHAR(512) NULL,
    status                VARCHAR(16) NOT NULL DEFAULT 'ACTIVE',
    discarded_at          DATETIME(6) NULL,
    discarded_by          BIGINT UNSIGNED NULL,
    discard_reason        VARCHAR(255) NULL,
    version               BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_by            BIGINT UNSIGNED NOT NULL,
    created_at            DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at            DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    KEY idx_inventory_items_active_expiry (status, expires_on),
    KEY idx_inventory_items_location (storage_location_id),
    CONSTRAINT fk_inventory_template FOREIGN KEY (template_id) REFERENCES material_templates(id) ON DELETE SET NULL,
    CONSTRAINT fk_inventory_location FOREIGN KEY (storage_location_id) REFERENCES storage_locations(id) ON DELETE RESTRICT,
    CONSTRAINT fk_inventory_created_by FOREIGN KEY (created_by) REFERENCES members(id) ON DELETE RESTRICT,
    CONSTRAINT fk_inventory_discarded_by FOREIGN KEY (discarded_by) REFERENCES members(id) ON DELETE SET NULL,
    CONSTRAINT chk_inventory_quantity CHECK (quantity > 0),
    CONSTRAINT chk_inventory_unit CHECK (unit IN ('G','PIECE','PACK','BOX','BOTTLE','CAN')),
    CONSTRAINT chk_inventory_status CHECK (status IN ('ACTIVE','DISCARDED')),
    CONSTRAINT chk_inventory_dates CHECK (expires_on >= stocked_on)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS material_reminder_reads (
    member_id    BIGINT UNSIGNED NOT NULL,
    item_id      BIGINT UNSIGNED NOT NULL,
    milestone    VARCHAR(24) NOT NULL,
    read_at      DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (member_id, item_id, milestone),
    CONSTRAINT fk_reminder_reads_member FOREIGN KEY (member_id) REFERENCES members(id) ON DELETE CASCADE,
    CONSTRAINT fk_reminder_reads_item FOREIGN KEY (item_id) REFERENCES inventory_items(id) ON DELETE CASCADE,
    CONSTRAINT chk_reminder_milestone CHECK (milestone IN ('HALF','THREE_QUARTERS','LAST_DAY'))
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

INSERT IGNORE INTO storage_locations (id, name, icon_key, storage_type, is_builtin)
VALUES (1, '冰箱', 'fridge', 'COLD', TRUE),
       (2, '仓库', 'pantry', 'AMBIENT', TRUE);

INSERT IGNORE INTO expense_categories
    (id, code, name, icon_key, sort_order, is_builtin)
VALUES
    (1, 'LIVING', '生活开销', 'household', 10, TRUE),
    (2, 'ENTERTAINMENT_SHOPPING', '娱乐/购物消费', 'shopping', 20, TRUE),
    (3, 'MEDICAL', '医疗支出', 'medical', 30, TRUE),
    (4, 'TRANSPORTATION', '交通出行', 'transportation', 40, TRUE),
    (5, 'EDUCATION', '教育支出', 'education', 50, TRUE),
    (6, 'OTHER', '其他开销', 'other', 60, TRUE);

INSERT IGNORE INTO material_templates
    (id, name, icon_key, default_unit, cold_shelf_life_days, ambient_shelf_life_days,
     calories_per_100g, protein_per_100g, fat_per_100g, carbohydrate_per_100g, source)
VALUES
    (1, '牛奶', 'milk', 'BOTTLE', 14, NULL, 65, 3.3, 3.6, 4.9, 'BUILTIN'),
    (2, '鸡蛋', 'egg', 'PIECE', 30, 14, 144, 13.3, 8.8, 2.8, 'BUILTIN'),
    (3, '油菜', 'leafy-vegetable', 'G', 7, 2, 18, 1.8, 0.5, 2.7, 'BUILTIN'),
    (4, '土豆', 'potato', 'G', 20, 30, 81, 2.6, 0.2, 17.8, 'BUILTIN'),
    (5, '大葱', 'scallion', 'G', 7, 3, 27, 1.6, 0.4, 4.9, 'BUILTIN'),
    (6, '猪肉', 'pork', 'G', 3, NULL, 242, 27.3, 13.9, 0, 'BUILTIN'),
    (7, '牛肉', 'beef', 'G', 3, NULL, 250, 26, 15, 0, 'BUILTIN'),
    (8, '鸡肉', 'chicken', 'G', 3, NULL, 165, 31, 3.6, 0, 'BUILTIN'),
    (9, '薯片', 'chips', 'PACK', NULL, 180, 536, 7, 35, 53, 'BUILTIN'),
    (10, '水果', 'fruit', 'G', 7, 3, 52, 0.5, 0.2, 14, 'BUILTIN');

-- 初始食品与内置物料使用同一组营养数据，但两张表没有强外键关系：
-- 物料品类可以独立调整保存周期，饮食食品也可以独立编辑营养信息。
INSERT IGNORE INTO foods
    (name, calories_per_100g, carbohydrate_per_100g, protein_per_100g,
     fat_per_100g, icon_type, icon_value, source)
VALUES
    ('牛奶', 65, 4.9, 3.3, 3.6, 'BUILTIN', 'milk', 'BUILTIN'),
    ('鸡蛋', 144, 2.8, 13.3, 8.8, 'BUILTIN', 'egg', 'BUILTIN'),
    ('油菜', 18, 2.7, 1.8, 0.5, 'BUILTIN', 'leafy-vegetable', 'BUILTIN'),
    ('土豆', 81, 17.8, 2.6, 0.2, 'BUILTIN', 'potato', 'BUILTIN'),
    ('大葱', 27, 4.9, 1.6, 0.4, 'BUILTIN', 'scallion', 'BUILTIN'),
    ('猪肉', 242, 0, 27.3, 13.9, 'BUILTIN', 'pork', 'BUILTIN'),
    ('牛肉', 250, 0, 26, 15, 'BUILTIN', 'beef', 'BUILTIN'),
    ('鸡肉', 165, 0, 31, 3.6, 'BUILTIN', 'chicken', 'BUILTIN'),
    ('薯片', 536, 53, 7, 35, 'BUILTIN', 'chips', 'BUILTIN'),
    ('水果', 52, 14, 0.5, 0.2, 'BUILTIN', 'fruit', 'BUILTIN');

-- Backfill reusable materials created before food synchronization existed.
-- A food name already present (including a logically deleted row) is kept as
-- is, and templates without calories remain pantry-only records.
INSERT IGNORE INTO foods (
    name,
    calories_per_100g,
    carbohydrate_per_100g,
    protein_per_100g,
    fat_per_100g,
    icon_type,
    icon_value,
    source
)
SELECT
    template.name,
    template.calories_per_100g,
    template.carbohydrate_per_100g,
    template.protein_per_100g,
    template.fat_per_100g,
    'BUILTIN',
    template.icon_key,
    'USER'
FROM material_templates AS template
WHERE template.enabled = TRUE
  AND template.calories_per_100g IS NOT NULL;
