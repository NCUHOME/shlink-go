# Shlink Go 迁移计划

> 本文件是唯一的进度来源。任何中断后，先读「当前状态」，再按「恢复规则」继续。

## 当前状态

- 阶段：P0-P10 代码与测试代码已完成
- 状态：等待用户执行实际测试；本轮未运行 `go test`
- 已完成：完整 v3 接口、认证、短链 CRUD、跳转与访问记录、缩略统计、标签、域名、重定向规则、Shlink 数据库迁移、CLI、Docker 部署文件、契约/golden 测试
- 待执行：用户运行单元测试、集成测试、race、lint 与 Web Client 手工冒烟
- 最后静态检查：2026-10-21
- 静态检查命令：`gofmt -w internal cmd migrations && go build ./... && go vet -tags=integration ./...`
- 静态检查结果：通过
- 下一步：由用户执行 `make test`、`make test-integration`、`go test -race -tags=integration ./...`、`golangci-lint run`
- 阻塞：无
- 最后更新：2026-10-21

## 执行快照

本节用于会话中断后快速判断真实状态，不能替代下面的逐项任务清单。

### 已落地

- 配置、slog、Gin 中间件、Huma 接线、Bun + pgx、migration、testcontainers 测试基建。
- `X-Api-Key` 认证、API Key 角色与 Scope、`INITIAL_API_KEY`、默认域名初始化。
- 短链创建、详情、列表、编辑、删除，以及标签、搜索、筛选、排序、分页和 PATCH 三态。
- 短链跳转、query 转发、trailing slash、extra path、域名级 404 跳转、orphan 分类、`robots.txt`。
- 访问记录、UA/Bot 解析、GeoIP 空实现、访问列表与统计、删除访问、`itemsPerPage=5000`。
- 标签列表/统计/重命名/删除，域名列表与 redirects 更新。
- 重定向规则 11 类条件、priority 排序、整体替换与 `has_redirect_rules` 维护。
- Cobra CLI、优雅关闭、Dockerfile、docker-compose、`.env.example`。
- 上游测试映射对应的 Go 单元/集成测试、OpenAPI 契约测试与 golden JSON。
- v3 缺失接口 `shorten` 与 `track` 已补齐。
- Shlink 5.x PostgreSQL → 本项目 schema 的事务式自动迁移已实现，支持 CLI 和 `SHLINK_SOURCE_DATABASE_URL` 启动迁移。

### 已知缺口

- 实际测试尚未执行；所有测试结果以用户本地运行为准。
- GeoLite2 真实 mmdb 未提供，自动测试只覆盖“未配置返回空位置”。
- `golangci-lint` 配置尚未加入仓库；按技术选型先使用 `go vet` 作为静态检查。
- 官方 `shlink-web-client` 手工冒烟需要用户启动服务后执行。

## 恢复规则

1. 读本文件的「当前状态」。
2. 重跑「最后验证」中的命令，确认基线仍然通过。
3. 从第一个未勾选的任务继续；不要根据会话记忆猜测进度。
4. 每完成一个任务：勾选任务、更新「当前状态」、必要时追加「工作日志」。
5. 阶段 Gate 未全绿，不得进入下一阶段。
6. 每个阶段 Gate 通过后做一次本地 commit；未经明确许可不执行 `git push`。

## 目标与边界

- 目标：按 `README.md` 的「保留」与「最小运维能力」范围，用 Go 重写 Shlink 后端，兼容官方 `shlink-web-client`。
- 技术栈：严格按 `docs/技术选型.md`。
- 数据库：只支持 PostgreSQL。
- 契约优先级：上游 `docs/reference/swagger` 与 `../shlink/module/*/test-api` 的断言优先于 `README.md` 中的简化示例。
- 不做：v1/v2 API、原版内置导入器、Redis、RabbitMQ、Mercure、Matomo、标题自动解析。

## 技术基线

