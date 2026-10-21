# API 参考

所有接口都在 `/rest/v3` 下。除健康检查外都需要认证：默认用 `X-Api-Key` 请求头，`/shorten` 用 `apiKey` query 参数。

接口契约与 Shlink v3 保持一致，官方 `shlink-web-client` 可以直接对接。

## 短链

| Method | Path |
|---|---|
| GET | `/rest/v3/short-urls` |
| POST | `/rest/v3/short-urls` |
| GET | `/rest/v3/short-urls/shorten` |
| GET | `/rest/v3/short-urls/{shortCode}` |
| PATCH | `/rest/v3/short-urls/{shortCode}` |
| DELETE | `/rest/v3/short-urls/{shortCode}` |

`/shorten` 是单步创建接口，只接受 `longUrl` 与 `format`，API Key 走 `apiKey` query 参数；默认返回 JSON，`format=txt` 或 `Accept: text/plain` 时返回纯文本短链。

## 重定向规则

| Method | Path |
|---|---|
| GET | `/rest/v3/short-urls/{shortCode}/redirect-rules` |
| POST | `/rest/v3/short-urls/{shortCode}/redirect-rules` |

## 访问统计

| Method | Path |
|---|---|
| GET | `/rest/v3/visits` |
| GET | `/rest/v3/short-urls/{shortCode}/visits` |
| GET | `/rest/v3/tags/{tag}/visits` |
| GET | `/rest/v3/domains/{domain}/visits` |
| GET | `/rest/v3/visits/orphan` |
| GET | `/rest/v3/visits/non-orphan` |
| DELETE | `/rest/v3/short-urls/{shortCode}/visits` |
| DELETE | `/rest/v3/visits/orphan` |

## 标签与域名

| Method | Path |
|---|---|
| GET | `/rest/v3/tags` |
| GET | `/rest/v3/tags/stats` |
| PUT | `/rest/v3/tags` |
| DELETE | `/rest/v3/tags` |
| GET | `/rest/v3/domains` |
| PATCH | `/rest/v3/domains/redirects` |

## 其他

| Method | Path |
|---|---|
| GET | `/rest/health`、`/rest/v3/health` |
| GET | `/rest/v3/mercure-info` |

Mercure 未配置，`/mercure-info` 固定返回 `501 mercure-not-configured`。

## 响应格式

短链对象：

```json
{
  "shortCode": "abc123",
  "shortUrl": "https://s.example.com/abc123",
  "longUrl": "https://example.com/some/very/long/url",
  "dateCreated": "2026-10-21T10:00:00+08:00",
  "visitsSummary": { "total": 10, "nonBots": 8, "bots": 2 },
  "tags": ["tag1"],
  "meta": { "validSince": null, "validUntil": null, "maxVisits": null },
  "domain": null,
  "title": "Example title",
  "crawlable": false,
  "forwardQuery": true,
  "hasRedirectRules": false
}
```

列表统一返回分页结构（下例为短链列表的默认 `itemsPerPage=10`；访问与标签列表未指定时默认返回全部）：

```json
{
  "data": [],
  "pagination": {
    "currentPage": 1,
    "pagesCount": 1,
    "itemsPerPage": 10,
    "itemsInCurrentPage": 0,
    "totalItems": 0
  }
}
```

错误统一为 Problem Details：

```json
{
  "type": "https://shlink.io/api/error/invalid-data",
  "detail": "Provided data is not valid",
  "title": "Invalid data",
  "status": 400
}
```

已实现的错误类型：`invalid-data`、`non-unique-slug`、`short-url-not-found`、`invalid-short-url-deletion`、`domain-not-found`、`forbidden-tag-operation`、`tag-conflict`、`tag-not-found`、`mercure-not-configured`、`missing-authentication`、`invalid-api-key`、`malformed-request-body`、`not-found`。

## 认证与权限

- 通过 `X-Api-Key` 认证；`GET /rest/v3/short-urls/shorten` 通过 `apiKey` query 参数认证
- 禁用或过期的 Key 返回 `401 invalid-api-key`
- 没有任何角色的 Key 是管理员 Key，可访问全部数据

| 角色 | 语义 |
|---|---|
| `AUTHORED_SHORT_URLS` | 只能访问由该 Key 创建的短链 |
| `DOMAIN_SPECIFIC` | 只能访问指定域名的短链，创建时强制使用该域名 |
| `NO_ORPHAN_VISITS` | orphan 访问列表为空、统计为 0、删除为空操作 |

多个角色同时存在时，短链级限制按交集生效，`NO_ORPHAN_VISITS` 作为额外限制叠加。

## 跳转路由

非 REST 的公开路由：

| Method | Path | 说明 |
|---|---|---|
| GET | `/{shortCode}` | 命中且有效时 `302` 跳转 |
| GET | `/{shortCode}/track` | 返回 1×1 GIF，有效短链记录一次访问 |
| GET | `/robots.txt` | 输出 `crawlable=true` 的短链 |

详细行为见 README 的「短链跳转行为」。
