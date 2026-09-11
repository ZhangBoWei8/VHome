# 8V 后续模块设计骨架

> 配合 `8v-offline-tasks.md` 使用：那份讲**做什么和怎么验收**，这份讲**改哪里、为什么这么设计、骨架长什么样**。
>
> 这里只有签名和结构，函数体留给你填。凡是标 `// TODO` 的地方都是决策点，不是机械劳动。

---

## 通用：三条不能破的约束

动手前先看一眼，CI 会卡：

1. `8v/**` 只能调 `internal/service`，**不能碰 `internal/repository`**。新模块要落库，就得在 service 开口子。
2. `internal/service` 不能 import gin；`internal/http/handler` 不能出现 SQL。
3. 改了任何构造函数签名 → `go tool wire ./internal/app` 并提交 `wire_gen.go`。

新增工具时**必须**同步改 `8v/tools/registry_test.go` 的期望列表——那个测试断言完整工具集，漏了会红。

---

## T5 文件工具与菜谱库

### 修改范围

| 文件 | 改动 |
|---|---|
| `8v/files/root.go` | **新增**。`os.Root` 的封装，所有路径访问的唯一入口 |
| `8v/files/root_test.go` | **新增**。越界测试是这个模块存在的理由，不能省 |
| `8v/tools/file.go` | **新增**。`read_file` 工具 |
| `8v/tools/registry.go` | `Options` 加 `Files`，`NewRegistry` 里按 nil 守卫注册 |
| `8v/tools/registry_test.go` | 期望工具列表加 `read_file` |
| `internal/config/config.go` | 加 `StorageConfig.RecipeDir` / `SkillDir` |
| `compose.yaml` | 加两个卷 |
| `Dockerfile` | `mkdir -p` 两个目录并 `chown vhome:vhome` |

### 设计思路

**为什么必须用 `os.Root` 而不是自己拼路径校验**：`.env` 里有 `VHOME_SECRET_ENCRYPTION_KEY`、`DEEPSEEK_APIKEY`、数据库密码。手写的 `filepath.Clean` + 前缀比较能挡住 `../`，**挡不住符号链接逃逸**——目录里一个指向 `/app/.env` 的软链就绕过去了。`os.Root`（Go 1.24+，标准库）在系统调用层面把访问限制在根内，是唯一可靠的做法。

**为什么读写分根**：模型如果能写 skill 目录，就能改写自己的指令。这是自我修改的口子，必须从结构上堵死——写入的根和读取 skill 的根是两个不同对象。

### 骨架

```go
// 8v/files/root.go
package files

// Store 是 8V 能看到的全部文件。每个根是独立的 os.Root，
// 越界访问在系统调用层面就失败，不依赖调用方记得做校验。
type Store struct {
    recipes *os.Root  // 可读可写
    skills  *os.Root  // 只读：模型不能改写自己的指令
}

const (
    maxFileBytes = 256 << 10  // 单文件上限，防止一个大文件撑爆上下文
)

// NewStore 打开两个根。目录不存在时应当创建而不是报错——
// 全新部署时卷是空的。
func NewStore(recipeDir, skillDir string) (*Store, error)

// Read 读取一个文件。name 是根内的相对路径。
// 返回内容、是否被截断。
//
// TODO 决策点：截断时是从头截还是保留开头+结尾？菜谱从头截会丢失步骤，
// 建议直接拒绝超限文件并提示，而不是给模型一份残缺的菜谱。
func (s *Store) ReadRecipe(name string) (content string, truncated bool, err error)
func (s *Store) ReadSkill(name string) (content string, err error)

// WriteRecipe 只对菜谱根开放。
func (s *Store) WriteRecipe(name, content string) error

// validateName 是所有入口的第一道关：只允许 .md，不允许绝对路径，
// 不允许路径分隔符以外的可疑字符。
// os.Root 已经挡住了穿越，这里挡的是"文件类型不对"和"名字太离谱"。
func validateName(name string) error
```