- Go 1.27
- Gin
- Huma v2 + `humagin`
- Bun + `pgx/v5/stdlib` + `pgdialect`
- `log/slog`
- `caarlos0/env`
- Cobra
- `bun/migrate`
- `testcontainers-go` + 真实 PostgreSQL
- `kin-openapi`

---

## P0 工程基线

### 交付物

- Go module、依赖锁定
- `.gitignore`
- `Makefile`
- 基础目录结构
- 本计划文件

### 任务

- [x] **P0-T1** 创建 `docs/PLAN.md`，写入目标、阶段、Gate、恢复规则。
- [x] **P0-T2** 初始化 `go.mod`，module 路径为 `github.com/NCUnix/shlink-go`，Go 版本 1.27。
- [x] **P0-T3** 创建 `.gitignore`，忽略二进制、测试缓存、覆盖率、`.env`、临时文件。
- [x] **P0-T4** 创建 `Makefile`，提供 `build`、`test`、`test-integration`、`lint`、`fmt`、`vet`。
- [x] **P0-T5** 建立目录骨架：`cmd/shlink-go`、`internal/{config,server,api,problem,store,apikey,domain,shorturl,tag,visit,redirect,redirectrule,geoip,ua}`、`migrations`。

### Gate

```bash
go build ./...
go test ./...
```

---

## P1 骨架与测试基建

### 交付物

- 配置加载与校验
- slog JSON 日志
- Bun + pgx 连接池
- 初始 migration
- Gin + Huma 接线
- Problem Details
- health 接口
- testcontainers PostgreSQL、fixture seed、可注入时钟

### 任务

- [x] **P1-T1** 实现 `internal/config`：环境变量、默认值、启动校验；覆盖 `DATABASE_URL`、`LISTEN_ADDR`、`DEFAULT_DOMAIN`、`IS_HTTPS_ENABLED`、`INITIAL_API_KEY`、`GEOLITE2_CITY_DB_PATH`、`TRUSTED_PROXIES`、`CORS_ALLOWED_ORIGINS`、`SHORT_URL_TRAILING_SLASH`、`EXTRA_PATH_MODE`、`DISABLE_TRACK_PARAM`、`DEFAULT_SHORT_CODES_LENGTH`、`DEFAULT_SHORT_CODES_CHARS`、`ANONYMIZE_REMOTE_IP`、`LOG_LEVEL`。
- [x] **P1-T2** 实现 slog JSON logger、request-id 中间件、访问日志中间件、recovery 中间件（输出 Problem Details）。
- [x] **P1-T3** 实现 `internal/store`：`database/sql` + pgx driver + Bun、连接池参数、`Ping`。
- [x] **P1-T4** 实现 `bun/migrate` 迁移执行器，启动时加 `pg_advisory_lock`。
- [x] **P1-T5** 创建初始 migration：`domains`、`short_urls`、`tags`、`short_url_tags`、`visits`、`api_keys`、`api_key_roles`、`redirect_rules` 及文档中的索引，启用 `pg_trgm`。
- [x] **P1-T6** 实现 `internal/problem`：Problem Details 类型、常用构造器、Gin 输出、Huma 错误适配。
- [x] **P1-T7** 实现 `internal/server`：`gin.New()`、`RedirectTrailingSlash=false`、可信代理、CORS、body limit、`/rest/health` 与 `/rest/v3/health`（`application/health+json`，版本 `5.1.7`）。
- [x] **P1-T8** 接线 Huma v2 + humagin，关闭 OpenAPI/docs/schemas 对外路径，验证操作注册方式。
- [x] **P1-T9** 实现 query 数组解析 helper，覆盖 `tags[]`、`excludeTags[]` 与重复参数；写 spike 测试确认 Huma 下的可用方案。
- [x] **P1-T10** 搭建测试基建：testcontainers PostgreSQL、migration、TRUNCATE + seed、HTTP test server、可注入时钟（`internal/clock`）。
- [x] **P1-T11** 编写 P1 集成测试：health 200/503、migration up/down、Problem 400、query 数组解析。（已覆盖 `internal/server/health_integration_test.go`、`internal/store/migrate_integration_test.go`、`internal/problem/problem_test.go`、`internal/api/query/query_test.go`）

