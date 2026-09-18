请生成 `biosample-cold-custody-tracking`「生物样本交接与冻存追踪」Go 全栈项目，服务医院科研中心和生物样本库，管理样本接收、容器位置、交接链和协议复核。不要做诊疗、挂号、费用结算或普通资产仓库。

## 项目主要需求

复杂度下限：核心实体不少于 3 个、核心页面不少于 4 个、横切关注点不少于 2 个、共享前端组件不少于 3 个、自定义 hooks/utils 不少于 2 个、后端中间件不少于 2 个。

### 核心实体

`Specimen`（样本与来源协议）、`StorageContainer`（冻存容器和温区）、`CustodyTransfer`（交接人与时间）、`ProtocolReview`（研究协议复核）必须贯穿数据库、Go 分层、前端 API/store/page。

### 核心页面

`/specimens` 样本队列；`/storage` 冻存位置；`/transfers` 交接工作台；`/protocols` 协议复核；`/audit` 链路审计。`CustodyBadge` 在样本和交接页共用，`SampleDrawer` 在样本和协议页共用。

### 横切关注点

RBAC 与最小权限：角色、Go 中间件、路由守卫、前端显隐联动；不可篡改的交接审计：保存前后位置、操作者和 request ID；加入全局错误处理、脱敏日志和限流。

### 共享枚举/组件

同步 `SpecimenState`（received/aliquoted/stored/released/disposed）与 `TransferState`（prepared/accepted/rejected/cancelled）。共享 `StatusBadge`、`CustodyTimeline`、`EmptyState`，hooks 为 `useAuth`、`usePagination`。

### 技术与规模要求

前端 React 18 + TypeScript + Vite + Ant Design；后端 Go 1.22 + Gin + GORM；PostgreSQL、Redis、MinIO。目标 3000–4200 行、30–42 个 `.go` 文件，不含测试和依赖。

### 文件结构强制清单

前端必须有 `api/stores/types/components/common/hooks/pages/router/utils`；后端必须有 `model/dto/repository/service/handler/router/middleware/constants/util`，README 列出共享枚举的全部出现位置。

### 结构红线

严禁合并职责到单一文件；交接链路必须拆成多个后端和前端文件。

### 部署与交付

根目录必须提供 `docker-compose.yml`（顶层 `name: biosample-cold-custody-tracking`，且不写 `version:`）、`.env` 和 `.env.example`（均含 `COMPOSE_PROJECT_NAME=biosample-cold-custody-tracking`）、`README.md`、`frontend/Dockerfile`、`backend/Dockerfile` 和 `frontend/nginx.conf`。前端端口 `18505`、后端端口 `19505`；Nginx 反代 `/api`，依赖服务有 healthcheck、命名卷和 `condition: service_healthy`，并提供真实 `/healthz` 与 Git 初始化。
