# VHome API v1

> 状态：第一阶段开发契约  
> 版本：0.1.0  
> Base URL：`/api/v1`  
> 数据格式：`application/json; charset=utf-8`

本文档定义 VHome 第一阶段前后端接口。当前阶段只覆盖：

- 系统首次初始化
- 登录与会话
- 唯一家庭资料
- 家庭成员与角色
- 存储位置
- 物料种类与营养信息
- 库存批次与库存流水
- Dashboard 聚合数据
- 未登录访客的只读首页与物料仓库

饮食记录、Memo、通知、知识库、摄像头、智能家居和 Agent 不包含在本版本中。

---

## 1. 核心业务约束

### 1.1 单家庭约束

VHome 当前部署只服务一个家庭。

- 数据库中只能存在一个有效家庭。
- 只有系统从未初始化时，才能调用家庭创建接口。
- 家庭创建成功后，不再提供第二次创建入口。
- 后续只能通过 `PATCH /household` 修改家庭资料。
- 不提供删除家庭接口。

### 1.2 正确的首次启动流程

首次启动时还没有 OWNER，因此不能要求用户先登录再创建家庭。

```text
前端启动
  ↓
GET /bootstrap
  ├─ initialized=false → /setup
  │                       ↓
  │                   POST /setup
  │                       ↓
  │                   创建家庭 + OWNER + Session
  │                       ↓
  │                     /app
  │
  └─ initialized=true
          ├─ 已登录 → /app
          └─ 未登录 → /login
```

`POST /setup` 必须在一个数据库事务中完成：

1. 锁定系统初始化状态。
2. 再次确认系统尚未初始化。
3. 创建家庭。
4. 创建唯一的首位 OWNER。
5. 写入初始化完成标记。
6. 创建登录会话。
7. 提交事务。

如果并发请求中另一个请求已经完成初始化，返回：

```http
409 Conflict
```

错误码：

```text
SYSTEM_ALREADY_INITIALIZED
```

### 1.3 成员角色

| 角色 | 含义 | 主要权限 |
|---|---|---|
| `OWNER` | 家庭所有者 | 所有权限、转让所有权、管理管理员 |
| `ADMIN` | 家庭主要成员 | 管理普通成员、物料、库存、位置 |
| `MEMBER` | 家庭成员 | 查看家庭信息、使用和维护库存 |

未登录访客不是数据库成员角色，API 中称为 `ANONYMOUS`，只允许访问 `/public/*`。

系统必须保证：

- 至少存在一个有效 OWNER。
- 最后一个 OWNER 不能被删除、停用或降级。
- OWNER 不能直接删除自己。
- 所有权应通过专门的转让接口完成。

---

## 2. 通用规范

### 2.1 时间

- 所有时间字段使用 RFC 3339。
- 服务端统一存储 UTC。
- API 返回带时区的 UTC，例如：

```text
2026-07-28T03:30:00Z
```

- 家庭时区单独保存在 `household.timezone`，默认 `Asia/Shanghai`。
- 只包含日期的字段使用 `YYYY-MM-DD`。

### 2.2 ID

第一阶段允许使用 MySQL `BIGINT UNSIGNED AUTO_INCREMENT`。

API 中 ID 使用 JSON 整数：

```json
{
  "id": 1024
}
```

不要依赖 ID 实现权限控制。所有查询都必须经过当前家庭与当前用户权限校验。

### 2.3 金额、重量和营养数值

- 数据库存储使用 `DECIMAL`，不要使用 `FLOAT`。
- API 使用 JSON number。
- 热量单位为 `kcal`。
- 三大营养物质单位为 `g/100g`。
- 重量优先规范化为克，体积优先规范化为毫升。

### 2.4 分页

请求参数：

| 参数 | 默认值 | 限制 |
|---|---:|---:|
| `page` | 1 | 最小 1 |
| `page_size` | 20 | 1～100 |

分页响应：

```json
{
  "data": [],
  "meta": {
    "page": 1,
    "page_size": 20,
    "total": 0,
    "total_pages": 0
  },
  "request_id": "req_01J..."
}
```

### 2.5 成功响应

单个资源：

```json
{
  "data": {},
  "request_id": "req_01J..."
}
```

资源列表：

```json
{
  "data": [],
  "meta": {
    "page": 1,
    "page_size": 20,
    "total": 35,
    "total_pages": 2
  },
  "request_id": "req_01J..."
}
```

删除或退出成功允许返回：

```http
204 No Content
```

### 2.6 错误响应

```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "请求参数不合法",
    "details": [
      {
        "field": "email",
        "reason": "INVALID_EMAIL"
      }
    ]
  },
  "request_id": "req_01J..."
}
```

通用状态码：

