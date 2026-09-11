# 8V 离线开发任务书

> 配套文档：**[8v-design-skeleton.md](./8v-design-skeleton.md)** —— 这份讲做什么和怎么验收，
> 那份讲改哪些文件、为什么这么设计、骨架长什么样（含类型签名和决策点）。

> 日期：2026-09-11
> 适用场景：出差路上无网络开发
> 前置：Stage A/B/C（记忆库 + 对话持久化 + SSE 流式）已完成并通过 CI 等价检查

---

## 0. 出发前必做（需要网络）

```bash
# 离线时唯一会卡住你的东西：迁移镜像不在本地
docker pull migrate/migrate:v4.19.0

# 只有当你打算在本地构建镜像时才需要（纯 go run 开发用不到）
docker pull golang:1.26-alpine
docker pull node:22-alpine
docker pull alpine:3.23
docker pull nginx:1.28-alpine
```

确认基线是绿的，避免把别的问题带上车：

```bash
go build ./... && go vet ./... && go test ./... -race
npm --prefix web run build
```

---

## 1. 离线环境的硬约束

| 能做 | 不能做 |
|---|---|
| `go build` / `go test`（模块缓存已就位） | **加任何新的第三方依赖**——GOPROXY 不可达 |
| `npm run build`（`web/node_modules` 已就位） | `npm install` 新包 |
| 起 MySQL（`mysql:8.0` 已缓存）验证 SQL | 调真实 DeepSeek API |
| 用假 client 跑完整 agent 循环 | 验证真实模型的 prompt 效果 |

**两条推论，直接决定下面任务的设计：**

1. 新模块只能用标准库。Skill 模块因此用 `embed.FS`，不引入任何 YAML/模板库。
2. 凡是依赖真实模型输出的验收，路上都做不了。所以每个任务都给了**离线可验证的替代判据**，真实效果回来再测。

### 离线跑 agent 循环的办法

`llm.Client` 是接口，造一个假的即可，不需要网络：

```go
// 8v/agent/fake_test.go
type fakeClient struct{ responses []llm.ChatResponse; i int }

func (c *fakeClient) Chat(_ context.Context, _ []llm.Message, _ []llm.Tool) (llm.ChatResponse, error) {
    r := c.responses[c.i]
    c.i++
    return r, nil
}
func (c *fakeClient) Provider() string { return "fake" }
func (c *fakeClient) Model() string    { return "fake" }
```

按顺序摆好"第一轮返回 tool_call、第二轮返回文字"，就能在没有网络的情况下断言整个 `Agent.Run` 的行为。
流式解析同理：`readChatStream` 接受 `io.Reader`，`8v/llm/stream_test.go` 里已有用字符串喂 SSE 片段的样例，照抄即可。

---

## 2. 当前架构里不能破坏的约束

改任何东西之前先记住这四条，CI 会卡你：

1. `internal/service` 不能 import gin；`internal/http/handler` 不能出现 SQL；`internal/repository` 不能 import service。
2. `8v/tools` 只调 service，**永远不直接碰 repository**。
3. `wire_gen.go` 是生成并提交的。改了任何构造函数签名，跑 `go tool wire ./internal/app` 并提交。
4. `trimContext`（`8v/agent/history.go`）只能在 `user` 消息处截断。带 `tool_calls` 的 assistant 消息必须和它的 tool 结果一起出现，否则模型接口整个请求报错。

---

## 3. 任务

> **已完成（不必再做）**：T0 流式滚动、对话轮次上限、tool 组不落库、精灵系统与各类提醒、保留文案。
> 路上要做的是 **T5 → T1 → T3 → T4 → T2**。T5/T1 是主要设计工作；T3/T4 是确定性小活；T2 需要起 MySQL 且依赖真实模型。

---

### ~~T0 修流式滚动~~（已完成）

