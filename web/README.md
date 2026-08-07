# VHome Web

VHome 的 Vue 前端原型，包含登录页、首次家庭初始化、Dashboard 外壳、家庭首页和物料仓库。

## 本地运行

```bash
npm install
npm run dev
```

开发地址默认为 `http://127.0.0.1:5173`，`/api` 请求会代理到
`http://127.0.0.1:8080`。

## 家庭初始化规则

前端路由实现了以下约束：

- 系统未初始化时，无论是否登录都只能进入 `/setup`。
- `/setup` 同时创建唯一家庭和首位 OWNER，并建立登录会话。
- 家庭创建成功后，访问 `/setup` 会自动回到 Dashboard。
- 已创建家庭的资料只在 `/app/settings` 修改。

当前原型为了便于预览，默认认为家庭已经存在。后端接入时，应以类似下面的
启动状态接口替换本地演示状态：

```json
{
  "authenticated": true,
  "household_exists": true,
  "member": {
    "name": "林墨",
    "role": "OWNER"
  }
}
```

## 当前交互

- 登录及退出。
- Dashboard 左侧导航、收起和移动端菜单。
- 家庭首页统计、临期物料、储物空间、家庭成员状态。
- 物料搜索、位置筛选、网格/列表视图。
- 新增物料弹窗。
- 物料详情和营养信息抽屉。

数据暂时使用前端演示数据，后续接入 Gin 的 `/api/v1` 接口。

完整接口契约见：

- `../docs/api-v1.md`
- `../docs/openapi-v1.yaml`