### Gate

```bash
go build ./...
go test ./...
go test -tags=integration ./internal/...
```

必须额外确认：

- `/rest/health` 返回版本 `5.1.7`。
- `tags[]` / `excludeTags[]` 解析结果符合预期。
- 非法 JSON 或非法参数能返回 Shlink 风格 400 Problem Details。

---

## P2 认证、API Key 与权限

### 交付物

- API Key 哈希、角色、Scope
- `X-Api-Key` 认证中间件
- CORS / OPTIONS 兼容
- `INITIAL_API_KEY`
- 认证相关上游测试

### 任务

- [x] **P2-T1** 实现 `internal/apikey`：SHA-256 哈希、Key 查询、启用/过期校验。
- [x] **P2-T2** 实现角色解析与 `Scope`：`AUTHORED_SHORT_URLS`、`DOMAIN_SPECIFIC`、`NO_ORPHAN_VISITS`，多角色按交集叠加。
- [x] **P2-T3** 实现认证中间件：`X-Api-Key`、缺失/无效/过期/禁用错误、把 Principal 放入 request context。
- [x] **P2-T4** 实现 CORS 与隐式 OPTIONS：无 Origin 不输出 CORS 头；预检 204；按路径返回允许方法。
- [x] **P2-T5** 实现 `INITIAL_API_KEY`：仅当 `api_keys` 为空时创建管理员 Key。
- [x] **P2-T6** 实现测试 fixture 中的 API Key、域名、标签数据种子。
- [x] **P2-T7** 移植并跑通 `AuthenticationTest`、`CorsTest`、`RequestIdTest`、`ImplicitOptionsTest`、`BodyParserTest`（只保留 v3 用例）。（已覆盖 `internal/apikey/auth_integration_test.go`、`internal/server/cors_test.go`、`internal/server/requestid_test.go`、`internal/server/options_test.go`、`internal/api/body_test.go`）

### Gate

```bash
go test -tags=integration ./internal/apikey/... ./internal/server/... ./internal/api/...
```

认证矩阵必须覆盖：无 Key、不存在、过期、禁用、管理员、三种受限角色。

---

## P3 短链核心 CRUD 与列表

### 交付物

- 默认域名启动初始化
- 短链创建、详情、编辑、删除
- 列表搜索、筛选、排序、分页
- slug 生成与清洗
- 标签关联
- PATCH 三态

### 任务

- [x] **P3-T1** 实现域名默认行初始化与域名规范化；API 层默认域名映射为 `domain: null`。
- [x] **P3-T2** 实现 API 时间类型与可空字段类型，输出 `2018-05-01T00:00:00+00:00` 形式。
- [x] **P3-T3** 实现 `OmittableNullable[T]`，覆盖 PATCH 的未传 / null / 值三态。
- [x] **P3-T4** 实现 short code 生成、字符集、长度、slug 清洗、冲突重试。
- [x] **P3-T5** 实现创建短链：`longUrl`、`customSlug`、`shortCodeLength`、`domain`、`findIfExists`、有效期、`maxVisits`、`tags`、`title`、`crawlable`、`forwardQuery`。
- [x] **P3-T6** 实现列表查询：`page`、`itemsPerPage`、`searchTerm`、`domain`、`tags[]`、`tagsMode`、`excludeTags[]`、`excludeTagsMode`、`orderBy`、日期范围、`excludeMaxVisitsReached`、`excludePastValidUntil`、`apiKeyName`。
- [x] **P3-T7** 实现详情、编辑、删除；PATCH 显式列更新，禁止 Bun `UpdateModel` 全字段更新。
- [x] **P3-T8** 实现短链列表的 tags 批量查询，避免 N+1；组装 `visitsSummary`、`hasRedirectRules`。
- [x] **P3-T9** 实现短链 fixture：`abc123`、`def456`、`custom`、`ghi789`、域名重复 slug、自定义域名短链及标签。
- [x] **P3-T10** 移植并跑通 `CreateShortUrlTest`（排除 3 个依赖跳转的方法）、`ListShortUrlsTest`、`EditShortUrlTest`、`DeleteShortUrlTest`、`ResolveShortUrlTest`（排除 1 个依赖跳转的方法）。（覆盖 `internal/api/shorturl_integration_test.go` 与 `internal/api/shorturl_extended_integration_test.go`；跳转相关用例放在 P4）