**问题**：`web/src/views/AgentView.vue` 的 `onToken` 回调里每收到一个 token 就调一次 `scrollToLatest()`，而该函数是 `await nextTick()` + `behavior: "smooth"`。一条长回答几百个 token = 几百个互相打断的平滑滚动动画。这是 Stage C 引入的缺陷。

**改法**：

1. `scrollToLatest` 增加参数区分流式与非流式：流式期间用 `behavior: "auto"`，加载历史/切换会话时才用 `smooth`
2. 节流：用 `requestAnimationFrame` 合并，或只在"用户已经贴着底部"时才滚（用户往上翻看历史时不该被拽回底部）

**验收**：`npm --prefix web run build` 通过；长回答滚动不再抖动。

> 顺带确认：**会话拆分已在 Stage B 完成**（侧栏列表 + 新对话 + 按会话加载历史），服务端每人上限 100 个会话、单次加载 200 条消息。200 条 DOM 节点不构成性能问题，**暂不需要虚拟滚动**——真正的卡顿源是上面的滚动动画。

---

### T1（主线）Skill 模块

**目标**：让能力可以按需加载，而不是把所有指令都堆在系统提示里。现在 `baseSystemPrompt` 已经有 40 行，每加一个领域就线性变长，且每轮都全量发送。

**设计：渐进式披露（两段式）**

系统提示里只放一份**索引**（每个 skill 一行名字+描述），模型判断相关时，调用 `skill_load` 工具取回完整指令。这与现有工具注册表是同一套扩展机制，不引入第二种概念。

```
8v/skills/
  skills.go          # Skill 结构、Registry、embed.FS 解析
  skills_test.go
  assets/
    meal-planning.md
    pantry-audit.md
```

Skill 文件格式（自己解析，不引 YAML 库——用 `strings.Cut` 切出简单的 `key: value` 头部即可）：

```markdown
---
name: meal-planning
description: 规划一周饮食，兼顾库存、忌口和营养均衡
---

## 何时使用
用户要求安排一周的饭菜、问"这周吃什么"...

## 步骤
1. 先调用 pantry_list 看现有食材
2. ...
```

```go
type Skill struct {
    Name         string
    Description  string
    Instructions string   // 正文，按需返回
}

type Registry struct{ skills map[string]Skill }

func NewRegistry() (*Registry, error)   // 解析 embed.FS，启动时失败就报错
func (r *Registry) Index() string        // 渲染进系统提示的索引段
func (r *Registry) Get(name string) (Skill, bool)
```

**接线**：

- `8v/tools/skill.go` 注册 `skill_load` 工具（非 Dangerous，参数 `name`），executor 返回 `Instructions`。
- `tools.Options` 加 `Skills *skills.Registry`；`NewRegistry` 里按 `r.skills != nil` 守卫注册，跟现有 5 个服务一样的写法。
- 索引拼进系统提示：改 `8v/agent/agent.go` 的 `systemPrompt`，在 snapshot 段之后追加 `skills.Index()`。
- `8v/skills` 不依赖 `internal/*`，保持纯粹；wire 里加 `skills.NewRegistry` 到 `agent.ProviderSet`。

**验收（离线可做）**：

- [ ] `go test ./8v/skills` — 每个 embed 的 md 都能解析，name/description 非空，name 与文件名一致
- [ ] `8v/tools/registry_test.go` 的期望列表加上 `skill_load`（该测试断言完整工具集，不加会红）
- [ ] 用假 client 断言：第一轮返回 `skill_load` 调用时，`Agent.Run` 能把正文喂回下一轮
- [ ] `go tool wire ./internal/app` 后 `wire_gen.go` 无额外 diff

**回来后再验**：真实模型是否会在恰当的时候主动 `skill_load`，还是需要在 base prompt 里加更强的引导。

---

### T2 记忆精简与归档（解决无限膨胀）

**现状问题**：硬上限——家庭 50 条、成员各 50 条，超了返回 `ErrAgentMemoryFull`，用户看到"记忆库已满"。对家庭产品这个失败模式很糟：用户不知道该删哪条。

