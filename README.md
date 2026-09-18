# 生物样本交接与冻存追踪

面向医院科研中心和生物样本库的全栈系统，覆盖样本接收、分装状态、冻存容器、责任交接、研究协议复核和链路审计。项目不包含诊疗、挂号、费用结算或普通资产仓库能力。

## 主要流程

1. 接收专员登记样本接收号、脱敏受试者编码、来源协议、体积和当前保管人。
2. 保管员维护冷冻柜或液氮罐，发起交接并由另一名有权限的人员接收；接收成功后样本位置、格位、状态和容器占用量在同一事务内更新。
3. 协议复核员核验知情同意、使用范围、保留期限和可选的 MinIO 协议文件对象。通过复核会放行已冻存样本，暂缓或拒绝必须填写说明。
4. 所有关键写操作记录请求 ID、操作者、前后状态、前后位置和保管人，并使用 SHA-256 前向哈希形成只追加审计链。

首次启动会幂等创建 3 个冻存容器、4 份样本、2 条交接记录和 1 条协议复核记录，便于直接验证完整流程。

## 技术结构

- 前端：React 18、TypeScript、Vite、Ant Design、React Router、Axios
- 后端：Go 1.22、Gin、GORM、JWT、RBAC
- 基础设施：PostgreSQL 16、Redis 7、MinIO、Nginx、Docker Compose
- 默认端口：前端 `18505`，后端 `19505`

```text
.
├── frontend/
│   └── src/
│       ├── api,stores,types
│       ├── components/common
│       ├── hooks,utils
│       ├── pages
│       └── router
└── backend/
    ├── cmd/server
    └── internal/
        ├── model,dto,constants
        ├── repository,service
        ├── handler,router
        ├── middleware
        └── config,util
```

## 快速启动

需要 Docker 24+ 和 Docker Compose v2。

```bash
cp .env.example .env
# 修改 POSTGRES_PASSWORD、MINIO_ROOT_PASSWORD 和 JWT_SECRET
docker compose up -d --build
docker compose ps
curl http://localhost:19505/healthz
```

浏览器访问 <http://localhost:18505>。`/healthz` 会真实检查 PostgreSQL、Redis 和 MinIO bucket，任一依赖不可用时返回 `503`。

本地演示账号：

| 账号 | 密码 | 角色 | 写入权限 |
| --- | --- | --- | --- |
| `admin` | `admin123` | 样本库管理员 | 全部权限 |
| `receiver` | `receive123` | 样本接收员 | 接收/更新/变更样本、发起交接 |
| `custodian` | `custody123` | 冻存保管员 | 容器、样本状态、发起和处理交接 |
| `reviewer` | `review123` | 协议复核员 | 协议复核、读取审计 |
| `auditor` | `audit123` | 链路审计员 | 只读审计 |

这些账号和密码仅用于本地验收。部署到共享环境前必须替换或接入组织身份系统。

停止服务并保留数据：

```bash
docker compose down
```

删除本项目命名卷中的本地数据：

```bash
docker compose down -v
```

## API 概览

除登录和健康检查外，请求均需携带 `Authorization: Bearer <token>`。每个响应还会返回 `X-Request-ID`。

| 方法与路径 | 功能 | 写权限 |
| --- | --- | --- |
| `GET /healthz` | 检查 PostgreSQL、Redis、MinIO | 公开 |
| `POST /api/auth/login` | 登录并签发 JWT | 公开 |
| `GET /api/auth/me` | 查询当前用户和权限 | 已登录 |
| `GET /api/storage-containers[/:id]` | 查询冻存容器 | 已登录 |
| `POST /api/storage-containers` / `PATCH /api/storage-containers/:id` | 创建和更新冻存容器 | `storage:write` |
| `GET /api/specimens[/:id]` | 查询样本和交接链 | 已登录 |
| `POST /api/specimens` / `PATCH /api/specimens/:id` | 接收和更新样本 | `specimen:create` / `specimen:update` |
| `POST /api/specimens/:id/transition` | 分装或处置状态变更 | `specimen:transition` |
| `GET /api/custody-transfers[/:id]` | 查询交接 | 已登录 |
| `POST /api/custody-transfers` | 发起交接 | `transfer:prepare` |
| `POST /api/custody-transfers/:id/resolve` | 接收、拒绝或取消交接 | `transfer:resolve` |
| `GET /api/protocol-reviews[/:id]` | 查询协议复核 | 已登录 |
| `POST /api/protocol-reviews` | 提交协议复核 | `protocol:review` |
| `GET /api/audit-logs` | 查询只追加审计事件 | `audit:read` |