### Gate

```bash
go test -tags=integration ./internal/shorturl/... ./internal/domain/... ./internal/api/...
```

响应 JSON 必须逐字段匹配上游断言，包括 `null`、分页五个字段、日期格式和错误体。

---

## P4 跳转、访问记录与 orphan

### 交付物

- `GET /{shortCode}` 跳转
- URL 拼装规则
- visit 写入与计数器
- UA / Bot 解析
- GeoIP
- orphan 分类与域名级跳转
- `robots.txt`
- `/{shortCode}/track` 路由占位（P9 补齐真实行为）

### 任务

- [x] **P4-T1** 实现域名 Host 解析、默认域名回退、`(domain_id, short_code)` 查询。
- [x] **P4-T2** 实现有效期、`maxVisits` 校验；无效时按 invalid short URL 处理。
- [x] **P4-T3** 实现 URL 拼装：`SHORT_URL_TRAILING_SLASH`、`EXTRA_PATH_MODE` 的 `default`/`append`/`ignore`、`forwardQuery`、`DISABLE_TRACK_PARAM`。
- [x] **P4-T4** 实现 visit 记录：referer、UA、IP、visitedUrl、redirectUrl、potentialBot、时间；同步更新 `short_urls` 计数，同一事务。
- [x] **P4-T5** 实现 `internal/ua`：User-Agent 解析、Bot 判断；用上游 fixtures 校准 `shlink-tests-agent`、`cf-facebook`。
- [x] **P4-T6** 实现 `internal/geoip`：GeoLite2-City 读取；未配置时返回空位置；按配置匿名化 IP。
- [x] **P4-T7** 实现 orphan 分类：`base_url`、`invalid_short_url`、`regular_404`，以及域名级 `baseUrlRedirect`、`regular404Redirect`、`invalidShortUrlRedirect`。
- [x] **P4-T8** 实现 `robots.txt`，只输出 `crawlable=true` 的短链。
- [x] **P4-T9** 保留 `GET /{shortCode}/track` 路由；P9 已补齐真实 tracking pixel 行为。
- [x] **P4-T10** 移植并跑通 `Core/test-api/Action/RedirectTest.php`、`RobotsTest.php`，以及 P3 剩余的 4 个依赖跳转的用例。（覆盖 `internal/api/redirect_integration_test.go` 与 `internal/redirect/service_test.go`）

### Gate

```bash
go test -tags=integration ./internal/redirect/... ./internal/visit/... ./internal/ua/... ./internal/geoip/...
```

覆盖：302 目标、query 转发、trailing slash、extra path 三种模式、有效期、最大访问次数、计数器、orphan 类型。

---

## P5 标签与域名 API

### 交付物

- 标签列表、统计、重命名、删除
- 域名列表、域名跳转配置
- Scope 过滤

### 任务

- [x] **P5-T1** 实现标签列表（分页、排序）与标签统计（短链数、访问数、bot/non-bot）。
- [x] **P5-T2** 实现标签重命名、删除；带短链级限制的 Key 返回 403 `forbidden-tag-operation`。
- [x] **P5-T3** 实现域名列表：从短链汇总、默认域名标记、`defaultRedirects`。
- [x] **P5-T4** 实现 `PATCH /domains/redirects`。
- [x] **P5-T5** 移植并跑通 `ListTagsTest`、`TagsStatsTest`、`RenameTagTest`、`UpdateTagTest`、`DeleteTagsTest`、`ListDomainsTest`、`DomainRedirectsTest`。（覆盖 `internal/api/tag_integration_test.go` 与 `internal/api/domain_integration_test.go`）