**设计决策：不新建 summary 表，而是给现有表加来源标记。**

理由是提示词注入那条路径（`ListAgentMemoriesForMember` → `HouseholdSnapshot.memorySection()`）**一行都不用改**——summary 本身就是一条记忆，只是 origin 不同。新建表则要在拼提示词处合并两个来源，还要维护两套上限和两套去重。

```
agent_memories       ← 工作集，唯一进提示词的东西
  + origin ENUM('USER','SUMMARY')
  + source_count INT UNSIGNED DEFAULT 0    -- SUMMARY 折叠了多少条
agent_memory_archive ← 被折叠掉的原件，不进提示词，仅供追溯
```

留着原件是有用的：用户问"你为什么觉得我不吃辣"时能答得出来，也给误合并留了回退余地。

**迁移 `000005_agent_memory_summary.{up,down}.sql`**：

- `agent_memories` 加 `origin`、`source_count`，加 CHECK 约束 `origin IN ('USER','SUMMARY')`
- 新表 `agent_memory_archive`：保留原 id、content、scope、member_id、`archived_at`、`summary_id`（指向折叠它的那条）
- **注意**：`member_key` 生成列的去重逻辑对 SUMMARY 行同样适用，别漏了

**触发时机（关键）**：

**精简要调 LLM，绝不能放在 `Remember` 里同步做**——用户记一件事要等一次模型调用，还可能失败。正确做法：

1. `Remember` 达到上限时不报错，先按最旧归档腾位置，并给该 scope 打上"待精简"标记
2. 真正的语义合并交给**已有的每日 maintenance worker**（`internal/app/memo_workers.go` 的 `runMemoMaintenance`，已经是每日 tick，加一个 `conversationService` 那样的依赖即可）

**路上能做 / 不能做**：

| 能做（离线） | 不能做（需要真实模型） |
|---|---|
| 迁移、表结构、归档逻辑 | 语义合并本身（"爸爸不吃辣"+"爸爸怕辣"→ 一条）|
| 超限时归档最旧条目 | 合并质量评估 |
| worker 的调度骨架 + 打标记 | 合并提示词调优 |

建议路上把**归档机制和 worker 骨架**做完，合并的那次 LLM 调用留一个 `TODO` 接口，回来再填。

**验收**：

- [ ] 迁移 up/down 往返干净（起 `mysql:8.0` 容器手工验）
- [ ] 手工 SQL：写满 51 条后第 51 条成功，最旧一条进了 `agent_memory_archive`
- [ ] `ListFor` 不返回归档行（**漏掉这条等于归档没做**）

> 提醒：`internal/` 下目前**零测试文件**，service 层没有现成测试脚手架。要么手工验，要么单独搭一个——别混进 T2。

---

### T3 记忆管理界面（最容易的一块）

**目标**：现在用户看不到 8V 记住了什么，只有模型能通过工具增删。对一个存着"谁在减脂""谁过敏"的家庭产品，这是隐私缺口。

**改动**：

- `GET /api/v1/agent/memories` → 返回当前成员可见的记忆（复用 `AgentMemoryService.ListFor`，已存在）
- `DELETE /api/v1/agent/memories/:id` → 复用 `Forget`（已存在，含归属校验）
- handler 放 `internal/http/handler/agent.go`，路由放 `internal/http/router/agent.go`；读走 `requireSession`，删走 `requireCSRF`
- 前端：`SettingsView.vue` 加一块"8V 记住的事"，或 AgentView 侧栏加入口。api.ts 加两个函数，照 `listAgentConversations` 的写法

**注意**：service 层两个方法都已实现且已做归属校验（别人的私有记忆返回 `ErrNotFound` 而非 403，是刻意的——避免 id 探测）。**这个任务基本只是接线，不要重写 service 逻辑。**

**验收**：