| HTTP 状态码 | 含义 |
|---:|---|
| `400` | 请求格式错误 |
| `401` | 未登录或 Session 失效 |
| `403` | 已登录但没有权限 |
| `404` | 资源不存在 |
| `409` | 当前状态冲突 |
| `422` | 字段验证失败 |
| `429` | 请求过于频繁 |
| `500` | 未预期的服务端错误 |

通用错误码：

```text
BAD_REQUEST
VALIDATION_FAILED
UNAUTHORIZED
SESSION_EXPIRED
FORBIDDEN
NOT_FOUND
CONFLICT
VERSION_CONFLICT
RATE_LIMITED
INTERNAL_ERROR
```

### 2.7 乐观锁

可编辑资源包含：

```json
{
  "version": 3
}
```

更新时客户端必须提交当前版本：

```json
{
  "name": "新的名称",
  "version": 3
}
```

版本不一致时返回：

```http
409 Conflict
```

```json
{
  "error": {
    "code": "VERSION_CONFLICT",
    "message": "资源已被其他成员修改，请刷新后重试"
  }
}
```

### 2.8 幂等性

以下操作建议支持：

```http
Idempotency-Key: 0191f47a-...
```

- 首次初始化
- 入库
- 消耗
- 丢弃
- 调整库存
- 移动库存

相同用户、相同接口、相同 `Idempotency-Key` 重复提交时，不得重复扣减或增加库存。

---

## 3. 认证与安全

### 3.1 Session Cookie

推荐使用服务端 Session，不在浏览器 Local Storage 中保存访问令牌。

Cookie 示例：

```http
Set-Cookie: vhome_session=<opaque-token>; Path=/; HttpOnly; Secure; SameSite=Lax
```

本地开发允许关闭 `Secure`，生产环境必须开启。

### 3.2 CSRF

所有修改数据的请求都应校验 CSRF Token：

```http
X-CSRF-Token: <token>
```

Token 可以由登录、首次初始化和 Session 查询接口返回。

### 3.3 密码

- 密码不允许明文存储。
- 推荐使用 Argon2id；也可以使用配置合理的 bcrypt。
- 第一阶段密码长度至少 10 个字符。
- 登录错误不能暴露“邮箱存在但密码错误”这类细节。

### 3.4 权限校验

前端隐藏按钮不构成权限控制。

服务端每个受保护接口都必须校验：

1. Session 是否有效。
2. 成员是否为 `ACTIVE`。
3. 成员是否属于当前唯一家庭。
4. 当前角色是否有对应权限。
5. 目标资源是否属于当前家庭。

---

## 4. 数据枚举

### 4.1 成员

```text
MemberRole:
  OWNER
  ADMIN
  MEMBER

MemberStatus:
  ACTIVE
  DISABLED
```

### 4.2 图标

```text
IconType:
  EMOJI
  BUILTIN
  UPLOAD
```

第一阶段只要求实现 `EMOJI` 和 `BUILTIN`。

### 4.3 存储位置

```text
StorageLocationKind:
  REFRIGERATED
  FROZEN
  CABINET
  CUSTOM
```

### 4.4 物料

```text
MaterialStatus:
  ACTIVE
  ARCHIVED

MaterialUnit:
  G
  KG
  ML
  L
  PIECE
  BOX
  BOTTLE
  CAN

ExpiryPolicy:
  ESTIMATED_DURATION
  LABELED_DATE
  NONE
```

含义：

- `ESTIMATED_DURATION`：按入库时间加默认保存天数计算。
- `LABELED_DATE`：每个库存批次必须录入包装日期。
- `NONE`：不管理期限。

### 4.5 库存

```text
InventoryBatchStatus:
  ACTIVE
  DEPLETED
  DISCARDED

ExpiryStatus:
  NONE
  NORMAL
  EXPIRING_SOON
  EXPIRING_URGENT
  EXPIRED

InventoryTransactionType:
  STOCK_IN
  CONSUME
  DISCARD
  ADJUST
  MOVE
```

建议的期限状态规则：

| 状态 | 条件 |
|---|---|
| `NONE` | `expires_at = null` |
| `EXPIRED` | 已经过期 |
| `EXPIRING_URGENT` | 剩余不超过 1 天 |
| `EXPIRING_SOON` | 剩余 2～3 天 |
| `NORMAL` | 剩余超过 3 天 |

---

## 5. 接口总览

### 5.1 系统与认证

| 方法 | 路径 | 登录 | 权限 |
|---|---|---|---|
| `GET` | `/bootstrap` | 否 | 公共 |
| `POST` | `/setup` | 否 | 仅未初始化 |
| `POST` | `/auth/sessions` | 否 | 公共 |
| `GET` | `/auth/session` | 是 | 任意有效成员 |
| `DELETE` | `/auth/session` | 是 | 任意有效成员 |
| `POST` | `/auth/password` | 是 | 当前成员 |