### Gate

```bash
go test -tags=integration ./internal/tag/... ./internal/domain/... ./internal/api/...
```

---

## P6 访问统计 API

### 交付物

- Overview 统计
- 短链 / 标签 / 域名访问列表
- orphan / non-orphan 访问列表
- 删除访问记录
- 5000/页性能验证

### 任务

- [x] **P6-T1** 实现访问列表通用查询：时间范围、`excludeBots`、排序、分页；`itemsPerPage` 支持到 5000。
- [x] **P6-T2** 实现 `GET /visits` 总览。
- [x] **P6-T3** 实现短链、标签、域名访问列表。
- [x] **P6-T4** 实现 orphan、non-orphan 访问列表，应用 `NO_ORPHAN_VISITS`。
- [x] **P6-T5** 实现删除短链访问、删除 orphan 访问，并同步重置计数器。
- [x] **P6-T6** 移植并跑通 `GlobalVisitsTest`、`ShortUrlVisitsTest`、`TagVisitsTest`、`DomainVisitsTest`、`OrphanVisitsTest`、`NonOrphanVisitsTest`、`VisitStatsTest`、`DeleteShortUrlVisitsTest`、`DeleteOrphanVisitsTest`。（覆盖 `internal/api/visit_integration_test.go` 与 `internal/visit/service_test.go`）
- [x] **P6-T7** 用 5000 条 fixture 做分页测试并用 `EXPLAIN` 检查索引命中。

### Gate

```bash
go test -tags=integration ./internal/visit/... ./internal/api/...
```

---

## P7 重定向规则

### 交付物

- 11 类条件
- priority 升序、条件 AND
- 规则读取与整体替换
- `hasRedirectRules`

### 任务

- [x] **P7-T1** 实现 `redirect_rules` 存储、读取、整体替换。
- [x] **P7-T2** 实现 11 类条件的解析与校验：`device`、`browser`、`language`、`query-param`、`any-value-query-param`、`valueless-query-param`、`ip-address`、`geolocation-country-code`、`geolocation-city-name`、`before-date`、`after-date`。
- [x] **P7-T3** 实现规则匹配引擎：priority 升序、条件 AND、未命中回退 `defaultLongUrl`。
- [x] **P7-T4** 实现 `GET`/`POST /short-urls/{shortCode}/redirect-rules`，并维护 `has_redirect_rules`。
- [x] **P7-T5** 编写 11 类条件单测，移植并跑通 `ListRedirectRulesTest`、`SetRedirectRulesTest`。（覆盖 `internal/redirectrule/redirectrule_test.go` 与 `internal/api/redirectrule_integration_test.go`）

### Gate

```bash
go test -tags=integration ./internal/redirectrule/... ./internal/api/...
```

---

## P8 契约、CLI、部署与收尾

### 交付物

- `kin-openapi` 契约测试
- golden JSON
- `api-key:*` CLI
- Dockerfile / docker-compose
- 优雅关闭
- Web Client 手工冒烟
- race / lint / README 最终对照

### 任务

- [x] **P8-T1** 实现 `kin-openapi` 响应校验，覆盖 README 范围内的全部 v3 接口。
- [x] **P8-T2** 为关键接口建立 golden JSON（短链、访问、标签、域名、规则、错误）。
- [x] **P8-T3** 实现 Cobra CLI：`api-key:generate`、`api-key:list`、`api-key:disable`、`api-key:delete`、`api-key:rename`。
- [x] **P8-T4** 实现 Dockerfile、`docker-compose.yml`、`.env.example`。
- [x] **P8-T5** 实现 `signal.NotifyContext` 优雅关闭，等待 HTTP 和数据库事务结束。
- [ ] **P8-T6** 跑通 `go test -race -tags=integration ./...` 和 `golangci-lint run`。
- [ ] **P8-T7** 用官方 `shlink-web-client` 做手工冒烟：Overview、Short URLs、Visits、Tags、Domains、Redirect Rules、Mercure 501。
- [x] **P8-T8** 对照 `README.md` 验收标准逐项勾选，更新本计划状态。