- [ ] `go build ./... && go vet ./...`
- [ ] `npm --prefix web run build` 通过类型检查
- [ ] 三条布局约束：手机宽度可用、删除有二次确认、空态有文案

---

### T4 思考过程流式展示

**现状**：当前 SSE 只发四种事件——`meta`（对话 id）、`tool`（工具名，前端映射成"正在查看库存"）、`token`（答案片段）、`done`/`error`。**没有**思考过程：`8v/llm/stream.go` 的 `streamChunk` 只解析了 `content` 和 `tool_calls`，DeepSeek 的 `reasoning_content` 字段被丢弃了。

**目标**：把推理内容也流出来，前端折叠展示。

**改动**：

1. `streamChunk.Choices[].Delta` 加 `ReasoningContent string \`json:"reasoning_content"\``
2. `llm.Delta` 加 `Reasoning string` 字段（**不要**和 `Content` 混在一起，否则会被当成答案存进数据库）
3. `readChatStream`：reasoning 片段只回调，**不累加进 `ChatResponse.Content`**——思考过程不入库，只是过程展示
4. handler 的 `sseWriter` 加 `OnThinking`，发 `thinking` 事件；`agent.Sink` 接口同步加一个方法
5. 前端 `streamAgentChat` 处理 `thinking` 事件，AgentView 里做成可折叠的浅色小字

**验收（完全可离线）**：

- [ ] 照 `8v/llm/stream_test.go` 的样子，喂一段含 `reasoning_content` 的 SSE 字符串，断言：reasoning 走了回调、且**没有**混进 `response.Content`
- [ ] `go test ./8v/... -race`
- [ ] `npm --prefix web run build`

---

### T5 文件工具与菜谱库

**目标**：skill 和菜谱以 md 文件存在服务器文件系统里，8V 通过工具读取。

#### 5.1 安全：这是本任务书里风险最高的一块

`.env` 里有 `VHOME_SECRET_ENCRYPTION_KEY`、`DEEPSEEK_APIKEY`、数据库密码。**一个能读任意路径的工具，被诱导 `read_file("../.env")` 就会把它们念进聊天记录。**

**用 `os.Root`（Go 1.24 引入，你在 1.26，标准库，离线可用）**，不要自己拼路径校验——手写的 `filepath.Clean` + 前缀比较挡不住符号链接逃逸：

```go
root, err := os.OpenRoot("/app/data/recipes")  // 越界/穿越/符号链接逃逸直接报错
f, err := root.Open(name)
```

**读写分根**：

| 根 | 权限 | 内容 |
|---|---|---|
| `/app/data/skills` | **只读** | skill 的 md |
| `/app/data/recipes` | 读 + 写 | 菜谱 |

skills 根必须只读，否则模型能改写自己的指令——这是自我修改的口子。

其他必须做的限制：单文件大小上限（比如 256KB，否则一个大文件撑爆上下文）、只允许 `.md` 后缀、返回内容前做长度截断。

#### 5.2 别让模型猜路径

`read_file("meal_list/红烧排骨.md")` 是在赌文件名——模型不知道磁盘上有什么。可靠的是两段式，和 skill 同一个模式：

- `recipe_search(keyword)` → 返回匹配的菜名 + 文件名（扫目录 + 读每个文件的 H1/frontmatter 标题）
- `recipe_read(name)` → 返回正文

这样"给我红烧排骨的做法"会走：`recipe_search("红烧排骨")` → 拿到确切文件名 → `recipe_read` → 照着正文回答。**能可靠工作，前提是有 search 这一步。**

菜谱 md 建议带个简单头部，便于 search 提取：

```markdown
---
title: 红烧排骨
tags: 猪肉, 家常菜
---
```

#### 5.3 部署：不加卷的话，重新部署就全没了

`compose.yaml` 现在**只挂了** `vhome_uploads:/app/data/uploads`。容器是不可变的，写在别处的 md 每次 `docker compose up -d` 重建容器就消失。

