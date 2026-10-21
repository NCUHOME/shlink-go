# Shlink 后端 Go 重写范围与兼容性说明

- 目标：使用 Go 重写 Shlink 后端，继续使用官方 `shlink-web-client`
- 数据库：只支持 PostgreSQL
- 使用场景：自己使用；保留多 API Key 与角色权限
- 原则：数据库 schema 可以重新设计，但 Web 端依赖的 API 契约和行为必须兼容

## 目标运行形态

- Redis 未启用：没有 Redis cache、Redis lock、Redis Pub/Sub
- RabbitMQ 未启用：没有异步事件投递
- Mercure 未启用：Web 端没有实时访问刷新
- Matomo 未启用：不发送访问数据到 Matomo

因此，运行只依赖 PostgreSQL；缓存、锁和异步处理都在进程内或本地完成。

## 1. 总体范围

### 保留

- PostgreSQL
- REST API v3
- 短链创建、编辑、删除、列表、搜索、筛选、分页
- 短链跳转、query 转发、trailing slash、extra path
- 标签管理
- 域名列表和域名跳转
- 访问统计、GeoIP、User-Agent 解析、爬虫识别
- orphan / non-orphan visits
- 重定向规则
- 多 API Key 与角色权限（`AUTHORED_SHORT_URLS`、`DOMAIN_SPECIFIC`、`NO_ORPHAN_VISITS`）
- `robots.txt`（如果保留 `crawlable` 功能）

### 明确不做

- MySQL、MariaDB、SQL Server、SQLite
- 除 API Key 管理外的 CLI 命令
- 5 种导入源：Bitly、CSV、Kutt、Shlink、YOURLS
- 原版安装器
- Matomo
- RabbitMQ
- Redis
- Mercure 实时刷新（`GET /rest/v3/mercure-info` 固定返回未配置错误）
- `POST /short-urls/shorten`
- `GET /{shortCode}/track`
- `deviceLongUrls`

### 最小运维能力

不需要完整 CLI，但需要保留：

- 建表 / migration
- 初始化第一个 API Key：空库时通过 `INITIAL_API_KEY` 创建无角色（管理员）Key
- API Key 管理命令：`api-key:generate`、`api-key:list`、`api-key:disable`、`api-key:delete`、`api-key:rename`
- 准备 GeoLite2 数据库文件
- 健康检查

除上述最小能力和 `api-key:*` 子集外，不保留其他 CLI 命令。原版没有 API Key 的 REST 管理接口，本重写也不新增。

## 2. Web 功能覆盖

现有 `shlink-web-client` 的以下页面应保持可用：

- Overview：访问总览
- Short URLs：列表、创建、编辑、删除
- Short URL Visits：单个短链的访问统计
- Tag Visits：按标签查看访问
- Domain Visits：按域名查看访问
- Orphan Visits：无效短链、基础 URL、普通 404 的访问
- Non-orphan Visits：所有非 orphan 访问
- Visits Comparison：短链、标签、域名之间的访问对比
- Tags：标签列表、统计、重命名、删除
- Domains：域名列表、域名跳转配置
- Redirect Rules：短链重定向规则
- Real-time Updates：不启用；Web 端可以正常打开和操作，只是没有实时访问刷新

Web 端启动时会请求 `GET /rest/v3/mercure-info`。Go 后端应返回 `501` 的 `mercure-not-configured` Problem Details，让 Web 端按“未配置”处理，不影响其他页面。

以下功能由 Web 端自行完成，不需要额外后端接口：

- QR Code 生成
- 短链列表导出：Web 端循环调用 `GET /short-urls` 后在前端生成 CSV
- 用户偏好设置：保存在 Web 端本地

## 3. REST API 兼容清单

Web 端通过 `@shlinkio/shlink-js-sdk` 调用 `/rest/v3`，需要兼容以下接口。

只需要兼容 v3：官方 Web 客户端固定使用 v3，不需要兼容 v1/v2 旧客户端。

### 短链