### 5.2 家庭与成员

| 方法 | 路径 | 权限 |
|---|---|---|
| `GET` | `/household` | 任意成员 |
| `PATCH` | `/household` | OWNER / ADMIN |
| `GET` | `/members` | 任意成员 |
| `POST` | `/members` | OWNER / ADMIN |
| `GET` | `/members/{id}` | 任意成员 |
| `PATCH` | `/members/{id}` | OWNER / ADMIN，或本人有限字段 |
| `DELETE` | `/members/{id}` | OWNER / ADMIN |
| `POST` | `/members/{id}/ownership-transfer` | OWNER |

### 5.3 存储位置与物料

| 方法 | 路径 | 权限 |
|---|---|---|
| `GET` | `/storage-locations` | 任意成员 |
| `POST` | `/storage-locations` | OWNER / ADMIN |
| `GET` | `/storage-locations/{id}` | 任意成员 |
| `PATCH` | `/storage-locations/{id}` | OWNER / ADMIN |
| `DELETE` | `/storage-locations/{id}` | OWNER / ADMIN |
| `GET` | `/materials` | 任意成员 |
| `POST` | `/materials` | OWNER / ADMIN |
| `GET` | `/materials/{id}` | 任意成员 |
| `PATCH` | `/materials/{id}` | OWNER / ADMIN |
| `DELETE` | `/materials/{id}` | OWNER / ADMIN |

### 5.4 库存

| 方法 | 路径 | 权限 |
|---|---|---|
| `GET` | `/inventory/summary` | 任意成员 |
| `GET` | `/inventory/batches` | 任意成员 |
| `POST` | `/inventory/batches` | OWNER / ADMIN / MEMBER |
| `GET` | `/inventory/batches/{id}` | 任意成员 |
| `PATCH` | `/inventory/batches/{id}` | OWNER / ADMIN / MEMBER |
| `POST` | `/inventory/batches/{id}/transactions` | OWNER / ADMIN / MEMBER |
| `GET` | `/inventory/transactions` | 任意成员 |

### 5.5 首页与公共只读

| 方法 | 路径 | 登录 |
|---|---|---|
| `GET` | `/dashboard/overview` | 是 |
| `GET` | `/public/dashboard` | 否 |
| `GET` | `/public/inventory` | 否 |

---

## 6. 系统初始化

### 6.1 查询启动状态

```http
GET /api/v1/bootstrap
```

无需登录。

已初始化：

```json
{
  "data": {
    "initialized": true,
    "authenticated": false,
    "next_route": "LOGIN",
    "api_version": "v1"
  },
  "request_id": "req_01J..."
}
```

已初始化且当前 Session 有效：

```json
{
  "data": {
    "initialized": true,
    "authenticated": true,
    "next_route": "DASHBOARD",
    "api_version": "v1"
  },
  "request_id": "req_01J..."
}
```

尚未初始化：

```json
{
  "data": {
    "initialized": false,
    "authenticated": false,
    "next_route": "SETUP",
    "api_version": "v1"
  },
  "request_id": "req_01J..."
}
```

此接口不能泄露 OWNER 邮箱、成员数量等敏感数据。

### 6.2 首次创建家庭和 OWNER

```http
POST /api/v1/setup
Idempotency-Key: 0191f47a-...
```

仅当 `initialized=false` 时允许。

请求：

```json
{
  "household": {
    "name": "橡木小屋",
    "timezone": "Asia/Shanghai",
    "icon": {
      "type": "EMOJI",
      "value": "🏡"
    }
  },
  "owner": {
    "name": "林墨",
    "email": "owner@example.com",
    "password": "a-strong-password"
  }
}
```

响应：

```http
201 Created
Set-Cookie: vhome_session=...; HttpOnly; SameSite=Lax
```

```json
{
  "data": {
    "csrf_token": "csrf_...",
    "household": {
      "id": 1,
      "name": "橡木小屋",
      "timezone": "Asia/Shanghai",
      "icon": {
        "type": "EMOJI",
        "value": "🏡"
      },
      "version": 1
    },
    "member": {
      "id": 1,
      "name": "林墨",
      "email": "owner@example.com",
      "role": "OWNER",
      "status": "ACTIVE"
    },
    "permissions": ["*"]
  },
  "request_id": "req_01J..."
}
```

可能错误：

```text
SYSTEM_ALREADY_INITIALIZED
EMAIL_ALREADY_EXISTS
VALIDATION_FAILED
```

---

## 7. 登录与会话

### 7.1 登录

```http
POST /api/v1/auth/sessions
```

请求：