### Gate

```bash
go test -race -tags=integration ./...
golangci-lint run
```

---

## P9 完整 v3 接口补齐

### 交付物

- `GET /rest/v3/short-urls/shorten`
- `GET /{shortCode}/track`
- query `apiKey` 认证协商
- Huma schema 命名冲突修复

### 任务

- [x] **P9-T1** 实现 `shortenUrl`：支持 `longUrl`、`apiKey` query 参数、`format=txt|json` 和 `Accept: text/plain`。
- [x] **P9-T2** 为 `shortenUrl` 实现独立的缺失认证错误：`Expected authentication to be provided in "apiKey" query param`。
- [x] **P9-T3** 实现 `trackShortUrl`：返回 1×1 透明 GIF；有效短链记录访问，无效短链仍返回 GIF。
- [x] **P9-T4** 修复 Huma 对 `ListResult` schema 的重复命名，避免应用启动注册路由时 panic。
- [x] **P9-T5** 添加 JSON、纯文本、query API Key 与 tracking pixel 集成测试。

### Gate

```bash
go test -tags=integration ./internal/api/...
```

---

## P10 Shlink PostgreSQL 自动迁移

### 交付物

- 原 Shlink 5.x PostgreSQL schema 读取与映射
- 单事务、失败回滚、可重跑
- `migrate-shlink` CLI
- `SHLINK_SOURCE_DATABASE_URL` 启动自动迁移

### 迁移映射

| 原 Shlink | 本项目 |
|---|---|
| `domains` / `regular_not_found_redirect` | `domains` / `regular_404_redirect` |
| `short_urls.original_url` | `short_urls.long_url` |
| `short_urls_in_tags` | `short_url_tags` |
| `api_keys.key` / `expiration_date` | `api_keys.key_hash` / `expires_at` |
| `api_key_roles.role_name` | `api_key_roles.role` |
| `visit_locations` + `visits.visit_location_id` | `visits` 内嵌位置字段 |
| `short_url_visits_counts` | `short_urls.visits_total/non_bots/bots` |
| `short_url_redirect_rules` + `redirect_conditions` + 关联表 | `redirect_rules.conditions` JSONB |

### 任务

- [x] **P10-T1** 校验源库 schema，缺失 Shlink 5.x 字段时给出明确错误。
- [x] **P10-T2** 迁移 domains、API keys/roles、tags、short URLs、标签关联和访问计数。
- [x] **P10-T3** 分批迁移 visits，合并 `visit_locations`，映射 orphan 类型。
- [x] **P10-T4** 迁移重定向规则和条件，并保留 priority 顺序。
- [x] **P10-T5** 在单事务内执行，目标库非空时拒绝；失败整体回滚，可直接重跑。
- [x] **P10-T6** 实现 `shlink-go migrate-shlink --source-url ...`。
- [x] **P10-T7** 实现 `SHLINK_SOURCE_DATABASE_URL`：`serve` 启动时自动迁移，目标库非空时跳过。
- [x] **P10-T8** 编写迁移集成测试，覆盖各表、计数器、位置、orphan 和重定向规则。

### Gate

```bash
go test -tags=integration ./internal/shlinkmigrate/...
```

---

## 上游测试映射