| Method | Path | 用途 |
|---|---|---|
| GET | `/rest/v3/short-urls` | 短链列表 |
| POST | `/rest/v3/short-urls` | 创建短链 |
| GET | `/rest/v3/short-urls/{shortCode}` | 获取短链详情 |
| DELETE | `/rest/v3/short-urls/{shortCode}` | 删除短链 |
| PATCH | `/rest/v3/short-urls/{shortCode}` | 编辑短链 |

### 重定向规则

| Method | Path | 用途 |
|---|---|---|
| GET | `/rest/v3/short-urls/{shortCode}/redirect-rules` | 读取规则 |
| POST | `/rest/v3/short-urls/{shortCode}/redirect-rules` | 设置规则 |

### 访问统计

| Method | Path | 用途 |
|---|---|---|
| GET | `/rest/v3/visits` | 访问总览 |
| GET | `/rest/v3/short-urls/{shortCode}/visits` | 短链访问列表 |
| GET | `/rest/v3/tags/{tag}/visits` | 标签访问列表 |
| GET | `/rest/v3/domains/{domain}/visits` | 域名访问列表 |
| GET | `/rest/v3/visits/orphan` | orphan 访问列表 |
| GET | `/rest/v3/visits/non-orphan` | non-orphan 访问列表 |
| DELETE | `/rest/v3/short-urls/{shortCode}/visits` | 删除短链访问 |
| DELETE | `/rest/v3/visits/orphan` | 删除 orphan 访问 |

### 标签

| Method | Path | 用途 |
|---|---|---|
| GET | `/rest/v3/tags` | 标签列表 |
| GET | `/rest/v3/tags/stats` | 标签统计 |
| DELETE | `/rest/v3/tags` | 删除标签 |
| PUT | `/rest/v3/tags` | 重命名标签 |

### 域名

| Method | Path | 用途 |
|---|---|---|
| GET | `/rest/v3/domains` | 域名列表 |
| PATCH | `/rest/v3/domains/redirects` | 编辑域名跳转 |

### 其他

| Method | Path | 用途 |
|---|---|---|
| GET | `/rest/v3/health` 和 `/rest/health` | 健康检查，可选接受 `domain` 查询参数 |
| GET | `/rest/v3/mercure-info` | Mercure 未配置时返回 `501` Problem Details |

### 非 REST 路由

| Method | Path | 用途 |
|---|---|---|
| GET | `/{shortCode}` | 短链跳转 |
| GET | `/robots.txt` | 输出可被爬取的短链 |

## 4. 关键 DTO 与响应格式

字段名、类型、可空性需要与 Shlink 保持一致，否则 Web 端会出现解析错误或页面空白。

### 短链对象

```json
{
  "shortCode": "abc123",
  "domain": null,
  "shortUrl": "https://s.example.com/abc123",
  "longUrl": "https://example.com/some/very/long/url",
  "dateCreated": "2026-10-21T10:00:00+08:00",
  "visitsSummary": {
    "total": 10,
    "nonBots": 8,
    "bots": 2
  },
  "meta": {
    "validSince": null,
    "validUntil": null,
    "maxVisits": null
  },
  "tags": ["tag1", "tag2"],
  "title": "Example title",
  "crawlable": false,
  "forwardQuery": true,
  "hasRedirectRules": false
}
```

### 访问对象

```json
{
  "referer": "https://example.com",
  "date": "2026-10-21T10:00:00+08:00",
  "userAgent": "Mozilla/5.0 ...",
  "visitLocation": {
    "countryCode": "CN",
    "countryName": "China",
    "regionName": "Shanghai",
    "cityName": "Shanghai",
    "latitude": 31.2304,
    "longitude": 121.4737,
    "timezone": "Asia/Shanghai",
    "isEmpty": false
  },
  "potentialBot": false,
  "visitedUrl": "https://s.example.com/abc123",
  "redirectUrl": "https://example.com/some/very/long/url"
}
```

orphan 访问额外包含：

```json
{
  "type": "base_url | invalid_short_url | regular_404"
}
```

### 标签统计对象

```json
{
  "tag": "tag1",
  "shortUrlsCount": 3,
  "visitsSummary": {
    "total": 10,
    "nonBots": 8,
    "bots": 2
  }
}
```