```json
{
  "email": "owner@example.com",
  "password": "a-strong-password",
  "remember_me": true
}
```

响应：

```http
200 OK
Set-Cookie: vhome_session=...; HttpOnly; SameSite=Lax
```

```json
{
  "data": {
    "csrf_token": "csrf_...",
    "member": {
      "id": 1,
      "name": "林墨",
      "email": "owner@example.com",
      "role": "OWNER",
      "status": "ACTIVE",
      "avatar": {
        "type": "EMOJI",
        "value": "🌻"
      }
    },
    "household": {
      "id": 1,
      "name": "橡木小屋",
      "timezone": "Asia/Shanghai",
      "icon": {
        "type": "EMOJI",
        "value": "🏡"
      }
    },
    "permissions": ["*"]
  },
  "request_id": "req_01J..."
}
```

登录失败统一返回：

```http
401 Unauthorized
```

```json
{
  "error": {
    "code": "INVALID_CREDENTIALS",
    "message": "邮箱或密码不正确"
  }
}
```

### 7.2 获取当前 Session

```http
GET /api/v1/auth/session
```

响应结构与登录成功响应相同。

### 7.3 退出

```http
DELETE /api/v1/auth/session
X-CSRF-Token: csrf_...
```

响应：

```http
204 No Content
```

### 7.4 修改当前密码

```http
POST /api/v1/auth/password
X-CSRF-Token: csrf_...
```

```json
{
  "current_password": "old-password",
  "new_password": "new-strong-password"
}
```

成功后服务端应注销其他设备 Session。

---

## 8. 家庭资料

### 8.1 获取家庭资料

```http
GET /api/v1/household
```

```json
{
  "data": {
    "id": 1,
    "name": "橡木小屋",
    "timezone": "Asia/Shanghai",
    "icon": {
      "type": "EMOJI",
      "value": "🏡"
    },
    "public_dashboard_enabled": true,
    "public_inventory_enabled": true,
    "public_inventory_mode": "CATALOG_ONLY",
    "created_at": "2026-07-28T03:30:00Z",
    "updated_at": "2026-07-28T03:30:00Z",
    "version": 1
  },
  "request_id": "req_01J..."
}
```

`public_inventory_mode`：

```text
CATALOG_ONLY   只显示物料种类，不显示数量和位置
SUMMARY        显示是否有库存以及临期数量
FULL_READONLY  显示完整只读库存
```

### 8.2 修改家庭资料

```http
PATCH /api/v1/household
X-CSRF-Token: csrf_...
```

```json
{
  "name": "梧桐小屋",
  "timezone": "Asia/Shanghai",
  "icon": {
    "type": "EMOJI",
    "value": "🌳"
  },
  "public_dashboard_enabled": true,
  "public_inventory_enabled": true,
  "public_inventory_mode": "CATALOG_ONLY",
  "version": 1
}
```

响应返回更新后的完整家庭资源。

不存在：

```text
POST /households
DELETE /household
```

---

## 9. 家庭成员

### 9.1 成员资源

```json
{
  "id": 2,
  "name": "夏禾",
  "email": "xiahe@example.com",
  "role": "ADMIN",
  "status": "ACTIVE",
  "avatar": {
    "type": "EMOJI",
    "value": "🌼"
  },
  "last_login_at": "2026-07-28T01:20:00Z",
  "created_at": "2026-07-20T08:00:00Z",
  "updated_at": "2026-07-28T01:20:00Z",
  "version": 2
}
```

### 9.2 成员列表

```http
GET /api/v1/members?page=1&page_size=20&role=ADMIN&status=ACTIVE
```

支持参数：

```text
search
role
status
page
page_size
```

### 9.3 创建成员

第一阶段暂不实现邮件邀请，OWNER/ADMIN 直接创建成员并设置初始密码。

```http
POST /api/v1/members
X-CSRF-Token: csrf_...
```

```json
{
  "name": "小满",
  "email": "xiaoman@example.com",
  "initial_password": "temporary-password",
  "role": "MEMBER",
  "avatar": {
    "type": "EMOJI",
    "value": "🌱"
  }
}
```

约束：

- ADMIN 只能创建 `MEMBER`。
- 只有 OWNER 能创建另一个 `OWNER` 或 `ADMIN`。
- 邮箱在当前系统内唯一。

### 9.4 获取成员

```http
GET /api/v1/members/{id}
```

### 9.5 修改成员

```http
PATCH /api/v1/members/{id}
X-CSRF-Token: csrf_...
```

OWNER/ADMIN：

```json
{
  "name": "新的昵称",
  "role": "MEMBER",
  "status": "ACTIVE",
  "avatar": {
    "type": "EMOJI",
    "value": "🌿"
  },
  "version": 2
}
```