```go
// 8v/files/root_test.go —— 这几个用例是模块的验收标准
func TestRootRejectsTraversal(t *testing.T)   // "../.env"、"../../etc/passwd"
func TestRootRejectsSymlinkEscape(t *testing.T) // 在根里建一个指向根外的软链
func TestRootRejectsNonMarkdown(t *testing.T)  // "x.txt"、"x.md.sh"
func TestSkillRootIsReadOnly(t *testing.T)     // 写 skill 根必须失败
```

### 菜谱：别让模型猜路径

模型不知道磁盘上有什么。只加 `read_file` 的话，必须给它一个**索引文件**：

```
/app/data/recipes/
  INDEX.md          ← 菜名 → 文件名
  红烧排骨.md
```

系统提示里加一句硬性要求：**查菜谱前先读 `INDEX.md`**。流程是两次调用：读索引拿到确切文件名 → 读正文。

菜谱 md 建头部便于将来生成索引：

```markdown
---
title: 红烧排骨
tags: 猪肉, 家常菜
---
```

**索引同步是这里最大的坑**。以后加了 `write_file`，新菜谱写完索引没更新，这道菜就等于不存在。

```go
// TODO 决策点：索引谁来维护？
// 方案 A：提示词要求模型写完菜谱顺手更新 INDEX.md —— 不可靠，模型会忘
// 方案 B：Store.WriteRecipe 内部写完后重新扫描目录生成 INDEX.md —— 可靠，推荐
// 方案 C：干脆不用索引文件，加一个 list_recipes 工具 —— 最可靠，但你说只想加 read_file
func (s *Store) RebuildIndex() error
```

### 部署（不做这步，重新部署菜谱就全没了）

容器是不可变的，`compose.yaml` 现在**只挂了** `vhome_uploads`。

```yaml
  api:
    volumes:
      - vhome_uploads:/app/data/uploads
      - vhome_recipes:/app/data/recipes    # 新增
      - vhome_skills:/app/data/skills      # 新增

volumes:
  vhome_recipes:
  vhome_skills:
```

Dockerfile 里同步（注意容器以 `vhome` 用户运行，不是 root）：

```dockerfile
RUN ... && mkdir -p /app/data/uploads /app/data/recipes /app/data/skills \
    && chown -R vhome:vhome /app
```

---

## T1 Skill 模块

### 修改范围

| 文件 | 改动 |
|---|---|
| `8v/skills/skills.go` | **新增**。Skill 结构 + Registry + md 解析 |
| `8v/skills/skills_test.go` | **新增** |
| `8v/tools/skill.go` | **新增**。`skill_load` 工具 |
| `8v/agent/agent.go` | `systemPrompt` 追加 skill 索引段 |
| `8v/tools/registry.go` | `Options` 加 `Skills` |
| `internal/app/wire.go` | `skills.NewRegistry` 进 ProviderSet |

### 设计思路

**渐进式披露**：系统提示里只放索引（每个 skill 一行名字+描述），模型判断相关时调 `skill_load` 取回完整指令。

理由是 `baseSystemPrompt` 已经 40 多行，每加一个领域就线性变长，**而且每一轮都全量发送**。索引一行、正文按需，token 成本从"每轮 × 全部 skill"降到"偶尔 × 一个 skill"。

这和现有工具注册表是同一套扩展机制，不引入第二种概念。

### 骨架