### 域名对象

```json
{
  "domain": "s.example.com",
  "isDefault": false,
  "redirects": {
    "baseUrlRedirect": null,
    "regular404Redirect": null,
    "invalidShortUrlRedirect": null
  }
}
```

域名列表额外返回 `defaultRedirects`。

### 重定向规则对象

```json
{
  "defaultLongUrl": "https://example.com/default",
  "redirectRules": [
    {
      "longUrl": "https://example.com/mobile",
      "priority": 1,
      "conditions": [
        {
          "type": "device",
          "matchKey": null,
          "matchValue": "mobile"
        }
      ]
    }
  ]
}
```

### Mercure 未配置响应

```json
{
  "type": "https://shlink.io/api/error/mercure-not-configured",
  "detail": "This Shlink instance is not integrated with a mercure hub.",
  "title": "Mercure integration not configured",
  "status": 501
}
```

## 5. 行为兼容点

### 5.1 短链创建与编辑

需要支持 Web 表单涉及的字段：

- `longUrl`
- `customSlug`
- `shortCodeLength`
- `domain`
- `findIfExists`
- `validSince`
- `validUntil`
- `maxVisits`
- `tags`
- `title`
- `crawlable`
- `forwardQuery`

需要兼容的行为：

- 自定义 slug
- 自动生成 slug
- slug 冲突时返回 `non-unique-slug` 错误
- `findIfExists=true` 时复用已存在的长 URL
- 默认域名与自定义域名
- 短链有效期
- 最大访问次数
- 标题保存
- 标签关联
- `crawlable` 影响 `robots.txt`
- `forwardQuery` 决定是否把访问者 query 参数合并到目标 URL

### 5.2 列表、搜索、筛选、排序

`GET /short-urls` 需要兼容以下 query 参数：

- `page`
- `itemsPerPage`
- `searchTerm`
- `domain`
- `tags[]`
- `tagsMode=all|any`
- `excludeTags[]`
- `excludeTagsMode=all|any`
- `orderBy`
- `startDate`
- `endDate`
- `excludeMaxVisitsReached`
- `excludePastValidUntil`
- `apiKeyName`（按创建短链的 API Key 名称过滤；管理员 Key 可过滤任意 Key，受限 Key 只能过滤自身，其他值忽略）

`orderBy` 支持：

- `dateCreated-ASC|DESC`
- `shortCode-ASC|DESC`
- `longUrl-ASC|DESC`
- `title-ASC|DESC`
- `visits-ASC|DESC`
- `nonBotVisits-ASC|DESC`

分页响应必须是：

```json
{
  "data": [],
  "pagination": {
    "currentPage": 1,
    "pagesCount": 1,
    "totalItems": 0
  }
}
```

### 5.3 短链跳转

需要兼容：

- `/{shortCode}` 跳转
- 自定义域名下查找短链
- 默认域名回退
- 查询参数转发
- `SHORT_URL_TRAILING_SLASH` 行为
- extra path 的 `default` / `append` / `ignore` 模式
- 短链未生效、已过期、达到最大访问次数时按“无效短链”处理
- 无效短链、基础 URL、普通 404 的域名级跳转配置
- orphan visit 记录
- `DISABLE_TRACK_PARAM` 参数不转发
- `robots.txt` 只输出 `crawlable=true` 的短链

### 5.4 访问统计

需要兼容：

- 记录 referer、User-Agent、IP、visitedUrl、redirectUrl
- GeoLite2 城市级位置解析
- IP 匿名化配置
- 爬虫识别，写入 `potentialBot`
- `excludeBots` 过滤
- orphan / non-orphan 区分
- orphan 类型：`base_url`、`invalid_short_url`、`regular_404`
- 按时间范围过滤
- 分页
- 访问总览统计 `total`、`nonBots`、`bots`
- 短链、标签、域名的 `visitsSummary` 统计

Web 端的访问统计页面会按 **5000 条/页** 并发拉取历史访问记录，因此后端需要支持较大的 `itemsPerPage`，并为查询建立合适的 PostgreSQL 索引。

### 5.5 重定向规则

需要兼容 11 类条件：