成员本人只能修改：

- `name`
- `avatar`

本人不能修改：

- `role`
- `status`
- `email`

### 9.6 删除成员

```http
DELETE /api/v1/members/{id}
X-CSRF-Token: csrf_...
```

第一阶段建议软删除或改为 `DISABLED`，保留其历史库存流水。

限制：

```text
LAST_OWNER_PROTECTED
CANNOT_DELETE_SELF
MEMBER_HAS_ACTIVE_SESSION
```

可以先注销该成员全部 Session，再执行停用。

### 9.7 转让家庭所有权

```http
POST /api/v1/members/{target_member_id}/ownership-transfer
X-CSRF-Token: csrf_...
```

```json
{
  "current_password": "owner-password"
}
```

事务行为：

1. 校验当前用户为 OWNER。
2. 校验目标成员为 ACTIVE。
3. 将目标成员升级为 OWNER。
4. 当前 OWNER 默认保留 OWNER 身份。

如果未来需要把当前 OWNER 降级，应在转让成功后单独修改角色。

---

## 10. 存储位置

### 10.1 位置资源

```json
{
  "id": 10,
  "name": "冰箱冷鲜",
  "kind": "REFRIGERATED",
  "parent_id": null,
  "icon": {
    "type": "EMOJI",
    "value": "❄️"
  },
  "sort_order": 10,
  "material_count": 12,
  "active_batch_count": 15,
  "created_at": "2026-07-28T03:30:00Z",
  "updated_at": "2026-07-28T03:30:00Z",
  "version": 1
}
```

### 10.2 位置列表

```http
GET /api/v1/storage-locations?include_counts=true
```

默认按：

```text
sort_order ASC, id ASC
```

### 10.3 创建位置

```http
POST /api/v1/storage-locations
X-CSRF-Token: csrf_...
```

```json
{
  "name": "零食抽屉",
  "kind": "CUSTOM",
  "parent_id": 12,
  "icon": {
    "type": "EMOJI",
    "value": "🍪"
  },
  "sort_order": 40
}
```

### 10.4 获取、修改和删除

```http
GET    /api/v1/storage-locations/{id}
PATCH  /api/v1/storage-locations/{id}
DELETE /api/v1/storage-locations/{id}
```

存在有效库存或子位置时不能直接删除：

```text
LOCATION_NOT_EMPTY
LOCATION_HAS_CHILDREN
```

---

## 11. 物料种类

### 11.1 营养资源

```json
{
  "kcal_per_100g": 144,
  "protein_g_per_100g": 13.3,
  "fat_g_per_100g": 8.8,
  "carbohydrate_g_per_100g": 2.8
}
```

所有值必须大于等于 0。

三大营养物质之和不要求等于 100g。

### 11.2 物料资源

```json
{
  "id": 100,
  "name": "可生食鸡蛋",
  "category": "蛋奶",
  "base_unit": "PIECE",
  "grams_per_unit": 50,
  "expiry_policy": "ESTIMATED_DURATION",
  "default_shelf_life_days": 14,
  "opened_shelf_life_days": null,
  "default_location_id": 10,
  "nutrition": {
    "kcal_per_100g": 144,
    "protein_g_per_100g": 13.3,
    "fat_g_per_100g": 8.8,
    "carbohydrate_g_per_100g": 2.8
  },
  "icon": {
    "type": "EMOJI",
    "value": "🥚"
  },
  "status": "ACTIVE",
  "created_at": "2026-07-28T03:30:00Z",
  "updated_at": "2026-07-28T03:30:00Z",
  "version": 1
}
```

规则：

- `PIECE/BOX/BOTTLE/CAN` 用于营养计算时，建议配置 `grams_per_unit`。
- `ESTIMATED_DURATION` 必须配置 `default_shelf_life_days`。
- `LABELED_DATE` 的默认保存天数可以为空。
- `NONE` 的保存天数必须为空。

### 11.3 物料列表

```http
GET /api/v1/materials?search=鸡蛋&category=蛋奶&status=ACTIVE&page=1&page_size=20
```

支持：

```text
search
category
status
expiry_policy
page
page_size
sort=name|created_at|updated_at
order=asc|desc
```

### 11.4 创建物料

```http
POST /api/v1/materials
X-CSRF-Token: csrf_...
```

请求结构与物料资源相同，但不包含：

- `id`
- `status`
- `created_at`
- `updated_at`
- `version`

### 11.5 获取和修改

```http
GET   /api/v1/materials/{id}
PATCH /api/v1/materials/{id}
```

修改营养信息只影响未来计算。

已经生成的历史饮食记录或营养快照不得被反向修改。

### 11.6 删除物料

```http
DELETE /api/v1/materials/{id}
```

