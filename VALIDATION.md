# gb-505 验证记录

验证日期：2026-08-22（Asia/Shanghai）

## 提示词核验

- 领域边界：仅实现科研生物样本接收、冻存容器、责任交接、协议复核和只追加审计；未实现诊疗、挂号、收费或普通资产仓库。
- 核心实体：`Specimen`、`StorageContainer`、`CustodyTransfer`、`ProtocolReview` 均独立贯穿数据库、Go 分层和前端 API/store/page。
- 核心页面：`/specimens`、`/storage`、`/transfers`、`/protocols`、`/audit` 均加载真实 API 数据并完成内置 Browser 验收。
- 共享能力：`CustodyBadge`、`CustodyTimeline`、`SampleDrawer`、`StatusBadge`、`EmptyState`、`useAuth`、`usePagination` 均已实际使用。
- 横切能力：JWT、Gin RBAC、前端路由守卫和按钮显隐同步生效；统一错误处理、脱敏日志、Redis 限流、request ID 和 SHA-256 前向哈希审计均已接入。
- 结构：前端包含 `api/stores/types/components/common/hooks/pages/router/utils`；后端包含 `model/dto/repository/service/handler/router/middleware/constants/util`。
- 规模：42 个非测试 Go 文件，共 3777 行；符合 30-42 个 Go 文件和 3000-4200 行要求。
- 部署：Compose 顶层名称、18505/19505 端口、PostgreSQL、Redis、MinIO、健康检查、命名卷、健康依赖和 Nginx `/api` 反代均已验证。

## 构建与静态检查

以下命令均以退出码 0 完成：

```text
cd backend && go test ./...
cd backend && go test -race ./...
cd backend && go vet ./...
cd backend && go build ./...
cd frontend && npm run build
docker compose config --quiet
```

前端构建仅报告 Vite bundle 大小提示，无构建错误。项目从空命名卷启动后，PostgreSQL、Redis、MinIO、backend、frontend 五个服务均为 healthy；`GET /healthz` 返回 PostgreSQL、Redis、MinIO 均为 `ok`。

## API 与业务链

- 实测保管员接收交接单 `#2`，将样本 `#4` 放入容器 `#2` 的 `QA-R99-P01`，温度为 `-79.5°C`。
- 接收交接后产生两条 request ID 均为 `gb505-revalidate-accept` 的审计事件，分别记录交接接收和样本位置更新。
- 接收员读取审计接口返回 403；只读审计员调用写接口返回 403。
- 管理员批准 `BIO-20260822-004 / PROTO-IMMUNE-011` 后，样本由 `stored` 变为 `released`，协议记录和样本放行审计使用同一 request ID。
- 审计详情真实展示操作者、request ID、位置与保管人变化、变更前后 JSON 快照及 SHA-256 审计哈希。

## 内置 Browser

全程仅使用 Codex 内置 Browser，未使用外部 Chrome 或独立 Playwright。

- 五个核心页面、`SampleDrawer`、`CustodyTimeline`、协议复核弹窗、协议历史和审计详情均可正常打开与交互。
- 在协议页完成“通过”复核，勾选知情同意和协议范围核验，提交后列表即时更新并显示成功提示；复核历史显示新记录。
- 审计员登录后可读取审计，但样本接收、容器写入、发起交接和协议复核按钮均不显示。
- 接收员不显示审计导航；直接访问 `/audit` 被路由守卫重定向到 `/specimens`，其样本接收按钮按权限正常显示。
- 桌面截图目视无布局重叠；390×844 视口下 `innerWidth`、`documentElement.scrollWidth`、`body.scrollWidth` 均为 390，无页面级横向溢出。
- 浏览器控制台最终检查：0 error，0 warning。

## 收尾状态

- 验收完成后执行 `docker compose down -v --remove-orphans`。
- 复查 Compose project 容器、命名卷和网络残留均为空。
- 独立 Git 仓库分支为 `main`，本地身份为 `gaobo <gaobo@benzhi.io>`；保留既有提交，不新增提交、不推送。