```go
// 8v/skills/skills.go
package skills

type Skill struct {
    Name         string // 与文件名一致，模型用它调 skill_load
    Description  string // 一行，进系统提示索引
    Instructions string // 正文，按需返回
}

type Registry struct {
    skills map[string]Skill
    order  []string // 固定顺序，索引每轮一致，避免模型被抖动的列表影响
}

// NewRegistry 解析全部 skill 文件。
// 解析失败应当直接返回 error 让启动失败——一个格式错误的 skill
// 悄悄被跳过，比启动失败难查得多。
//
// TODO 决策点：v1 你说先用文件系统。那这里接收 *files.Store 还是目录路径？
// 建议直接收路径 + 用 os.Root 自己读，保持 8v/skills 不依赖 8v/tools。
func NewRegistry(dir string) (*Registry, error)

// Index 渲染进系统提示的索引段。无 skill 时返回空串，
// 别输出一个空标题。
func (r *Registry) Index() string

func (r *Registry) Get(name string) (Skill, bool)

// parseSkill 切 frontmatter。不要引 YAML 库（离线装不上，
// 而且这里只需要 key: value）。用 strings.Cut 按 "---" 切三段即可。
func parseSkill(filename string, raw []byte) (Skill, error)
```

skill 文件格式：

```markdown
---
name: meal-planning
description: 规划一周饮食，兼顾库存、忌口和营养均衡
---

## 何时使用
用户要求安排一周饭菜、问"这周吃什么"时。

## 步骤
1. 先 pantry_list 看现有食材
2. 查记忆库里的忌口约束（已在系统提示里，不用再查）
3. ...
```

### 接线

```go
// 8v/tools/skill.go
// skill_load 不是 Dangerous：它只读指令，不改数据。
func (r *Registry) registerSkillTools() {
    r.register(Tool{
        Name: "skill_load",
        Description: "加载一份技能指南的完整内容。系统提示里列出了有哪些技能，" +
            "当用户的需求匹配某个技能时，先调用它拿到详细步骤再动手。",
        // TODO: parameters = { name: string }, required
        Executor: r.executeSkillLoad,
    })
}
```

```go
// 8v/agent/agent.go —— systemPrompt 里追加
b.WriteString(snapshot.PromptSection())
b.WriteString(a.skills.Index())   // 新增
```

### 验收要点

- [ ] 每个 skill 文件都能解析，`name` 与文件名一致
- [ ] 用假 client 断言：第一轮返回 `skill_load` 调用 → 正文被喂回下一轮
- [ ] `registry_test.go` 加 `skill_load`

**回来后要验的**：真实模型会不会主动调 `skill_load`。如果不会，多半要在 `baseSystemPrompt` 里加更强的引导句。

---

## T2 记忆精简与归档

### 修改范围

| 文件 | 改动 |
|---|---|
| `migrations/000005_agent_memory_summary.{up,down}.sql` | **新增** |
| `internal/model/agent.go` | `AgentMemory` 加 `Origin`、`SourceCount` |
| `internal/repository/agent_memory.go` | 归档相关查询 |
| `internal/service/agent_memory.go` | 超限改为归档而非报错 |
| `internal/app/memo_workers.go` | 每日 worker 加语义合并 |

### 设计思路

**不新建 summary 表**，而是给现有表加来源标记。因为提示词注入那条路径（`ListAgentMemoriesForMember` → `HouseholdSnapshot.memorySection()`）**一行都不用改**——summary 本身就是一条记忆，只是 origin 不同。新建表则要在拼提示词处合并两个来源，还要维护两套上限、两套去重、两套作用域过滤。

被折叠的原件进 `agent_memory_archive` 留档：用户问"你为什么觉得我不吃辣"时答得出来，误合并也能回退。

### 骨架

```sql
-- agent_memories 新增
origin       VARCHAR(16) NOT NULL DEFAULT 'USER',  -- USER | SUMMARY
source_count INT UNSIGNED NOT NULL DEFAULT 0,      -- SUMMARY 折叠了几条
CONSTRAINT chk_agent_memories_origin CHECK (origin IN ('USER','SUMMARY'))

-- 新表：被折叠掉的原件，不进提示词
CREATE TABLE agent_memory_archive (
    id, household_id, scope, member_id, content,
    original_id   BIGINT UNSIGNED NOT NULL,   -- 原来的 agent_memories.id
    summary_id    BIGINT UNSIGNED NULL,       -- 折叠它的那条 summary
    archived_at   DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    ...
);
```