如果物料已有库存或流水，不执行物理删除，而是设置：

```text
status=ARCHIVED
```

归档物料不能继续入库，但历史记录仍可查询。

---

## 12. 库存批次

### 12.1 批次资源

```json
{
  "id": 10001,
  "material": {
    "id": 100,
    "name": "可生食鸡蛋",
    "category": "蛋奶",
    "icon": {
      "type": "EMOJI",
      "value": "🥚"
    }
  },
  "location": {
    "id": 10,
    "name": "冰箱冷鲜",
    "kind": "REFRIGERATED"
  },
  "quantity": 8,
  "unit": "PIECE",
  "stored_at": "2026-07-24T09:00:00Z",
  "production_date": null,
  "expires_at": "2026-08-07T09:00:00Z",
  "opened_at": null,
  "expiry_status": "NORMAL",
  "days_until_expiry": 10,
  "status": "ACTIVE",
  "note": null,
  "created_by": {
    "id": 1,
    "name": "林墨"
  },
  "created_at": "2026-07-24T09:00:00Z",
  "updated_at": "2026-07-28T03:30:00Z",
  "version": 2
}
```

### 12.2 库存汇总

```http
GET /api/v1/inventory/summary
```

```json
{
  "data": {
    "material_count": 23,
    "active_batch_count": 31,
    "expiring_soon_count": 4,
    "expired_count": 1,
    "frozen_material_count": 5,
    "consumed_this_week_count": 12,
    "locations": [
      {
        "location_id": 10,
        "location_name": "冰箱冷鲜",
        "material_count": 12,
        "active_batch_count": 15
      }
    ]
  },
  "request_id": "req_01J..."
}
```

### 12.3 查询库存批次

```http
GET /api/v1/inventory/batches
```

支持：

```text
search
material_id
location_id
category
status
expiry_status
page
page_size
sort=expires_at|stored_at|updated_at|material_name
order=asc|desc
```

推荐默认排序：

```text
已过期 → 紧急临期 → 即将到期 → 正常 → 无期限
同一状态按 expires_at ASC
```

### 12.4 入库

```http
POST /api/v1/inventory/batches
X-CSRF-Token: csrf_...
Idempotency-Key: 0191f47a-...
```

估算期限物料：

```json
{
  "material_id": 100,
  "location_id": 10,
  "quantity": 12,
  "unit": "PIECE",
  "stored_at": "2026-07-28T03:30:00Z",
  "production_date": null,
  "expires_at": null,
  "note": "盒马采购"
}
```

服务端根据：

```text
stored_at + default_shelf_life_days
```

生成 `expires_at`。

包装日期物料：

```json
{
  "material_id": 101,
  "location_id": 10,
  "quantity": 1000,
  "unit": "ML",
  "stored_at": "2026-07-28T03:30:00Z",
  "production_date": "2026-07-26",
  "expires_at": "2026-08-02T15:59:59Z",
  "note": null
}
```

`LABELED_DATE` 物料缺少 `expires_at` 时返回：

```text
EXPIRES_AT_REQUIRED
```

成功时：

- 创建库存批次。
- 创建 `STOCK_IN` 流水。
- 两者在同一事务中提交。

### 12.5 获取和修改批次

```http
GET   /api/v1/inventory/batches/{id}
PATCH /api/v1/inventory/batches/{id}
```

允许直接修改的字段：

- `expires_at`
- `production_date`
- `opened_at`
- `note`
- `version`

数量和位置变更必须通过流水接口完成。

---

## 13. 库存流水

### 13.1 创建流水

```http
POST /api/v1/inventory/batches/{id}/transactions
X-CSRF-Token: csrf_...
Idempotency-Key: 0191f47a-...
```

#### 消耗

```json
{
  "type": "CONSUME",
  "quantity": 2,
  "unit": "PIECE",
  "occurred_at": "2026-07-28T03:30:00Z",
  "note": "早餐"
}
```

#### 丢弃

```json
{
  "type": "DISCARD",
  "quantity": 1,
  "unit": "PIECE",
  "occurred_at": "2026-07-28T03:30:00Z",
  "note": "破损"
}
```

#### 调整

`ADJUST` 使用调整后的最终数量，而不是增减量：

```json
{
  "type": "ADJUST",
  "new_quantity": 7,
  "unit": "PIECE",
  "occurred_at": "2026-07-28T03:30:00Z",
  "note": "盘点修正"
}
```

#### 移动

```json
{
  "type": "MOVE",
  "quantity": 4,
  "unit": "PIECE",
  "target_location_id": 11,
  "occurred_at": "2026-07-28T03:30:00Z",
  "note": null
}
```

移动部分数量时：

- 原批次数量减少。
- 在目标位置创建新批次。
- 新批次继承原批次生产日期、到期日和物料。
- 返回原批次与新批次。

