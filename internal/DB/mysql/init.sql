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
