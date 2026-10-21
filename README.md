# shlink-go

用 Go 编写的自托管短链服务，兼容 [Shlink](https://github.com/shlinkio/shlink) 的 REST API v3，可以直接对接官方 `shlink-web-client`。

- 单二进制部署，运行时只依赖 PostgreSQL
- 暂不支持 Redis、RabbitMQ、Mercure、Matomo
- 内置访问统计：GeoLite2 城市级定位、User-Agent 解析、爬虫识别
- 接口细节见 [docs/api.md](docs/api.md)

## 功能

- 短链创建、编辑、删除、列表、搜索、筛选、排序、分页
- 短链跳转：query 转发、trailing slash、extra path、域名级 404 跳转、`robots.txt`、`/{shortCode}/track` 像素
- 访问统计：referer、User-Agent、IP、GeoLite2 城市级定位、爬虫识别、orphan / non-orphan 分类
- 标签管理：列表、统计、重命名、删除，支持 `tagsMode=all|any`
- 域名管理：域名列表、默认域名标记、base URL / 404 / 无效短链的跳转配置
- 重定向规则：11 类条件、按 priority 匹配、整体替换
- 多 API Key 与角色权限（`AUTHORED_SHORT_URLS`、`DOMAIN_SPECIFIC`、`NO_ORPHAN_VISITS`）

## 快速开始

### Docker Compose

```bash
# 1. 生成第一个管理员 API Key
export INITIAL_API_KEY=$(openssl rand -hex 32)

# 2. 启动
docker compose up -d --build
```

服务监听 `:8080`，`serve` 启动时会自动建表、创建默认域名，并在 `api_keys` 为空时用 `INITIAL_API_KEY` 创建管理员 Key。

`docker-compose.yml` 默认把 `GEOLITE2_CITY_DB_PATH` 指向 `/data/GeoLite2-City.mmdb`，并只读挂载 `./data`。需要地理位置解析时把 mmdb 放到 `./data/`；不需要就删掉该环境变量，否则文件缺失会导致启动失败。

### 本地运行

```bash
cp .env.example .env   # 至少填写 DATABASE_URL 和 DEFAULT_DOMAIN
go run ./cmd/shlink-go serve
```

`.env` 从当前工作目录加载，已存在的系统环境变量优先。

### 健康检查

```bash
curl http://127.0.0.1:8080/rest/health
```

```json
{
  "status": "pass",
  "version": "5.1.7",
  "links": {
    "about": "https://shlink.io",
    "project": "https://github.com/shlinkio/shlink"
  }
}
```

`/rest/v3/health` 等价；数据库不可达时返回 `503`，`status` 为 `fail`。

## 配置

全部通过环境变量配置，默认值见 `.env.example`。

| 变量 | 默认值 | 说明 |
|---|---|---|
| `DATABASE_URL` | 必填 | PostgreSQL 连接串 |
| `DEFAULT_DOMAIN` | 必填 | 默认短链域名，不带 scheme 和 path |
| `LISTEN_ADDR` | `:8080` | HTTP 监听地址 |
| `IS_HTTPS_ENABLED` | `false` | 生成 `shortUrl` 时使用 `https` |
| `INITIAL_API_KEY` | 空 | 仅在 `api_keys` 为空时创建管理员 Key |
| `SHLINK_SOURCE_DATABASE_URL` | 空 | 源数据库；配置后 `serve` 自动导入 |
| `GEOLITE2_CITY_DB_PATH` | 空 | GeoLite2-City.mmdb 路径；留空则不解析地理位置 |
| `TRUSTED_PROXIES` | `127.0.0.1,::1` | Gin 可信代理列表，逗号分隔 |
| `CORS_ALLOWED_ORIGINS` | `*` | 允许的跨域来源，逗号分隔 |
| `CORS_ALLOW_CREDENTIALS` | `false` | 是否返回 `Access-Control-Allow-Credentials` |
| `CORS_MAX_AGE` | `3600` | 预检结果缓存秒数 |
| `SHORT_URL_TRAILING_SLASH` | `false` | 短链是否允许以 `/` 结尾 |
| `EXTRA_PATH_MODE` | `default` | extra path 处理：`default` / `append` / `ignore` |
| `DISABLE_TRACK_PARAM` | 空 | 逗号分隔的 query 参数名，请求带这些参数时不记录访问 |
| `DEFAULT_SHORT_CODES_LENGTH` | `5` | 自动生成短码的长度 |
| `DEFAULT_SHORT_CODES_CHARS` | `0-9a-zA-Z` | 自动生成短码的字符集 |
| `ANONYMIZE_REMOTE_IP` | `true` | 存储访问记录前匿名化 IP |
| `TRACK_ORPHAN_VISITS` | `true` | 是否记录 orphan 访问 |
| `LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |

数据库需要 `pg_trgm` 扩展（迁移会尝试 `CREATE EXTENSION`），连接用户需有相应权限。

## CLI

```bash
shlink-go serve                    # 建表、迁移、启动 HTTP 服务
shlink-go migrate                  # 只执行 schema 迁移
shlink-go migrate-shlink           # 从已有数据库导入数据
shlink-go api-key:generate         # 生成 API Key
shlink-go api-key:list             # 列出 API Key
shlink-go api-key:disable          # 禁用
shlink-go api-key:delete           # 删除
shlink-go api-key:rename           # 重命名
```

生成 API Key：

```bash
# 管理员 Key（不带任何角色）
shlink-go api-key:generate --name ci

# 只能看自己创建的短链，指定时间后过期
shlink-go api-key:generate --name bot \
  --role AUTHORED_SHORT_URLS \
  --expiration-date 2026-12-31T00:00:00+08:00

# 只能操作指定域名的短链
shlink-go api-key:generate --name blog --role DOMAIN_SPECIFIC --domain blog.example.com
```

原始 Key 只在生成时输出一次，数据库只保存 SHA-256 哈希。`DOMAIN_SPECIFIC` 不能指向默认域名。

`migrate-shlink` 参数：

```bash
shlink-go migrate-shlink --source-url 'postgres://user:pass@host:5432/shlink?sslmode=disable' --batch-size 1000
```

`--source-url` 省略时读取 `SHLINK_SOURCE_DATABASE_URL`。目标库必须为空且与源库不是同一个数据库；迁移在一个事务内完成，失败整体回滚，可直接重跑。

## 短链跳转行为

| 场景 | 行为 |
|---|---|
| `GET /{shortCode}` | 命中且有效时 `302` 跳转到 `longUrl` |
| `GET /{shortCode}/track` | 返回 1×1 GIF，有效短链记录一次访问 |
| `GET /robots.txt` | 输出 `crawlable=true` 的短链，其余 `Disallow: /` |
| 未生效、已过期、达到最大访问次数 | 按无效短链处理，走域名级 `invalidShortUrlRedirect` |
| 不存在的短码 | 走域名级 `invalidShortUrlRedirect` |
| 多级路径 / 带 `/` 结尾 | 由 `SHORT_URL_TRAILING_SLASH` 与 `EXTRA_PATH_MODE` 决定 |
| 请求带 `DISABLE_TRACK_PARAM` 中的参数 | 正常跳转但不记录访问 |

`EXTRA_PATH_MODE`：

- `default`：带 extra path 的请求按 404 处理
- `append`：把 extra path 追加到目标 URL 后跳转
- `ignore`：忽略 extra path，直接跳转到 `longUrl`

未命中的请求会按 `base_url`、`invalid_short_url`、`regular_404` 分类记录 orphan 访问，并应用对应域名跳转配置。

## 对接 Web 客户端

官方 `shlink-web-client` 可以直接连接本服务，无需修改前端代码：

1. 部署 `shlink-web-client`，或使用 <https://app.shlink.io>
2. 服务器地址填本服务地址，API Key 填 `api-key:generate` 输出的原始 Key
3. Overview、Short URLs、Visits、Tags、Domains、Redirect Rules 页面均可正常使用，仅缺少实时访问刷新

QR Code 生成、短链导出 CSV、用户偏好设置都由 Web 端本地完成，不需要额外后端接口。

## 开发

```bash
make build             # 编译到 bin/shlink-go
make run               # go run ./cmd/shlink-go
make test              # 单元测试
make test-integration  # 集成测试，需要 Docker
make test-all          # 全部测试
make fmt vet lint
```

集成测试用 `testcontainers-go` 拉起 `postgres:18-alpine`，需要本地 Docker 可用。契约测试在缺少 OpenAPI 规范文件时自动跳过。

## 项目结构

```
cmd/shlink-go/          CLI 入口（Cobra）
internal/api/           /rest/v3 handler、DTO、契约与 golden 测试
internal/server/        Gin 中间件、CORS、health、请求日志
internal/redirect/      短链跳转、robots.txt、track 像素、orphan 处理
internal/shorturl/      短链领域服务、短码生成
internal/visit/         访问记录、统计、GeoIP 与 UA 解析
internal/redirectrule/  重定向规则匹配
internal/apikey/        API Key 认证与角色
internal/shlinkmigrate/ 数据导入
internal/store/         Bun/pgx 连接与 schema 迁移
migrations/             SQL 迁移文件（embed 进二进制）
```

## 许可

本项目采用 MIT 协议，见 [LICENSE](LICENSE)。`docs/reference` 下的 OpenAPI 规范、`templates` 与 `public` 中的资源来自上游项目，同样遵循 MIT 协议，详见 [NOTICE](NOTICE)。