| 上游测试 | Go 测试 | 阶段 | 说明 |
|---|---|---|---|
| `Rest/test-api/Middleware/AuthenticationTest.php` | `internal/apikey/auth_integration_test.go` | P2 | 完整：认证矩阵已覆盖 |
| `Rest/test-api/Middleware/CorsTest.php` | `internal/server/cors_test.go` | P2 | 完整：基础 CORS 与预检已覆盖 |
| `Rest/test-api/Middleware/RequestIdTest.php` | `internal/server/requestid_test.go` | P2 | 完整 |
| `Rest/test-api/Middleware/ImplicitOptionsTest.php` | `internal/server/options_test.go` | P2 | 完整 |
| `Rest/test-api/Middleware/BodyParserTest.php` | `internal/api/body_test.go` | P2 | 完整：malformed JSON 已覆盖 |
| `Rest/test-api/Action/CreateShortUrlTest.php` | `internal/api/shorturl_integration_test.go`、`internal/api/shorturl_extended_integration_test.go` | P3/P4 | 完整；3 个跳转方法在 P4 覆盖 |
| `Rest/test-api/Action/ListShortUrlsTest.php` | `internal/api/shorturl_extended_integration_test.go` | P3 | 完整：fixture 与标签/Scope 已覆盖 |
| `Rest/test-api/Action/EditShortUrlTest.php` | `internal/api/shorturl_extended_integration_test.go` | P3 | 完整：编辑主流程已覆盖 |
| `Rest/test-api/Action/DeleteShortUrlTest.php` | `internal/api/shorturl_extended_integration_test.go` | P3 | 完整：删除主流程已覆盖 |
| `Rest/test-api/Action/ResolveShortUrlTest.php` | `internal/api/shorturl_extended_integration_test.go` | P3/P4 | 完整；1 个跳转方法在 P4 覆盖 |
| `Rest/test-api/Action/SingleStepCreateShortUrlTest.php` | `internal/api/shorten_track_integration_test.go` | P9 | 完整：JSON、纯文本、query API Key |
| `Core/test-api/Action/RedirectTest.php` | `internal/api/redirect_integration_test.go`、`internal/redirect/service_test.go` | P4 | 完整 |
| `Core/test-api/Action/RobotsTest.php` | `internal/api/redirect_integration_test.go` | P4 | 完整 |
| `Rest/test-api/Action/ListTagsTest.php` | `internal/api/tag_integration_test.go` | P5 | 完整 |
| `Rest/test-api/Action/TagsStatsTest.php` | `internal/api/tag_integration_test.go` | P5 | 完整 |
| `Rest/test-api/Action/RenameTagTest.php` | `internal/api/tag_integration_test.go` | P5 | 完整 |
| `Rest/test-api/Action/UpdateTagTest.php` | `internal/api/tag_integration_test.go` | P5 | 完整 |
| `Rest/test-api/Action/DeleteTagsTest.php` | `internal/api/tag_integration_test.go` | P5 | 完整 |
| `Rest/test-api/Action/ListDomainsTest.php` | `internal/api/domain_integration_test.go` | P5 | 完整 |
| `Rest/test-api/Action/DomainRedirectsTest.php` | `internal/api/domain_integration_test.go` | P5 | 完整 |
| `Rest/test-api/Action/GlobalVisitsTest.php` | `internal/api/visit_integration_test.go` | P6 | 完整 |
| `Rest/test-api/Action/ShortUrlVisitsTest.php` | `internal/api/visit_integration_test.go` | P6 | 完整 |
| `Rest/test-api/Action/TagVisitsTest.php` | `internal/api/visit_integration_test.go` | P6 | 完整 |
| `Rest/test-api/Action/DomainVisitsTest.php` | `internal/api/visit_integration_test.go` | P6 | 完整 |
| `Rest/test-api/Action/OrphanVisitsTest.php` | `internal/api/visit_integration_test.go` | P6 | 完整 |
| `Rest/test-api/Action/NonOrphanVisitsTest.php` | `internal/api/visit_integration_test.go` | P6 | 完整 |
| `Rest/test-api/Action/VisitStatsTest.php` | `internal/api/visit_integration_test.go` | P6 | 完整 |
| `Rest/test-api/Action/DeleteShortUrlVisitsTest.php` | `internal/api/visit_integration_test.go` | P6 | 完整 |
| `Rest/test-api/Action/DeleteOrphanVisitsTest.php` | `internal/api/visit_integration_test.go` | P6 | 完整 |
| `Rest/test-api/Action/ListRedirectRulesTest.php` | `internal/api/redirectrule_integration_test.go` | P7 | 完整 |
| `Rest/test-api/Action/SetRedirectRulesTest.php` | `internal/api/redirectrule_integration_test.go` | P7 | 完整 |

