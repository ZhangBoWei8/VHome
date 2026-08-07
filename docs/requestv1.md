# 一、用户和家庭管理

包括刚才提到的全部内容。

### 成员审批

```http
GET  /api/v1/members?status=PENDING
POST /api/v1/members/:id/approve
POST /api/v1/members/:id/reject
```

OWNER 可以：

- 查看待审批成员；
- 批准申请；
- 拒绝申请；
- 在灯泡上看到待审批数量。

### 成员管理

```http
GET    /api/v1/members
PATCH  /api/v1/members/:id/role
DELETE /api/v1/members/:id
```

业务规则：

- 只有 OWNER 能修改角色；
- OWNER 可以设置 `ADMIN/MEMBER`；
- OWNER 永远不能被管理员修改；
- OWNER 不能禁用自己；
- 禁用成员时撤销该成员全部 Session；
- 修改角色后撤销旧 Session，使权限立即生效；
- 不物理删除成员，使用 `DISABLED` 保留历史数据。

### 家庭资料

还需要实现：

```http
GET   /api/v1/household
PATCH /api/v1/household
POST  /api/v1/household/password
```

OWNER 可以修改：

- 家庭显示名称；
- 家庭图标；
- 是否开放注册；
- 家庭加入密码。

这里仍然不允许创建第二个家庭，也不提供删除家庭接口。

---

# 二、物料部分的数据关系

物料不能只用一张表完成，至少需要以下关系：

```text
物料种类 materials
        ↓
实际库存批次 inventory_batches
        ↓
库存流水 inventory_transactions

存储位置 storage_locations
        ↓
每个库存批次所在的位置
```

例如：

```text
牛奶
├── 每 100ml 65 kcal
├── 蛋白质 3.2g
├── 冷鲜位置
└── 库存批次
    ├── 1 号批次：500ml，8 月 5 日到期
    └── 2 号批次：1L，8 月 9 日到期
```

“牛奶”是物料种类；买回来的两瓶牛奶是两个库存批次。

---

# 三、存储位置

先实现三种默认位置：

```text
冰箱冷鲜
冰箱冷冻
橱柜
```

允许增加：

```text
零食柜
酒柜
地下储藏室
厨房调料架
```

建议建立 `storage_locations` 表，主要字段：

```text
id
name
kind
icon_type
icon_value
sort_order
enabled
version
created_at
updated_at
```

`kind` 可以是：

```text
REFRIGERATED
FROZEN
CABINET
CUSTOM
```

接口：

```http
GET    /api/v1/storage-locations
POST   /api/v1/storage-locations
PATCH  /api/v1/storage-locations/:id
DELETE /api/v1/storage-locations/:id
```

删除不建议物理删除，而是设置：

```text
enabled = false
```

否则会影响历史库存记录。

---

# 四、物料种类与营养数据

建议建立 `materials` 表，保存“物料是什么”，而不是“现在家里有多少”。

核心字段：

```text
id
name
icon_type
icon_value
expiry_policy
default_shelf_life_days
base_unit
calories_per_100
protein_per_100
fat_per_100
carbohydrate_per_100
status
version
created_at
updated_at
```

营养信息统一表示为：

```text
每 100g 或每 100ml
```

例如鸡蛋可以保存：

```text
热量：143 kcal
蛋白质：12.6g
脂肪：9.5g
碳水：0.7g
```

前端可以根据蛋白质、脂肪、碳水生成三大营养物质占比。

接口：

```http
GET    /api/v1/materials
POST   /api/v1/materials
GET    /api/v1/materials/:id
PATCH  /api/v1/materials/:id
DELETE /api/v1/materials/:id
```

这里的 DELETE 也应该是归档：

```text
status = ARCHIVED
```

因为后续饮食记录和库存流水可能仍然引用它。

---

# 五、期限策略

你最初说的 `fixed/variable` 可以保留在前端显示，但数据库建议使用含义更明确的名称。

### 固定保存时长

```text
ESTIMATED_DURATION
```

适用于：