> **注意**：`member_key` 生成列的去重逻辑对 SUMMARY 行同样适用，建表时别漏。

```go
// internal/service/agent_memory.go

// Remember 达到上限时不再返回 ErrAgentMemoryFull，
// 改为归档最旧的一条腾位置，并标记该 scope 待精简。
//
// TODO 决策点：归档挑哪一条？
//   - 最旧（created_at）：简单，但可能丢掉仍然有效的老偏好
//   - 最少用（last_used_at）：更合理，但"用过"很难判定——
//     模型读了提示词不等于用了这条记忆
// 建议 v1 用最旧，把 last_used_at 列先加上但不依赖它。
func (s *AgentMemoryService) Remember(...) (RememberResult, error)

// NeedsCompaction 报告哪些 scope 该精简了，给 worker 用。
func (s *AgentMemoryService) NeedsCompaction(ctx) ([]CompactionTarget, error)

// Compact 把一组记忆合并成一条 SUMMARY。
// 这里要调 LLM，所以签名带 client。
//
// !! 绝不能在 Remember 里同步调用：用户记一件事要等一次模型调用，还可能失败。
func (s *AgentMemoryService) Compact(ctx, target CompactionTarget, summarize SummarizeFunc) error

// SummarizeFunc 把合并这一步抽成函数，service 层就不必依赖 8v/llm，
// 也方便离线时传一个假的进来测。
type SummarizeFunc func(ctx context.Context, contents []string) (string, error)
```

**路上能做的**：迁移、归档逻辑、worker 骨架、`SummarizeFunc` 的假实现 + 测试。
**回来再做**：真正的 LLM 合并提示词和质量调优。

---

## T3 记忆管理界面

最轻的一块，**基本只是接线**：`ListFor` 和 `Forget` 已经实现且已做归属校验（别人的私有记忆返回 `ErrNotFound` 而非 403，是刻意的，防 id 探测）。**不要重写 service 逻辑。**

| 端点 | 复用 | 守卫 |
|---|---|---|
| `GET /api/v1/agent/memories` | `AgentMemoryService.ListFor` | `requireSession` |
| `DELETE /api/v1/agent/memories/:id` | `AgentMemoryService.Forget` | `+ requireCSRF` |

`Forget` 需要 `version`，从 query 取或让前端带上。前端照 `listAgentConversations` 的写法加两个函数，界面挂在 `SettingsView.vue` 或 AgentView 侧栏。

> 这是个隐私缺口：现在用户看不到 8V 记住了什么，只有模型能增删。

---

## T4 思考过程流式展示

### 修改范围

`8v/llm/stream.go`（解析）→ `8v/llm/types.go`（Delta）→ `8v/agent/agent.go`（Sink）→ `internal/http/handler/agent.go`（SSE 事件）→ `web/src/api.ts` + `AgentView.vue`（渲染）。

### 骨架

```go
// 8v/llm/stream.go —— streamChunk 的 Delta 加一个字段
ReasoningContent string `json:"reasoning_content"`

// types.go
type Delta struct {
    Content   string
    Reasoning string   // 新增
}
```

**最关键的一点**：reasoning 片段**只回调，不累加进 `ChatResponse.Content`**。

```go
if delta.ReasoningContent != "" && onDelta != nil {
    // 注意：这里不写 content.WriteString
    if err := onDelta(Delta{Reasoning: delta.ReasoningContent}); err != nil { ... }
}
```

混进 `Content` 会连锁出两个问题：思考过程被当成答案存进数据库，以及下一轮回放时模型看到自己的思考被当成了发言。

前端把 `thinking` 事件渲染成可折叠的浅色小字，默认收起。精灵此时正好是 `thinking` 状态。

### 验收（完全可离线）

照 `8v/llm/stream_test.go` 现成的写法，喂一段含 `reasoning_content` 的 SSE 字符串，断言：走了回调 **且没有**混进 `response.Content`。