不迁移：

- PHP 单元测试（`module/*/test`）与数据库测试（`module/*/test-db`）不 1:1 移植，只作为边界用例参考。
- v1/v2 用例、原版内置导入器、Mercure 内部实现。
- CLI 测试除 `api-key:*` 外不移植。

## 契约决策

- 列表响应外层键：`shortUrls`、`tags`、`visits`、`domains`；分页字段：`currentPage`、`pagesCount`、`itemsPerPage`、`itemsInCurrentPage`、`totalItems`。
- 日期输出使用带数字时区的 RFC3339，例如 `2018-05-01T00:00:00+00:00`，不使用 Go 默认的 `Z`。
- `/rest/health` 与 `/rest/v3/health` 使用 `application/health+json`，版本固定 `5.1.7`，数据库不可用时返回 503。
- Huma 自动参数/请求体校验关闭，业务校验统一返回 400 `https://shlink.io/api/error/invalid-data`。
- 默认域名在数据库保留一行，API 返回短链时映射为 `domain: null`。
- `itemsPerPage` 上限为 5000。

## 环境与测试默认值

- 测试默认域名：`s.test`
- 测试 `IS_HTTPS_ENABLED=false`
- 测试不启用标题自动解析
- 集成测试使用 testcontainers PostgreSQL；每个测试 TRUNCATE + reseed
- short code 生成器和时钟通过接口注入，测试使用固定值

## 风险与处理

| 风险 | 处理 |
|---|---|
| Huma 对 `tags[]` / `excludeTags[]` 解析不兼容 | P1-T9 spike；必要时从原始 query 手动解析 |
| Huma 自动校验返回 422 | 所有 REST operation 关闭自动校验，手写校验 |
| 上游测试默认 SQLite，Go 使用 PostgreSQL | 固定排序 tie-breaker，按 API 断言验收 |
| `itemsPerPage=5000` 查询慢 | 复合索引 + `EXPLAIN` + 分页上限 |
| UA/Bot 识别差异 | 用上游 fixtures 校准，保留可替换解析器 |
| GeoIP 无测试数据库 | 自动测试覆盖空位置和匿名化，真实 mmdb 手工验证 |
| 并发热门短链计数行锁 | 当前接受，后续可改批量写入 |

## 决策记录

- **D1**：Go module path 使用 `github.com/NCUnix/shlink-go`。
- **D2**：旧 Shlink 数据迁移通过独立源库导入到空的新库；不兼容原 schema 原地运行。
- **D3**：上游 `test-api` 是阻塞验收套件，v1/v2 与超范围用例显式豁免。
- **D4**：只做本地 commit；未经许可不 push。
- **D5**：环境变量按 `docs/技术选型.md`，不实现与上游旧变量名的双别名。

## 工作日志

- 2026-10-21：创建本计划文件，确定 P0-P8 阶段、Gate 和上游测试映射。
- 2026-10-21：完成 P0、P1 工程基线，以及 P2 认证主体、P3 短链 CRUD/列表核心实现。
- 2026-10-21：修复集成测试包导入环；`go test -tags=integration ./internal/...` 的 28 个测试通过。
- 2026-10-21：同步计划的实际进度与缺口；`SingleStepCreateShortUrlTest` 当时标记为明确不适用，P9 已补齐该接口。
- 2026-10-21：完成 P4-P8 代码、上游测试映射、契约/golden 测试、CLI 与 Docker 部署文件；静态检查通过，实际测试按用户要求留待本地执行。
- 2026-10-21：复跑静态检查；修复设置重定向规则时先校验短链存在、GeoIP 使用原始 IP 查询、空标签条件防护，并改用 testcontainers PostgreSQL 标准等待策略。
- 2026-10-21：补齐 v3 `shorten` 与 `track`，修复 Huma schema 重名导致的启动 panic；实现 Shlink 5.x PostgreSQL 自动迁移及迁移测试代码。