- 大葱冷鲜保存 7 天；
- 油菜冷鲜保存 5 天；
- 土豆橱柜保存 20 天。

物料种类中保存：

```text
default_shelf_life_days = 7
```

入库后自动计算：

```text
expires_at = stocked_at + 7 天
```

### 每批次指定日期

```text
LABELED_DATE
```

适用于：

- 牛奶；
- 罐头；
- 冷冻包装食品；
- 有包装保质期的商品。

每次入库时填写包装上的到期日。不同批次可以有不同日期。

### 不管理期限

建议额外保留：

```text
NONE
```

适用于暂时不需要到期提醒的物料，避免强迫所有数据都填写期限。

---

# 六、实际库存批次

建议建立 `inventory_batches` 表。

主要字段：

```text
id
material_id
storage_location_id
quantity
unit
stocked_at
expires_at
purchase_cost
status
version
created_by
created_at
updated_at
```

它表示家里真实存在的库存。

例如：

```text
material_id = 牛奶
storage_location_id = 冰箱冷鲜
quantity = 1000
unit = ML
stocked_at = 2026-08-04
expires_at = 2026-08-10
```

接口：

```http
GET   /api/v1/inventory/batches
POST  /api/v1/inventory/batches
GET   /api/v1/inventory/batches/:id
PATCH /api/v1/inventory/batches/:id
```

入库时根据物料的期限策略决定：

```text
ESTIMATED_DURATION → 后端自动计算
LABELED_DATE       → 用户必须提交 expires_at
NONE               → expires_at = NULL
```

---

# 七、库存流水

不能每次直接修改库存数量而不留下记录，否则无法知道库存为什么减少。

建议建立 `inventory_transactions`：

```text
id
batch_id
member_id
type
quantity_delta
from_location_id
to_location_id
note
occurred_at
created_at
```

流水类型：

```text
STOCK_IN
CONSUME
DISCARD
ADJUST
MOVE
```

例如消耗两个鸡蛋：

```text
type = CONSUME
quantity_delta = -2
```

移动到另一个位置：

```text
type = MOVE
from_location_id = 冰箱冷鲜
to_location_id = 冰箱冷冻
```

接口可以统一为：

```http
POST /api/v1/inventory/batches/:id/transactions
```

提交：

```json
{
  "type": "CONSUME",
  "quantity": 2,
  "note": "早餐使用"
}
```

库存数量修改和流水创建必须处于同一个数据库事务中。

---

# 八、到期状态

后端根据 `expires_at` 返回状态：

```text
NONE
NORMAL
EXPIRING_SOON
EXPIRING_URGENT
EXPIRED
```

建议规则：

```text
已过期       → EXPIRED
剩余不超过1天 → EXPIRING_URGENT
剩余2～3天   → EXPIRING_SOON
超过3天      → NORMAL
没有期限     → NONE
```

前端物料仓库可以据此显示不同颜色。

---

# 九、Dashboard 接入真实数据

本阶段只接入与用户和物料有关的数据：

- 家庭成员数量；
- 待审批成员数量；
- 物料种类数量；
- 实际库存批次数量；
- 即将到期数量；
- 已过期数量；
- 冷藏、冷冻、橱柜占比；
- 本周入库成本。

温湿度、水电、家庭总开销暂时保持占位，因为需要智能硬件或单独的账单模块。

---

# 十、访客只读

物料完成后增加：

```http
GET /api/v1/public/dashboard
GET /api/v1/public/inventory
```

访客可以：

- 查看首页部分统计；
- 查看物料和库存；
- 不能看到成员隐私；
- 不能查看成本；
- 不能进行入库、消耗、修改；
- 不能调用 Agent。

---

# 本阶段不包含的功能

以下内容放在后续阶段：

- 每日饮食记录；
- 按重量计算成员摄入热量；
- Memo 和时间提醒；
- 邮件、短信通知；
- 家庭知识库；
- RAG 和 Agent；
- RTSP 家庭监控；
- 智能家居连接；
- 水电和传感器数据；
- 完整家庭账单。

不过物料表中的营养数据会为下一阶段的饮食记录提供基础。

---