- `device`
- `browser`
- `language`
- `query-param`
- `any-value-query-param`
- `valueless-query-param`
- `ip-address`
- `geolocation-country-code`
- `geolocation-city-name`
- `before-date`
- `after-date`

行为要求：

- 规则按 priority 匹配
- 未命中任何规则时使用 `defaultLongUrl`
- 条件之间的组合语义与 Shlink 一致
- Web 端读取和整体替换规则时保持顺序和优先级

### 5.6 标签与域名

标签：

- 标签列表
- 标签统计：短链数、访问数、bot/non-bot 数
- 重命名
- 删除
- 标签过滤的 any/all 逻辑

域名：

- 从现有短链中汇总域名
- 默认域名标记
- 每个域名的 base URL、普通 404、无效短链跳转配置
- 域名级别 health 检查

### 5.7 Mercure 不启用

- `GET /rest/v3/mercure-info` 返回 `501` Problem Details，类型为 `https://shlink.io/api/error/mercure-not-configured`
- Web 端不会建立 EventSource，也不会实时刷新访问数据
- Overview、Visits、Short URLs、Tags、Domains、Redirect Rules 等其他页面不受影响
- 不实现 Mercure Hub、JWT、topic 发布和订阅逻辑

## 6. 认证、错误、CORS、分页

### 认证与 API Key

- Web 端通过 `X-Api-Key` 请求头认证
- 支持多个 API Key，每个 Key 包含名称、过期时间、启用状态和角色集合
- 通过 `api-key:generate` 生成时，原始 Key 只输出一次；数据库只保存 `SHA-256` 哈希
- Key 名称在应用层唯一；重名创建或重命名需要报错
- 禁用或过期的 Key 视为无效，返回 `401 invalid-api-key`
- 没有任何角色的 Key 是管理员 Key，可访问全部数据
- 初始化第一个 Key：`INITIAL_API_KEY` 仅在 `api_keys` 为空时生效
- 管理入口：保留 `api-key:*` 命令子集；不提供 REST 管理接口

### API Key 角色与权限

支持三种角色，行为与原版一致：

| 角色 | 语义 |
|---|---|
| `AUTHORED_SHORT_URLS` | 只能访问由该 Key 创建的短链 |
| `DOMAIN_SPECIFIC` | 只能访问指定域名的短链；创建短链时强制使用该域名 |
| `NO_ORPHAN_VISITS` | orphan visits 列表为空、统计为 0、删除为空操作 |

原版只有这三种角色，没有通用 RBAC；本重写不扩展自定义角色或权限点。

规则：

- 无角色的 Key 是管理员 Key。
- 多个角色同时存在时，短链级限制按交集（AND）生效；`NO_ORPHAN_VISITS` 作为额外限制叠加。
- `DOMAIN_SPECIFIC` 不能指向默认域名，创建此类 Key 时需要拒绝。
- `DOMAIN_SPECIFIC` 的 `meta` 至少保存 `domain_id` 和 `authority`；角色保存在 `api_key_roles`，同一 Key 同一角色唯一。
- 上游 5.x 已将 API Key 角色标记为 deprecated，但现有实现仍然生效；本重写按现有行为兼容。

权限覆盖范围：

- 短链：列表、详情、编辑、删除、访问统计、重定向规则都只能访问当前 Key 可见的短链；创建时记录 `author_api_key_id`。
- 标签：列表和统计只基于可见短链；带短链级限制的 Key 重命名或删除标签返回 `403 forbidden-tag-operation`。
- 域名：列表只返回可见域名；`DOMAIN_SPECIFIC` Key 不返回默认域名项。
- 访问统计：Overview、短链/标签/域名访问、non-orphan 都按可见短链过滤；`NO_ORPHAN_VISITS` 下 orphan 相关结果为 0 或空。
- orphan 访问不属于任何短链：`AUTHORED_SHORT_URLS` 和 `DOMAIN_SPECIFIC` 不会隐藏 orphan 访问，只有 `NO_ORPHAN_VISITS` 会隐藏。
- `apiKeyName` 过滤：管理员 Key 可过滤任意 Key；受限 Key 只能过滤自身，其他值忽略。