```yaml
  api:
    volumes:
      - vhome_uploads:/app/data/uploads
      - vhome_recipes:/app/data/recipes     # 新增
      - vhome_skills:/app/data/skills       # 新增（或烤进镜像作只读）

volumes:
  vhome_recipes:
  vhome_skills:
```

Dockerfile 里也要 `mkdir -p` 这两个目录并 `chown vhome:vhome`（容器以 `vhome` 用户运行，不是 root）。

> **和 T1 的取舍**：skill 如果用 `embed.FS` 烤进二进制，就不需要卷、不需要文件工具，但改 skill 要重新部署。放文件系统则可热更新，代价是多两个卷和上面这套安全约束。**菜谱肯定该放文件系统**（用户要能增删改）；**skill 两种都合理**，看你更看重热更新还是部署简单。

**验收（离线可做）**：

- [ ] `os.Root` 越界测试：`read_file("../.env")`、`read_file("/etc/passwd")`、指向根外的符号链接，三者都必须失败
- [ ] `recipe_search` 能从 md 头部提取标题
- [ ] 超大文件被截断而不是全量返回
- [ ] `8v/tools/registry_test.go` 的期望工具列表同步更新

---

## 4. 收工前的完整检查

跟 CI 一致，全部要绿：

```bash
gofmt -l cmd internal 8v          # 必须无输出
go vet ./...
go build ./...
go test ./... -race
go tool wire ./internal/app       # 之后 wire_gen.go 不应有新 diff
npm --prefix web run build

# 分层检查（CI 会跑，本地先过一遍）
grep -rniE 'github.com/gin-gonic/gin' --include='*.go' internal/service      # 应无结果
grep -rniE '(select |insert into |update .* set )' --include='*.go' internal/http/handler
grep -rniE 'vhome/internal/service' --include='*.go' internal/repository
```

---

## 5. 关于 SQLite 与 RAG 的结论（决策记录）

考虑过把长期记忆拆到 SQLite，**当前结论是不拆**，理由按重要性排序：

1. **`sqlite-vec` 这类向量扩展需要 CGO，而 Dockerfile 是 `CGO_ENABLED=0` 静态构建。**纯 Go 的 `modernc.org/sqlite` 不需要 CGO 但加载不了该扩展。"SQLite + 向量检索"意味着放弃静态二进制、重做构建链。
2. **家庭规模下不需要向量索引。**1 万个 chunk × 1024 维 float32 ≈ 40MB，Go 里暴力算余弦约 10ms。向量存成 MySQL 的 BLOB 列完全够用——**RAG 不等于必须换库**。
3. `agent_memories` 有指向 `households`/`members` 的外键；拆库就没有外键完整性。
4. `repository.WithinTransaction` 跨不了两个库，"记账的同时记住这条偏好"无法原子化。

**什么情况下应该重新考虑**：如果是为了单文件备份、独立迁移——那是合理的产品理由（不是技术理由），代价是上面 3、4 两条。真到了检索性能瓶颈，独立的向量服务（qdrant 等）比 SQLite 是更好的下一步。

---

## 6. 回来后需要联网验证的事

这些路上做不了，列出来免得遗漏：

1. 真实模型是否会恰当地调用 `skill_load`（T1）
2. 记忆合并/去重的语义压缩（T2 的 LLM 部分）
3. 端到端追问：「冰箱里还有什么」→「那第二个还能放几天」
4. 跨对话记忆：新开对话后，模型是否遵守记忆库里的忌口约束
5. **流式经过 nginx**：`deploy/nginx.conf` 已加 `proxy_buffering off`，但只有真正过一遍 nginx 容器才算验证过——这个 bug 只在生产出现，`vite dev` 直连 API 看不出来
6. 记忆语义合并的质量（T2 的 LLM 部分）
7. 模型是否会先 `recipe_search` 再 `recipe_read`，而不是直接猜文件名（T5）