登录和创建容器示例：

```bash
curl -s http://localhost:19505/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}'

curl -s http://localhost:19505/api/storage-containers \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"code":"FZ-80-QA1","name":"验收冻存柜","containerType":"ultra_low_freezer","temperatureZone":"minus80","location":"验收区","capacity":100,"status":"available"}'
```

协议复核的 `documentObjectKey` 可留空；传入时，后端会到 `MINIO_BUCKET` 指定的 bucket 中确认对象真实存在。

## 本地开发与校验

```bash
docker compose up -d postgres redis minio

cd backend
go mod download
DATABASE_URL='postgres://biosample:biosample_dev_password@localhost:5432/biosample_custody?sslmode=disable' \
REDIS_ADDR='localhost:6379' \
MINIO_ENDPOINT='localhost:9000' \
MINIO_ACCESS_KEY='biosample-admin' \
MINIO_SECRET_KEY='biosample-minio-password' \
JWT_SECRET='local-development-secret-change-me' \
go run ./cmd/server
```

前端开发服务器会把 `/api` 和 `/healthz` 代理到 `localhost:19505`：

```bash
cd frontend
npm ci
npm run dev
```

完整静态校验：

```bash
cd backend && gofmt -w . && go test ./... && go vet ./... && go build ./...
cd ../frontend && npm ci && npm run build
docker compose config --quiet
```

## 共享枚举同步位置

`SpecimenState` 固定为 `received`、`aliquoted`、`stored`、`released`、`disposed`。

- 后端：`internal/constants/specimen_state.go`、`internal/model/specimen.go`、`internal/dto/common.go`、`internal/handler/specimen_handler.go`、`internal/repository/specimen_repository.go`、`internal/repository/transfer_repository.go`、`internal/repository/protocol_repository.go`、`internal/service/specimen_service.go`、`internal/service/protocol_service.go`、`internal/util/database.go`
- 前端：`src/types/domain.ts`、`src/api/index.ts`、`src/stores/specimenStore.ts`、`src/components/common/CustodyBadge.tsx`、`src/components/common/SampleDrawer.tsx`、`src/pages/SpecimensPage.tsx`、`src/pages/SpecimenDetailPage.tsx`、`src/pages/TransfersPage.tsx`、`src/pages/ProtocolsPage.tsx`
- 测试：`internal/constants/specimen_state_test.go`、`internal/model/quality_rules_test.go`

`TransferState` 固定为 `prepared`、`accepted`、`rejected`、`cancelled`。

- 后端：`internal/constants/transfer_state.go`、`internal/dto/transfer_review.go`、`internal/model/custody_transfer.go`、`internal/model/specimen.go`、`internal/repository/specimen_repository.go`、`internal/repository/transfer_repository.go`、`internal/service/transfer_service.go`、`internal/util/database.go`
- 前端：`src/types/domain.ts`、`src/api/index.ts`、`src/stores/transferStore.ts`、`src/components/common/CustodyBadge.tsx`、`src/components/common/CustodyTimeline.tsx`、`src/pages/TransfersPage.tsx`
- 测试：`internal/constants/specimen_state_test.go`、`internal/model/quality_rules_test.go`

修改枚举时必须同步更新上述位置、数据库兼容策略、测试和 README。

## 安全与一致性

- JWT 使用 HS256，并在后端路由执行 RBAC；前端导航、路由守卫和操作按钮同步权限，但后端仍是最终权限边界。
- 交接受理使用事务和行锁，同时校验来源保管人、来源位置、目标容器容量、格位占用和温区。
- 审计模型拒绝更新和删除，记录前后位置与责任人，并可验证整条 SHA-256 哈希链。
- 请求日志不记录认证头或请求正文；全局错误处理中间件不会向客户端泄露内部错误。
- Redis 提供全局限流；MinIO 承载并校验协议附件对象；所有依赖都由 Compose healthcheck 管理启动顺序。