### 错误格式

必须返回 Problem Details JSON：

```json
{
  "type": "https://shlink.io/api/error/invalid-data",
  "detail": "Invalid data",
  "title": "Invalid data",
  "status": 400
}
```

至少需要兼容 Web 端识别的错误类型：

- `invalid-data`
- `non-unique-slug`
- `short-url-not-found`
- `invalid-short-url-deletion`
- `domain-not-found`
- `forbidden-tag-operation`
- `tag-conflict`
- `tag-not-found`
- `mercure-not-configured`
- `missing-authentication`
- `invalid-api-key`
- `not-found`

### CORS

Web 客户端可能与 API 不同源，需要支持：

- `Access-Control-Allow-Origin`
- `Access-Control-Allow-Credentials`
- `Access-Control-Allow-Headers`，至少包含 `X-Api-Key`、`Content-Type`
- `Access-Control-Allow-Methods`
- `Access-Control-Max-Age`

### 分页

所有列表接口统一返回：

```json
{
  "data": [],
  "pagination": {
    "currentPage": 1,
    "pagesCount": 1,
    "totalItems": 0
  }
}
```

### Health 与版本

`GET /health` 返回：

```json
{
  "status": "pass",
  "version": "5.x.x",
  "links": {
    "about": "https://shlink.io",
    "project": "https://github.com/shlinkio/shlink"
  }
}
```

Web 端会根据 `version` 决定是否显示某些功能入口，例如 IP 重定向、地理位置重定向、日期重定向、浏览器重定向。版本号需要与实际支持的功能匹配。

## 7. 数据与运维范围

- 数据库只使用 PostgreSQL
- 可以重新设计 schema，不必照搬 Doctrine 映射
- 建议表：`short_urls`、`tags`、`short_url_tags`、`domains`、`visits`、`visit_locations`、`redirect_rules`、`redirect_conditions`、`api_keys`、`api_key_roles`；短链表需要 `author_api_key_id`
- API 的 JSON DTO 必须保持兼容，数据库结构不需要兼容
- GeoLite2 数据库文件由部署环境提供，不实现自动下载和安装器
- 如果已有旧 Shlink 数据，需要单独做一次性数据迁移；如果从零开始则不需要
- 不实现导入器，但可以为旧数据写一次性的迁移脚本

## 8. 验收标准

- 现有 `shlink-web-client` 可以直接连接 Go 后端，无需修改前端代码
- Web 端可以正常创建、编辑、删除、搜索和分页展示短链
- 短链可以正常跳转，query 转发、trailing slash、extra path 行为正确
- Overview、Short URL Visits、Tag Visits、Domain Visits、Orphan Visits、Non-orphan Visits 页面正常
- Tags、Domains、Redirect Rules 页面正常
- Mercure 未启用时，`/mercure-info` 返回 `501 mercure-not-configured`，Web 端其他页面正常，但没有实时刷新
- API 错误格式、字段名、可空性、日期格式与 Web 端预期一致
- 访问统计接口在 `itemsPerPage=5000` 时仍能正常分页返回
- `X-Api-Key` 认证、CORS、健康检查正常
- 多 API Key、过期/禁用校验和三类角色的数据隔离行为与 Shlink 一致
- 只依赖 PostgreSQL，不引入 Redis、RabbitMQ、Mercure、Matomo

## 9. 待确认的边界

以下功能会明显影响范围，需要在开始实现前确认：

1. **自定义域名**：保留则实现域名表和域名跳转；只用单域名则可以去掉域名管理。
2. **重定向规则**：保留则实现 11 类条件和规则优先级；不需要则删除对应页面能力。
3. **GeoIP 地理位置**：保留则需要 GeoLite2；不需要则访问位置信息为空。
4. **Orphan visits**：保留则实现 404/orphan 跟踪；不需要则 orphan 页面为空。
5. **标题自动解析**：保留则创建短链时自动访问目标 URL 提取标题；不需要则只能手动填写。
6. **历史数据迁移**：如果当前已有 Shlink 数据，需要确认是否迁移以及迁移方式。