响应：

```json
{
  "data": {
    "transaction": {
      "id": 50001,
      "type": "CONSUME",
      "quantity": 2,
      "unit": "PIECE",
      "quantity_before": 8,
      "quantity_after": 6,
      "occurred_at": "2026-07-28T03:30:00Z",
      "operator": {
        "id": 1,
        "name": "林墨"
      },
      "note": "早餐"
    },
    "batch": {
      "id": 10001,
      "quantity": 6,
      "status": "ACTIVE",
      "version": 3
    }
  },
  "request_id": "req_01J..."
}
```

库存不足：

```text
INSUFFICIENT_STOCK
```

并发扣减必须使用数据库事务和行锁，不能先查数量再在事务外扣减。

### 13.2 查询流水

```http
GET /api/v1/inventory/transactions
```

支持：

```text
batch_id
material_id
operator_id
type
date_from
date_to
page
page_size
```

---

## 14. Dashboard

### 14.1 家庭首页聚合

```http
GET /api/v1/dashboard/overview
```

该接口用于当前首页首屏，避免前端并发请求多个统计接口。

```json
{
  "data": {
    "greeting": {
      "member_name": "林墨",
      "household_name": "橡木小屋",
      "local_date": "2026-07-28",
      "timezone": "Asia/Shanghai"
    },
    "inventory": {
      "material_count": 23,
      "active_batch_count": 31,
      "expiring_soon_count": 4,
      "expired_count": 1,
      "frozen_material_count": 5,
      "consumed_this_week_count": 12,
      "locations": []
    },
    "expiring_items": [
      {
        "batch_id": 10002,
        "material_id": 101,
        "material_name": "鲜牛奶",
        "icon": {
          "type": "EMOJI",
          "value": "🥛"
        },
        "quantity": 850,
        "unit": "ML",
        "location_name": "冰箱冷鲜",
        "expires_at": "2026-07-29T15:59:59Z",
        "expiry_status": "EXPIRING_URGENT",
        "days_until_expiry": 1
      }
    ],
    "storage_locations": [
      {
        "id": 10,
        "name": "冰箱冷鲜",
        "kind": "REFRIGERATED",
        "material_count": 12,
        "active_batch_count": 15
      }
    ],
    "recent_activities": [
      {
        "id": 50001,
        "type": "INVENTORY_CONSUMED",
        "summary": "林墨消耗了 2 个鸡蛋",
        "occurred_at": "2026-07-28T02:24:00Z"
      }
    ],
    "members": [
      {
        "id": 1,
        "name": "林墨",
        "role": "OWNER",
        "avatar": {
          "type": "EMOJI",
          "value": "🌻"
        }
      }
    ],
    "environment": null,
    "expense": null
  },
  "request_id": "req_01J..."
}
```

`environment` 和 `expense` 在对应模块未实现前返回 `null`，不要返回虚假统计值。

---

## 15. 未登录访客接口

公共接口必须由家庭设置显式开启。

### 15.1 公共首页

```http
GET /api/v1/public/dashboard
```

仅返回：

- 家庭展示名称与图标
- 物料种类数量
- 公开的库存汇总
- 不涉及人员身份的基础统计

禁止返回：

- 成员姓名和状态
- 最近操作人
- Memo
- 摄像头
- 智能设备
- Agent 内容
- 家庭敏感设置

未开启时返回：

```http
404 Not Found
```

不要用 `403` 暴露公共功能真实存在。

### 15.2 公共物料仓库

```http
GET /api/v1/public/inventory
```

实际字段根据 `public_inventory_mode` 决定。

`CATALOG_ONLY` 示例：

```json
{
  "data": [
    {
      "material_id": 100,
      "name": "可生食鸡蛋",
      "category": "蛋奶",
      "icon": {
        "type": "EMOJI",
        "value": "🥚"
      },
      "nutrition": {
        "kcal_per_100g": 144,
        "protein_g_per_100g": 13.3,
        "fat_g_per_100g": 8.8,
        "carbohydrate_g_per_100g": 2.8
      }
    }
  ],
  "meta": {
    "page": 1,
    "page_size": 20,
    "total": 23,
    "total_pages": 2
  },
  "request_id": "req_01J..."
}
```

公共接口必须：

- 只允许 `GET`。
- 限流。
- 不返回内部 ID 以外的敏感关联信息。
- 不允许通过查询参数绕过公开模式。

---

## 16. 权限矩阵

| 操作 | OWNER | ADMIN | MEMBER | ANONYMOUS |
|---|:---:|:---:|:---:|:---:|
| 查看 Dashboard | ✓ | ✓ | ✓ | 仅公共版 |
| 修改家庭资料 | ✓ | ✓ |  |  |
| 创建 ADMIN | ✓ |  |  |  |
| 创建 MEMBER | ✓ | ✓ |  |  |
| 修改成员角色 | ✓ | 有限 |  |  |
| 查看成员 | ✓ | ✓ | ✓ |  |
| 创建存储位置 | ✓ | ✓ |  |  |
| 修改存储位置 | ✓ | ✓ |  |  |
| 查看物料 | ✓ | ✓ | ✓ | 仅公共版 |
| 创建/修改物料种类 | ✓ | ✓ |  |  |
| 查看库存 | ✓ | ✓ | ✓ | 仅公共版 |
| 入库/消耗/移动 | ✓ | ✓ | ✓ |  |
| 查看库存流水 | ✓ | ✓ | ✓ |  |
| 转让所有权 | ✓ |  |  |  |

---

## 17. 第一阶段建议实现顺序

### Sprint 1：初始化与用户

1. `GET /bootstrap`
2. `POST /setup`
3. `POST /auth/sessions`
4. `GET /auth/session`
5. `DELETE /auth/session`
6. 家庭查询和修改
7. 成员 CRUD 和 OWNER 保护

### Sprint 2：物料基础数据

1. 存储位置 CRUD
2. 物料种类 CRUD
3. 营养信息验证
4. 保质期策略验证

### Sprint 3：库存

1. 入库
2. 库存列表和详情
3. 消耗、丢弃、调整、移动
4. 库存流水
5. 并发扣减和幂等性

### Sprint 4：首页与公共访问

1. Dashboard 聚合接口
2. 公共只读接口
3. 权限和限流
4. 前后端联调

---

## 18. Gin 路由建议

```go
api := router.Group("/api/v1")
{
    api.GET("/bootstrap", bootstrapHandler.Get)
    api.POST("/setup", bootstrapHandler.Setup)

    auth := api.Group("/auth")
    {
        auth.POST("/sessions", authHandler.Login)
        auth.GET("/session", requireAuth, authHandler.Session)
        auth.DELETE("/session", requireAuth, requireCSRF, authHandler.Logout)
        auth.POST("/password", requireAuth, requireCSRF, authHandler.ChangePassword)
    }

    protected := api.Group("")
    protected.Use(requireAuth)
    {
        protected.GET("/household", householdHandler.Get)
        protected.PATCH("/household", requireCSRF, householdHandler.Update)

        protected.GET("/members", memberHandler.List)
        protected.POST("/members", requireCSRF, memberHandler.Create)
        protected.GET("/members/:id", memberHandler.Get)
        protected.PATCH("/members/:id", requireCSRF, memberHandler.Update)
        protected.DELETE("/members/:id", requireCSRF, memberHandler.Delete)
        protected.POST(
            "/members/:id/ownership-transfer",
            requireCSRF,
            memberHandler.TransferOwnership,
        )

        protected.GET("/storage-locations", locationHandler.List)
        protected.POST("/storage-locations", requireCSRF, locationHandler.Create)
        protected.GET("/storage-locations/:id", locationHandler.Get)
        protected.PATCH("/storage-locations/:id", requireCSRF, locationHandler.Update)
        protected.DELETE("/storage-locations/:id", requireCSRF, locationHandler.Delete)

        protected.GET("/materials", materialHandler.List)
        protected.POST("/materials", requireCSRF, materialHandler.Create)
        protected.GET("/materials/:id", materialHandler.Get)
        protected.PATCH("/materials/:id", requireCSRF, materialHandler.Update)
        protected.DELETE("/materials/:id", requireCSRF, materialHandler.Delete)

        protected.GET("/inventory/summary", inventoryHandler.Summary)
        protected.GET("/inventory/batches", inventoryHandler.ListBatches)
        protected.POST("/inventory/batches", requireCSRF, inventoryHandler.StockIn)
        protected.GET("/inventory/batches/:id", inventoryHandler.GetBatch)
        protected.PATCH("/inventory/batches/:id", requireCSRF, inventoryHandler.UpdateBatch)
        protected.POST(
            "/inventory/batches/:id/transactions",
            requireCSRF,
            inventoryHandler.CreateTransaction,
        )
        protected.GET("/inventory/transactions", inventoryHandler.ListTransactions)

        protected.GET("/dashboard/overview", dashboardHandler.Overview)
    }

    public := api.Group("/public")
    {
        public.GET("/dashboard", publicHandler.Dashboard)
        public.GET("/inventory", publicHandler.Inventory)
    }
}
```

具体权限不要全部塞进路由中间件。推荐：

- 中间件负责认证、Session、CSRF、request ID。
- Service 层负责资源级权限和业务不变量。
- Repository 层只负责数据访问。
