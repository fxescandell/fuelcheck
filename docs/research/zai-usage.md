# z.ai (Zhipu AI): credential discovery and quota readouts

**Research date:** 2026-11-05
**Scope:** how a local CLI (this repo, `fuelcheck`) can discover a z.ai user's credentials and read their quota usage + reset time.
**Primary sources used:** z.ai's own published source code — the Claude Code plugin marketplace `zai-org/zai-coding-plugins` (first-party, `github.com`), the ZCode agent harness `zai-org/ZCode` (first-party, `github.com`), the ZCode desktop app bundle installed on this machine (`/Applications/ZCode.app`), the first-party npm package `@z_ai/coding-helper`, and z.ai's official docs (`docs.z.ai`). Plus live calls against the user's own z.ai account from this machine.

> Secret **values** are never reproduced here. Key names, file shapes, and path shapes are reproduced verbatim.
>
> The endpoints below are **not** in z.ai's public OpenAPI spec — `https://docs.z.ai/openapi.json` contains exactly 14 paths (`/paas/v4/chat/completions`, `/paas/v4/videos/generations`, …) and **none** of them is a usage/quota/billing path (verified locally: `python3` scan of the downloaded spec → `matching paths: []`). They are nevertheless *first-party*: they are the endpoints z.ai's own shipped clients call.

---

## 0. Answer summary

| Question | Answer |
| --- | --- |
| Is there a quota endpoint? | **Yes.** `GET {zai business base}/api/monitor/usage/quota/limit` → `https://api.z.ai/api/monitor/usage/quota/limit` (international) or `https://open.bigmodel.cn/api/monitor/usage/quota/limit` (CN mirror). Undocumented publicly, but called by z.ai's own plugin and app. |
| Does it return % used / limit / reset? | Yes: `percentage` (**used** share, 0–100), `usage` / `currentValue` / `remaining` (absolute, on some limits only), `nextResetTime` (**epoch milliseconds**), and `unit`/`number` (window identity). |
| Auth | `Authorization: <key>` **raw, no `Bearer` prefix** (first-party requirement). `Bearer <key>` also works in practice (verified live). |
| Pay-as-you-go balance endpoint? | **Not reachable with an API key.** `GET /api/biz/customer/getCustomerInfo` → `HTTP 403 "APIKey not allow access"`. The only balance readout found is ZCode's own backend (`/api/v1/zcode-plan/billing/balance`), which needs a ZCode session JWT. |
| Where are the credentials? | Env vars (`ZAI_API_KEY`, `ANTHROPIC_AUTH_TOKEN`+`ANTHROPIC_BASE_URL`), `~/.claude/settings.json`, `~/.zcode/v2/credentials.json` (AES-256-GCM encrypted), plus third-party tools' own files (`~/.config/openusage/zai.json`, `~/.config/glm-acp-agent/credentials.json`). **No macOS Keychain usage.** |
| Windows | **5-hour** (`unit=3, number=5`), **weekly** (`unit=6`, present only for accounts/plans that expose it), **monthly MCP/tool** (`TIME_LIMIT, unit=5, number=1`). Per-model breakdown via `usageDetails[]`. |
| Unknowns | §8. |

---

## 1. Which z.ai products expose a usage/quota readout?

z.ai has **three distinct, non-interchangeable** quota surfaces plus a subscription-metadata surface.

### 1.1 GLM Coding Plan (subscription) — *the one we want*

- A subscription ("GLM Coding Plan", tiers **Lite / Pro / Max**) with **two rolling windows**: a 5-hour credit pool and a **7-day weekly** credit pool.
  Source: <https://docs.z.ai/devpack/overview.md> → "Usage Instruction / Usage Credit Allowance": *"Each plan is subject to both a 5-hour usage limit and a weekly usage limit."* Table: Lite 2,000 / 10,000 credits; Pro 12,000 / 60,000; Max 28,000 / 140,000. *"5-hour credits: Dynamically refreshed; credit quota resets 5 hours after consumption. Weekly credits: Activated upon subscription; resets every 7 days."*
- Also a **monthly MCP/tool-call pool** (Web Search / Web Reader / Zread), charged in credits.
  Source: <https://docs.z.ai/devpack/overview.md> (MCP Server multipliers, 1.2 credits/call); <https://zcode.z.ai/en/docs/usage-stats> ("Quota status such as the 5-hour prompt pool, weekly quota, and monthly MCP quota").
- Legacy (pre-2026-07-30) plans instead used **prompts** as the unit, with the same 5-hour + weekly window shape.
  Source: <https://docs.z.ai/devpack/notice/usage-revision.md> → "Usage Reference for Legacy Plans": *"we apply usage limits on a 5-hour and weekly basis"*; table "5-Hour Limit … Weekly Limit (Activated upon subscription; resets every 7 days)".
- **Where the quota is read**: the console page <https://z.ai/manage-apikey/subscription> (web only) and — undocumented — the API in §2.
- Team Plan adds per-seat 5-hour and weekly quotas.
  Source: <https://docs.z.ai/devpack/teamplan.md> (linked from <https://docs.z.ai/devpack/usage-policy.md>).

### 1.2 Pay-as-you-go API balance (Standard API Key) — *different resource, no windows*

- Since 2026-10-16, z.ai splits keys: **Standard API Key** (pay-as-you-go / usage bundles, billed against account balance) vs **Coding Plan API Key** (plan quota) vs **Team Plan Key**.
  Source: <https://docs.z.ai/devpack/notice/api-key-management.md> → "Which API key should I use?" and the billing table.
- The account **balance** is *not* exposed to API keys: `GET https://api.z.ai/api/biz/customer/getCustomerInfo` with an API key returns `HTTP 403` body `APIKey not allow access: /biz/customer/getCustomerInfo` (live, 2026-11-05). The same path on `open.bigmodel.cn` returns the identical 403.
- The only balance readout found is in ZCode's own backend: `GET {zcode base}/api/v1/zcode-plan/billing/balance` and `.../billing/current`, built in first-party code as `zcodePlanBillingBalanceUrl` / `zcodePlanBillingCurrentUrl`. Unauthenticated probe returns `HTTP 401` (endpoint exists, requires a ZCode session token).
  Source: `zai-org/ZCode` → `packages/services/src/model-provider/zaiStartPlanBilling.ts`, and the endpoint table in the desktop bundle (see §6.3).
- **Conclusion for this repo**: a pay-as-you-go balance readout is *not* available to a local CLI without a logged-in ZCode session. Only the Coding Plan quota (§2.1) is reachable with a plain API key.

### 1.3 Official MCP / tool-call quota

- Two representations of the same monthly tool pool:
  - inside the quota endpoint as a `TIME_LIMIT` entry with `usageDetails[]` per MCP server;
  - as a dedicated ZCode-backend endpoint `GET {zcode base}/api/v1/mcp/usage` → `data.total_usage{used, limit, remaining}`, `data.next_refresh_at` (Unix **seconds**), `data.server_time`, `data.level`.
  Source: `zai-org/ZCode` → `packages/services/src/usage-stats/providers/zcodeMcpQuotaProvider.ts:1` (`GET /api/v1/mcp/usage`), `:24` (`const MCP_USAGE_PATH = "/api/v1/mcp/usage"`), `:46-57` (zod schema), `:210` (`buildRuntimeZCodeApiUrl(params.env, MCP_USAGE_PATH)`), `:299-311` (`nextResetTime: data.next_refresh_at * 1000`).

### 1.4 International platform (`z.ai` / `api.z.ai`) vs `open.bigmodel.cn`

- **Same endpoints, same backend, same account.** Live verification on 2026-11-05 with the same key:
  - `GET https://api.z.ai/api/monitor/usage/quota/limit` and `GET https://open.bigmodel.cn/api/monitor/usage/quota/limit` returned **byte-identical** `limits` arrays and identical `level`.
  - `GET https://api.z.ai/api/biz/subscription/list` and the `open.bigmodel.cn` twin returned the same `customerId`, same `id` (`"248671"`), same `productName` (`"GLM Coding Lite"`), same `valid` window.
- First-party code treats them as one family with two hosts: `zai-org/ZCode` → `packages/services/src/coding-plan-subscription/zaiCodingPlanSubscriptionProvider.ts:56-59` (`resolveZaiBusinessBaseUrl`), and the Claude Code plugin's own platform switch:
  `plugins/glm-plan-usage/skills/usage-query-skill/scripts/query-usage.mjs:44-53` → `api.z.ai` ⇒ `ZAI`, `open.bigmodel.cn` / `dev.bigmodel.cn` ⇒ `ZHIPU`, both using the identical path `/api/monitor/usage/quota/limit`.
- **Practical consequence**: a CLI can use whichever host the user's key belongs to; both behave the same. (Which host *issues* a key is a console-side distinction: <https://z.ai/manage-apikey/apikey-list> vs <https://bigmodel.cn/usercenter/proj-mgmt/apikeys>, the latter URL appearing in the first-party npm package `@z_ai/coding-helper`.)

### 1.5 Team Plan

- Team Plan quota lives on the same `quota/limit` endpoint but is routed by query parameter: `?type=2`.
  First-party comment, `zai-org/ZCode` → `packages/services/src/usage-stats/providers/bigmodelUsageQuotaProvider.ts:1171-1174`: *"BigModel Team Plan 的 quota/limit 后端按 type=2 路由到团队套餐。只切换团队项目 key/header 仍会走个人 Coding Plan 分支并返回『当前用户不存在 coding plan』。"*
  Team usage-stat endpoints additionally use `type=3` for `credit-usage/*` (`:1591-1593`).
- z.ai Team Plan quota entries use `CREDIT_LIMIT` instead of `TOKENS_LIMIT` (see §3.3).

---

## 2. The endpoints

All paths below are relative to the **business base URL**: `https://api.z.ai` (international) or `https://open.bigmodel.cn` (CN) — z.ai's own code calls this the "zai 业务域名 / ZAI Business origin".

### 2.1 `GET /api/monitor/usage/quota/limit` — the quota readout

| | |
| --- | --- |
| Method / path | `GET https://api.z.ai/api/monitor/usage/quota/limit` (no query params) |
| Auth header | `Authorization: <api key>` — **raw credential, no `Bearer` prefix** |
| Extra headers | `Accept-Language: en-US,en`, `Content-Type: application/json` |
| Returns | `data.limits[]` — one entry per quota window + `data.level` (plan tier) |

**First-party source of the path** — `zai-org/zai-coding-plugins` → `plugins/glm-plan-usage/skills/usage-query-skill/scripts/query-usage.mjs`:

```js
44:  if (baseUrl.includes('api.z.ai')) {
45:    platform = 'ZAI';
46:    modelUsageUrl = `${baseDomain}/api/monitor/usage/model-usage`;
47:    toolUsageUrl  = `${baseDomain}/api/monitor/usage/tool-usage`;
48:    quotaLimitUrl = `${baseDomain}/api/monitor/usage/quota/limit`;
49:  } else if (baseUrl.includes('open.bigmodel.cn') || baseUrl.includes('dev.bigmodel.cn')) {
50:    platform = 'ZHIPU';
...
53:    quotaLimitUrl = `${baseDomain}/api/monitor/usage/quota/limit`;
```

and the auth header, same file `:111-124`:

```js
118:      method: 'GET',
119:      headers: {
120:        'Authorization': authToken,      // ← raw token, no "Bearer "
121:        'Accept-Language': 'en-US,en',
122:        'Content-Type': 'application/json'
123:      }
```

The ZCode app hits the same path; `zai-org/ZCode` → `packages/services/src/usage-stats/providers/bigmodelUsageQuotaProvider.ts:73`:
`const BIGMODEL_QUOTA_PATH = "/api/monitor/usage/quota/limit";` and `:1688-1691`:

```ts
function buildZaiQuotaUrl(env: NodeJS.ProcessEnv = process.env): string {
  // ZAI usage/quota 与 business login 共用业务域名。
  return buildRuntimeZaiBusinessUrl(env, "/api/monitor/usage/quota/limit");
}
```

Header construction, same file `:1151-1157` — the authoritative statement on auth:

```ts
function createBigModelUsageHeaders(resolved: ResolvedQuotaAuthorization): Record<string, string> {
  const headers: Record<string, string> = {
    // monitor 接口要求 authorization 直接传完整凭据。
    // 普通用量和 Coding Plan 用量都走 API Key，不能额外加 Bearer 前缀。
    authorization: resolved.authorization,
  };
```

### 2.2 `GET /api/monitor/usage/model-usage` and `GET /api/monitor/usage/tool-usage` — consumption history

- `?startTime=YYYY-MM-DD HH:mm:ss&endTime=YYYY-MM-DD HH:mm:ss` (URL-encoded), `Authorization` raw.
- Response `data`: `x_time[]` (hour buckets), `modelCallCount[]`, `tokensUsage[]`, `totalUsage{...}` for model-usage; `networkSearchCount[]`, `webReadMcpCount[]`, `zreadMcpCount[]`, `totalUsage{...}` for tool-usage.
- First-party source: plugin `query-usage.mjs:44-53` (paths), `:65-85` (time window = "yesterday at the current hour → today at the current hour end"), and `zai-org/ZCode` → `bigmodelUsageQuotaProvider.ts:1564-1580` (`buildUsageMonitorUrl`, replaces `/quota/limit` with `/${endpoint}` and adds `startTime`/`endTime`).
- Live capture (2026-11-05, redacted to shape): `{"x_time":["2026-11-04 21:00", …], "modelCallCount":[129,1,…], "tokensUsage":[64484922,…], "totalUsage":{…}}`.

### 2.3 `GET /api/monitor/usage/credit-usage/activity` and `.../credit-usage/usage-detail`

- `?type=1` (personal) or `type=3` (team) + `startTime`/`endTime` (+ `usageType=MODEL|MCP`).
- Source: `zai-org/ZCode` → `bigmodelUsageQuotaProvider.ts:1582-1599`:

```ts
url.pathname = url.pathname.replace(/\/usage\/quota\/limit$/, `/credit-usage/${endpoint}`);
// Team Plan 的 quota/limit 仍用 type=2，但 credit-usage 使用 type=3；
url.searchParams.set("type", resolved.teamContext ? "3" : "1");
```

- [UNVERIFIED] — not called live from this machine (the ZCode account in question is on a legacy plan); response shape not captured.

### 2.4 `GET /api/monitor/usage/model-performance-day`

- `?startTime=&endTime=` (7d/30d ranges), source: `bigmodelUsageQuotaProvider.ts:1602-1611` (`buildModelPerformanceMonitorUrl`).
- [UNVERIFIED] — not called live; response shape not captured.

### 2.5 `GET /api/biz/subscription/list` — plan + renewal dates

- `Authorization: Bearer <key>` (or raw — both work live), `Accept: application/json`.
- Live capture (2026-11-05, identifying fields removed):

```json
{"code":200,"msg":"Operation successful","success":true,"data":[
  {"id":"248671","productId":"product-db684c","productName":"GLM Coding Lite",
   "productBigTitle":"GLM Coding Plan","status":"VALID",
   "purchaseTime":"2026-02-10 19:38:17","valid":"2027-02-10 19:38:13-2028-02-10 19:38:13",
   "autoRenew":0,"currentPeriod":2,"currentRenewTime":"2026-02-10","nextRenewTime":"2027-02-10",
   "billingCycle":"annually","inCurrentPeriod":true,"paymentChannel":"STRIPE",
   "banStatus":0,"version":"V1"},
  {"id":"474390","productName":"GLM Coding Lite","status":"VALID",
   "purchaseTime":"2026-04-30 17:24:25","currentRenewTime":"2027-02-10","nextRenewTime":"2027-03-10",
   "billingCycle":"monthly","paymentType":"SUCCESS"}]}
```

- **Field names that matter**: `productName` (plan tier, e.g. "GLM Coding Lite"), `status` (`VALID`), `inCurrentPeriod`, `currentRenewTime` / `nextRenewTime` (renewal ⇒ monthly pool boundary), `billingCycle`, `valid` (start–end of the current period).
- First-party source of the path and of the field set: `zai-org/ZCode` → `packages/services/src/usage-stats/providers/bigmodelUsageQuotaMapper.ts:11-23` (`BigModelSubscriptionListItem` interface: `productId, productName, status, inCurrentPeriod, purchaseTime, valid, autoRenew, currentRenewTime, nextRenewTime, billingCycle`) and `:100-124` (`pickCurrentSubscriptionFromList` → `renewTime`, `expireTime`).
- Also used by z.ai's own Claude Code plugin ecosystem? Not used by `glm-plan-usage`; the plugin only reads quota. [Note]

### 2.6 ZCode-backend endpoints (need a ZCode session, not an API key)

| Endpoint | Purpose | Auth |
| --- | --- | --- |
| `GET {zcode base}/api/v1/mcp/usage` | Official MCP quota (`total_usage`, `next_refresh_at`, `level`) | ZCode headers (see §2.7) |
| `GET {zcode base}/api/v1/coding-plan/reset/status` | Quota-reset-card availability | `Authorization: Bearer <zcodejwttoken>` + `X-Bigmodel-Authorization: <oauth token>` |
| `POST {zcode base}/api/v1/coding-plan/reset/use` | Consume a reset card | same |
| `POST {zcode base}/api/v1/coding-plan/reset/history/read` | Mark reset history read | same |
| `GET {zcode base}/api/v1/zcode-plan/billing/balance` | Pay-as-you-go balance for Start Plan | ZCode JWT (live probe → `HTTP 401`) |
| `GET {zcode base}/api/v1/zcode-plan/billing/current` | Current billing state | ZCode JWT |

First-party source: `zai-org/ZCode` → `packages/services/src/usage-stats/providers/bigmodelUsageQuotaProvider.ts:74` (`CODING_PLAN_RESET_BASE_PATH = "/api/v1/coding-plan/reset"`), `:525-530` (→ `.../reset/status`), `:558-562` (→ `.../reset/use`), `:634-638` (→ `.../reset/history/read`), `:982-991`:

```ts
function createCodingPlanResetHeaders(authorization: CodingPlanResetAuthorization, …) {
  const headers: Record<string, string> = {
    Authorization: authorization.zcodeAuthorization,        // Bearer <zcodejwttoken>
    "X-Bigmodel-Authorization": authorization.codingPlanAuthorization,  // raw OAuth token
  };
```

`zcode base` = `https://zcode.z.ai` (first-party endpoint table, §6.3).

### 2.7 `POST https://api.z.ai/api/auth/z/login` — OAuth → business token exchange

- Body `{"token": <oauth access token>}`, returns a business JWT.
- First-party source: `zai-org/ZCode` → `apps/zcode-cli/packages/adapters/src/auth/coding-plan-api-key.ts:21` (`const ZAI_API_HOST = "https://api.z.ai";`) and the call site in the same file:

```ts
businessTokens.resolve(oauth, () =>
  request(`${ZAI_API_HOST}/api/auth/z/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ token: oauth }),
  }),
)
```

- Relevance: it shows z.ai's own client converts an OAuth session token into a business token before calling business APIs — and the quota endpoint accepts **either** an API key or (per third-party report, §6.4) an OAuth token.

### 2.8 Endpoints confirmed NOT to exist

All probed live on 2026-11-05 against `https://api.z.ai` (and `open.bigmodel.cn` where noted):

| Path | Result |
| --- | --- |
| `/api/monitor/usage/quota` | 404 |
| `/api/monitor/usage/quota/detail` \| `/info` \| `/weekly` \| `/credit` \| `/limits` \| `/credit-limit` | 404 |
| `/api/monitor/usage/credit/limit` | 404 |
| `/api/monitor/usage/quota/limit?includeWeekly=true` | 200 (query param ignored — same payload) |
| `/api/biz/customer/balance`, `/api/biz/balance`, `/api/biz/user/balance` | `{"code":404,"msg":"接口不存在: …"}` |
| `/api/biz/subscription/usage`, `/api/biz/usage/list`, `/api/biz/subscription/quota`, `/api/biz/coding-plan/quota` | `{"code":404,"msg":"接口不存在: …"}` |
| `/api/biz/customer/getCustomerInfo` | `HTTP 403 APIKey not allow access` (both hosts) |
| `/api/v1/mcp/usage`, `/api/v1/coding-plan/reset` on `api.z.ai` | `{"code":500,"msg":"404 NOT_FOUND"}` — these live on the **zcode** base, not the model API base |
| `/api/v1/zcode-plan/billing/balance` on `api.z.ai` | `{"code":500,"msg":"404 NOT_FOUND"}` — zcode base only |

---

## 3. Response shape and field semantics

### 3.1 Envelope

`zai-org/ZCode` → `packages/services/src/usage-stats/providers/bigmodelUsageQuotaMapper.ts:25-50`:

```ts
export interface BigModelUsageQuotaEnvelope {
  code?: number; msg?: string; success?: boolean;
  data?: BigModelUsageQuotaPayload | null;
}
export interface BigModelUsageQuotaPayload {
  level?: string;                 // plan tier: "lite" | "pro" | "max" | …
  limits?: BigModelUsageQuotaLimitPayload[];
}
```

### 3.2 One limit entry

Same file `:52-64` (wire names) and `packages/shared/src/usage-quota.ts:14-40` (normalized type):

| Field | Meaning |
| --- | --- |
| `type` | `TOKENS_LIMIT` \| `CREDIT_LIMIT` (equivalent; z.ai team plan uses `CREDIT_LIMIT`) \| `TIME_LIMIT` (monthly tool pool) |
| `unit`, `number` | **window identity** — see §3.3 |
| `usage` | absolute limit for the window (present on some entries; **absent** on the 5-hour entry for legacy prompt-based plans) |
| `currentValue` | absolute consumed |
| `remaining` | absolute remaining |
| `percentage` | **percentage used** (0–100) — see §3.4 |
| `nextResetTime` | **epoch milliseconds** of the next reset — see §3.5 |
| `usageDetails[]` | per-MCP-server breakdown: `{ modelCode, displayName?, usage }` (e.g. `search-prime`, `web-reader`, `zread`) |

### 3.3 Window identity — `unit` / `number`

First-party statement, `zai-org/ZCode` → `packages/ui/src/lib/codingPlanQuotaPresentation.ts:5-20`:

```
 * Token / Credit 类配额的等价 type 集合。
 *
 * zai 业务后端 Team Plan 的 quota/limit 用 `CREDIT_LIMIT` 作为 type，
 * bigmodel 业务后端用 `TOKENS_LIMIT`。两者 unit/number 语义完全一致
 *（unit=3,number=5 → 5 小时窗口；unit=6 → 每周），仅 type 枚举命名不同。
```

and `:18-20`: *"目前 bigmodel/zai 后端都用 `TIME_LIMIT` 表示每月工具调用配额"* (both backends use `TIME_LIMIT` for the monthly tool-call quota).

Lookup helper, same file `:36-52`:

```ts
export function findCodingPlanQuotaLimit(limits, type, unit, number?) {
  return limits?.find(limit =>
    isSameLimitCategory(limit.type, type) &&
    limit.unit === unit &&
    (number == null || limit.number === number)) ?? null;
}
```

Desktop app usage (minified bundle, same semantics): `/Applications/ZCode.app/Contents/Resources/app.asar` contains
`var gEt=new Set([`TOKENS_LIMIT`,`CREDIT_LIMIT`]),_Et=new Set([`TIME_LIMIT`]);` and the 5-hour / weekly lookups
`_R(n,`TOKENS_LIMIT`,3,5)` (5-hour) and `_R(n,`TOKENS_LIMIT`,6)` (weekly), plus `S=_R(y,`TIME_LIMIT`,5,1)` (monthly MCP).

**Window table (from first-party code + live observation):**

| Window | `type` | `unit` | `number` | Reset semantics |
| --- | --- | --- | --- | --- |
| 5-hour prompt/token pool | `TOKENS_LIMIT` (or `CREDIT_LIMIT`) | `3` | `5` | dynamically refreshed; resets 5 h after consumption |
| Weekly pool | `TOKENS_LIMIT` (or `CREDIT_LIMIT`) | `6` | `7` per third-party ([§6.4](#64-third-party-cross-check)); first-party code only pins `unit=6` | activated at purchase, resets every 7 days |
| Monthly MCP / tool pool | `TIME_LIMIT` | `5` | `1` | monthly, boundary at plan renewal |

### 3.4 `percentage` is **used**, not remaining

- First-party, `zai-org/ZCode` → `packages/shared/src/usage-quota.ts:66-71`:

  ```
  /** MCP 额度所属的 Coding Plan 连接 … */
  * percentage 沿用 quota 接口口径：**已使用占比**，展示端负责反转成剩余。
  ```
  → *"percentage follows the quota API convention: **share already used**; the display layer inverts it to remaining."*
- UI helper `packages/ui/src/lib/codingPlanQuotaPresentation.ts:67-80` provides `normalizeQuotaPercentage` / `getQuotaRemainingPercentage` (100 − used).

### 3.5 `nextResetTime` is epoch **milliseconds**

- `packages/shared/src/usage-quota.ts:19-20`: *"周期时间为毫秒，供提醒去重"* (cycle time is in milliseconds).
- The MCP endpoint variant is in **seconds** and is explicitly converted: `zcodeMcpQuotaProvider.ts:301` → `nextResetTime: data.next_refresh_at * 1000`.
- Live sanity check (this account, 2026-11-05):
  - 5-hour pool `nextResetTime = 1793912822042` → `2026-11-05T21:07:02Z` (today, ~1 h out) ✔ consistent with a rolling 5-hour window.
  - monthly MCP `nextResetTime = 1794310692975` → `2026-11-10T11:38:12Z` (5 days out) ✔ consistent with a monthly pool.
- The third-party doc (§6.4) claims the monthly `TIME_LIMIT` has *no* `nextResetTime`; **contradicted by the live payload**, which carries one. Treat "no reset time for the MCP pool" as wrong for this account.

---

## 4. Live captures (this machine, this account) — redacted

`GET https://api.z.ai/api/monitor/usage/quota/limit` with a local Coding Plan API key, 2026-11-05:

```json
{
  "code": 200,
  "msg": "Operation successful",
  "data": {
    "limits": [
      {
        "type": "TIME_LIMIT", "unit": 5, "number": 1,
        "usage": 100, "currentValue": 38, "remaining": 62,
        "percentage": 38,
        "nextResetTime": 1794310692975,
        "usageDetails": [
          {"modelCode": "search-prime", "usage": 38},
          {"modelCode": "web-reader",  "usage": 0},
          {"modelCode": "zread",       "usage": 0}
        ]
      },
      {
        "type": "TOKENS_LIMIT", "unit": 3, "number": 5,
        "percentage": 69,
        "nextResetTime": 1793912822042
      }
    ],
    "level": "lite"
  },
  "success": true
}
```

(`percentage` for the 5-hour entry drifted 69 → 72 across calls minutes apart; it is a live counter.)

**Rendering through z.ai's own plugin** (executed locally from `zai-org/zai-coding-plugins`, `query-usage.mjs`):

```
Quota limit data:
{"limits":[{"type":"MCP usage(1 Month)","percentage":38,"currentUsage":38,"totol":100,
  "usageDetails":[{"modelCode":"search-prime","usage":38},{"modelCode":"web-reader","usage":0},{"modelCode":"zread","usage":0}]},
 {"type":"Token usage(5 Hour)","percentage":71}],"level":"lite"}
```

Note the plugin's own renaming logic (`query-usage.mjs:87-109`): `TOKENS_LIMIT` → `"Token usage(5 Hour)"`, `TIME_LIMIT` → `"MCP usage(1 Month)"`.

### 4.1 The **weekly** entry was absent for this account

- Four different locally-held z.ai keys were tested against `quota/limit`; **all four returned the identical 2-entry payload** (5-hour + monthly MCP) — **no `unit=6` entry**.
- [INFERENCE] This account is on a **legacy prompt-based plan** (its subscriptions are dated 2026-02-10 and 2026-04-30, i.e. before the 2026-07-30 credits switch — <https://docs.z.ai/devpack/notice/usage-revision.md>), and the legacy backend reports only the 5-hour percentage (no `usage`/`currentValue`/`remaining` on that entry, unlike the credits-based example in §6.4) plus the monthly MCP pool. A credits-based (Lite/Pro/Max, post-2026-07-30) account is expected to also expose `unit=6`.
- [UNVERIFIED] The exact condition under which the weekly entry appears (plan generation, key type, subscription age) was not determinable from this machine.

---

## 5. Where z.ai credentials live locally

### 5.1 Environment variables

| Variable | Who defines it | Notes |
| --- | --- | --- |
| `ANTHROPIC_AUTH_TOKEN` + `ANTHROPIC_BASE_URL` | **z.ai official docs** (<https://docs.z.ai/devpack/tool/claude.md:95-96,113-114>: `"ANTHROPIC_AUTH_TOKEN": "your_zai_api_key"`, `"ANTHROPIC_BASE_URL": "https://api.z.ai/api/anthropic"`) **and** z.ai's own plugin (`query-usage.mjs:12-13`) | This is the credential pair the quota script consumes. |
| `ZAI_API_KEY` | z.ai's **own shipped npm package** `@z_ai/coding-helper` writes it into tool configs (`package/dist/lib/claude-code-manager.js:258`, `codex-manager.js:234`, `crush-manager.js:136`, `factory-droid-manager.js:194`, `mcp-manager.js:169`, `opencode-manager.js:156` → `env.Z_AI_API_KEY = apiKey;`) | Also the variable used by the third-party `openusage` provider (`ZAI_API_KEY`, fallback `GLM_API_KEY`). Present on this machine (§7). |
| `Z_AI_API_KEY` | same npm package (MCP servers) | |
| `ZCODE_BIGMODEL_USAGE_API_KEY`, `BIGMODEL_USAGE_API_KEY` | **z.ai's own ZCode source**: `packages/services/src/usage-stats/providers/bigmodelUsageQuotaProvider.ts:843-845` | Explicit API-key override for the usage/quota endpoints. |
| `ZCODE_BIGMODEL_USAGE_QUOTA_URL`, `BIGMODEL_USAGE_QUOTA_URL` | same file `:1661-1663` | Override for the quota URL (base + path). |
| `ZCODE_CREDENTIAL_SECRET` | same repo, `apps/zcode-cli/packages/adapters/src/auth/credential-cipher.ts:8` | Key material for decrypting `credentials.json`; falls back to a machine-derived string if unset. |
| `ZCODE_DATA_BASE_DIR` | same repo, `.../shared-credentials.ts:288` | Overrides the base dir for `~/.zcode/v2/...`. |

### 5.2 Config files

| Path | Shape | Notes |
| --- | --- | --- |
| `~/.claude/settings.json` | `{"env": {"ANTHROPIC_AUTH_TOKEN": "<key>", "ANTHROPIC_BASE_URL": "https://api.z.ai/api/anthropic", "ANTHROPIC_DEFAULT_HAIKU_MODEL": "glm-5.3-flash[1m]", …}}` | Claude Code's own config; z.ai's docs tell users to put the key here. Present on this machine (§7). |
| `~/.claude.json` | Claude Code's project/global state; on this machine it holds **`lastModelUsage` entries for `glm-5.3` / `glm-5.3-flash[1m]`** but **no z.ai provider entry / no API key** | i.e. the widget can find *evidence of z.ai usage* here, but not credentials. |
| `~/.zcode/v2/credentials.json` | flat JSON map, every value `"enc:v1:<iv>.<tag>.<ciphertext>"` (AES-256-GCM, base64url) | Keys: `oauth:active_provider`, `oauth:zai:access_token`, `oauth:zai:refresh_token`, `oauth:zai:user_info`, `oauth:bigmodel:access_token`, `oauth:bigmodel:refresh_token`, `oauth:bigmodel:user_info`, `zcodejwttoken`, plus (desktop) `account-provider:coding-plan:account:<provider>:account:<uuid>:api-key` and `project-key-location:v1:<url-encoded tuple>`. |
| `~/.zcode/v2/provider_config.json` | `{"config": {"providerConfigRules": {"providerRules": [{providerId, providerName, config:{access:{type,apiKey}, api:{baseUrl,type}, …}}]}}}` | Plaintext API keys for *user-added* providers (e.g. the MiniMax provider on this machine); the built-in z.ai coding plan is OAuth-based and stored encrypted in `credentials.json`. |
| `~/.zcode/v2/setting.json` | `{"modelProviderFamilyModes": {"zai": "oauth"}, "modelProviderFamilySelectedKeys": {"zai": "coding-plan:builtin:zai-coding-plan"}, "providerFamilyConnectionSelections": {"zai": {"kind": "individual-coding-plan"}}}` | Tells you the account is connected to an **individual coding plan** (OAuth) rather than an API key. |
| `~/.zcode/v2/coding-plan-cache.json` | `{"version":1,"entryStatus":{"items":{"builtin:zai-coding-plan":{"status":"available"}, "builtin:bigmodel-coding-plan":{"status":"unavailable","reason":"coding_plan_not_connected"}}}}` | Cheap local signal of which plan families are connected — no network needed. |
| `~/.config/openusage/zai.json` (third-party tool) | `{"apiKey": "<49-char key>"}` | Not z.ai's format; ignore for z.ai-first-party discovery, but it is a valid z.ai key. |
| `~/.config/glm-acp-agent/credentials.json` (third-party tool) | `{"z_ai_api_key": "<49-char key>"}` | Same. |

**ZCode credential paths are first-party-defined:**
`zai-org/ZCode` → `apps/zcode-cli/packages/adapters/src/auth/shared-credentials.ts:288-289`:

```ts
const baseDir = options.baseDir ?? env[ZCODE_DATA_BASE_DIR_ENV_KEY] ?? homedir();
return join(resolveUserPath(baseDir), ".zcode", "v2", "credentials.json");
```

and the key names, same file `:15-24`:

```ts
export const SHARED_ZCODE_CREDENTIAL_KEYS = {
  activeProvider: "oauth:active_provider",
  bigmodelAccessToken: "oauth:bigmodel:access_token",
  bigmodelRefreshToken: "oauth:bigmodel:refresh_token",
  bigmodelUserInfo: "oauth:bigmodel:user_info",
  zaiAccessToken: "oauth:zai:access_token",
  zaiRefreshToken: "oauth:zai:refresh_token",
  zaiUserInfo: "oauth:zai:user_info",
  zcodeJwtToken: "zcodejwttoken",
} as const;
```

### 5.3 Encryption — **not** the OS keychain

`zai-org/ZCode` → `apps/zcode-cli/packages/adapters/src/auth/credential-cipher.ts:4-8`:

```ts
const ENCRYPTED_VALUE_PREFIX = "enc:v1:";
const CREDENTIAL_CIPHER_ALGORITHM = "aes-256-gcm";
const CREDENTIAL_CIPHER_IV_BYTES = 12;
const CREDENTIAL_CIPHER_AUTH_TAG_BYTES = 16;
const CREDENTIAL_SECRET_ENV_KEY = "ZCODE_CREDENTIAL_SECRET";
```

and `:92-101`:

```ts
function deriveCipherKey(secret: string): Buffer {
  return createHash("sha256").update(secret).digest();
}
function resolveCredentialSecret(env): string {
  const configuredSecret = env[CREDENTIAL_SECRET_ENV_KEY]?.trim();
  if (configuredSecret) return configuredSecret;
  …
  return `zcode-credential-fallback:${platform()}:${homedir()}:${username}`;
}
```

→ The key is `sha256(ZCODE_CREDENTIAL_SECRET)` or a **machine-derived fallback** (`platform:home:username`). AES-256-GCM with a 12-byte IV and 16-byte auth tag, values stored as `enc:v1:<iv>.<tag>.<ct>` (base64url).

**Practical consequence for a widget:** the OAuth access token in `~/.zcode/v2/credentials.json` can be decrypted **locally, without any secret** (fallback secret is reproducible on the same machine). A CLI that only wants the *quota* does not need it — the plain API key suffices — but it is a viable path for OAuth-only users.

### 5.4 Keychain

- **No macOS Keychain usage by z.ai or ZCode.** Verified on this machine:
  - `security find-generic-password -s zai|z.ai|zhipu|glm|zcode|ZCode` → *"The specified item could not be found in the keychain"* for every service.
  - `security dump-keychain | grep -iE "zcode|zai|zhipu|glm"` → no matches.
- The Electron app's own generic storage (`~/Library/Application Support/ZCode/session/`) contains `Cookies`, `Local Storage`, etc., but no z.ai secret material was found there.

### 5.5 Does z.ai ship an official CLI / IDE plugin? — Yes

| Artifact | Where | Relevant content |
| --- | --- | --- |
| **ZCode CLI** (`zcode`, v0.17.1) | shipped inside `/Applications/ZCode.app/Contents/Resources/glm/zcode.cjs`; public source at `github.com/zai-org/ZCode` (`apps/zcode-cli/**`) | `zcode login [zai\|bigmodel]` — *"Sign in through browser authorization"*; `zcode logout` — *"Remove the shared Z.AI login credentials"*. This is the OAuth path. |
| **ZCode desktop app** | `/Applications/ZCode.app` (Electron; `app.asar` is minified) | the Usage Stats UI that renders the same quota data (§3.3) |
| **Claude Code plugin** `glm-plan-usage` | `github.com/zai-org/zai-coding-plugins` → `plugins/glm-plan-usage/` | **the quota query script itself** (§2.1) |
| **Plugin marketplace** | `github.com/zai-org/zai-coding-plugins` → `.claude-plugin/marketplace.json` (plugins: `glm-plan-usage`, `glm-plan-bug`) | install via `claude plugin marketplace add zai-org/zai-coding-plugins` |
| **`@z_ai/coding-helper`** (npm, v0.1.1) | first-party; installs the plugin + writes provider config into Claude Code / Codex / OpenCode / Crush / Factory Droid / MCP | writes `Z_AI_API_KEY` into tool envs; writes `~/.claude/settings.json`, `~/.claude.json`, `~/.config/opencode/opencode.json`, `~/.config/crush/crush.json` |
| **VS Code / JetBrains plugin** | **none found.** `github.com/zai-org` (53 repos) contains only `zai-coding-plugins` and `zcode-plugins` as tool repos; no VS Code / JetBrains extension repository | [UNVERIFIED] — an extension may exist outside the `zai-org` org |

Other first-party GitHub repos in `zai-org`: `zai-coding-plugins`, `zcode-plugins` (Python; "ZCode Plugins Marketplace - official built-in and community plugins for ZCode"), `ZCode` (TypeScript; "Z.ai's coding agent harness").

---

## 6. First-party code that queries a quota endpoint

### 6.1 The Claude Code plugin (exact file + lines)

`github.com/zai-org/zai-coding-plugins` → `plugins/glm-plan-usage/skills/usage-query-skill/scripts/query-usage.mjs`

- `:44-53` — endpoint construction per platform (quoted in §2.1).
- `:12-13` — credential source: `const baseUrl = process.env.ANTHROPIC_BASE_URL || ''; const authToken = process.env.ANTHROPIC_AUTH_TOKEN || '';`
- `:111-124` — HTTP call, `Authorization: authToken` raw.
- `:87-109` — window labelling (`TOKENS_LIMIT` → "Token usage(5 Hour)", `TIME_LIMIT` → "MCP usage(1 Month)").
- `:169` — the three calls: `queryUsage(modelUsageUrl, 'Model usage'); queryUsage(toolUsageUrl, 'Tool usage'); queryUsage(quotaLimitUrl, 'Quota limit', false, processQuotaLimit);`
- Plugin manifest `plugins/glm-plan-usage/.claude-plugin/plugin.json` — author `chao.gong@z.ai` (a z.ai address) — confirms first-party status.
- Docs page: <https://docs.z.ai/devpack/extension/usage-query-plugin> — *"The `glm-plan-usage` plugin allows you to query your current quota and usage statistics for the GLM Coding Plan directly within your Claude Code. This plugin is only available for the Personal plan."*

### 6.2 The ZCode agent harness (exact file + lines)

`github.com/zai-org/ZCode`

| File | Line(s) | Content |
| --- | --- | --- |
| `packages/services/src/usage-stats/providers/bigmodelUsageQuotaProvider.ts` | 73 | `const BIGMODEL_QUOTA_PATH = "/api/monitor/usage/quota/limit";` |
| same | 1151-1157 | header construction + the "no Bearer prefix" rule |
| same | 1564-1580 | `buildUsageMonitorUrl` → `model-usage` / `tool-usage` |
| same | 1582-1599 | `buildCreditUsageMonitorUrl` → `credit-usage/activity`, `credit-usage/usage-detail` |
| same | 1602-1611 | `buildModelPerformanceMonitorUrl` → `model-performance-day` |
| same | 1661-1691 | env overrides + `buildZaiQuotaUrl` / `buildBigModelQuotaUrl` |
| same | 842-857 | `ZCODE_BIGMODEL_USAGE_API_KEY` / `BIGMODEL_USAGE_API_KEY` env override |
| same | 1171-1174 | Team Plan `?type=2` routing |
| `packages/ui/src/lib/codingPlanQuotaPresentation.ts` | 5-20, 36-52 | `unit=3,number=5` ⇒ 5-hour; `unit=6` ⇒ weekly; `TIME_LIMIT` ⇒ monthly tool pool; `TOKENS_LIMIT`/`CREDIT_LIMIT` equivalence |
| `packages/shared/src/usage-quota.ts` | 14-40, 66-71 | limit field set; `percentage` = used share; `nextResetTime` in ms |
| `packages/services/src/usage-stats/providers/bigmodelUsageQuotaMapper.ts` | 11-23, 25-64, 100-124 | subscription item fields; quota envelope + limit payload fields; subscription picking |
| `packages/services/src/usage-stats/providers/zcodeMcpQuotaProvider.ts` | 1, 24, 46-57, 210, 299-311 | `GET /api/v1/mcp/usage`, `total_usage`, `next_refresh_at` (seconds → ms) |
| `packages/services/src/coding-plan-subscription/zaiCodingPlanSubscriptionProvider.ts` | 20-52 | z.ai business host + `oauth:zai:access_token` + `/api/biz`, `/api/pay` families |
| `apps/zcode-cli/packages/adapters/src/auth/shared-credentials.ts` | 15-24, 288-289 | credential key names; `~/.zcode/v2/credentials.json` path |
| `apps/zcode-cli/packages/adapters/src/auth/credential-cipher.ts` | 4-8, 92-101 | AES-256-GCM `enc:v1:` format, key derivation |
| `apps/zcode-cli/packages/adapters/src/auth/coding-plan-api-key.ts` | 21, `POST /api/auth/z/login` call | `https://api.z.ai`, OAuth→business-token exchange |

### 6.3 The ZCode desktop bundle (installed locally, minified)

`/Applications/ZCode.app/Contents/Resources/app.asar` — the only occurrence of the monitor path in the whole bundle:

```js
var iOe="/api/monitor/usage/quota/limit",
    FC="/api/v1/coding-plan/reset",
    sOe="zcodejwttoken",
    aOe="oauth:zai:access_token",
    cOe="oauth:bigmodel:access_token",
```

plus the credential-key list:

```js
var Oxe="credentials.json",
  Vp=["oauth:active_provider","oauth:zai:access_token","oauth:zai:refresh_token","oauth:zai:user_info",
      "oauth:bigmodel:access_token","oauth:bigmodel:refresh_token","oauth:bigmodel:user_info","zcodejwttoken"];
```

plus the ZCode endpoint table (`buildZCodeEndpointUrls`):

```js
zcodePlanOpenAiBaseUrl:`${r}/api/v1/zcode-plan`,
zcodePlanAnthropicBaseUrl:`${r}/api/v1/zcode-plan/anthropic`,
zcodePlanBillingCurrentUrl:`${r}/api/v1/zcode-plan/billing/current`,
zcodePlanBillingBalanceUrl:`${r}/api/v1/zcode-plan/billing/balance`
```

and the window lookups: `var gEt=new Set([`TOKENS_LIMIT`,`CREDIT_LIMIT`]),_Et=new Set([`TIME_LIMIT`]);` … `_R(n,`TOKENS_LIMIT`,3,5)` (5-hour) … `_R(n,`TOKENS_LIMIT`,6)` (weekly) … `_R(y,`TIME_LIMIT`,5,1)` (monthly MCP).

Also in `/Applications/ZCode.app/Contents/Resources/config/provider/zcode-builtin.json` — the built-in provider template:

```json
{"templateId":"zai-api","templateNameMap":{"en-US":"Z.ai Coding Plan"},
 "config":{"access":{"type":"zhipu-coding-plan-api-key","apiKeyManagementUrl":"https://z.ai/manage-apikey/apikey-list"},
           "api":{"type":"anthropic-messages","baseUrl":"https://api.z.ai/api/anthropic"},
           "builtinModelIds":["GLM-5.3","GLM-5.3-Flash"]}}
```

and the standard-API template `zai-standard-api` → `baseUrl: https://api.z.ai/api/paas/v4`.

### 6.4 Third-party cross-check (not first-party — used only to corroborate)

`github.com/openusage-community/openusage` → `docs/providers/zai.md` and `plugins/zai/plugin.js` (community project, not z.ai):

- *"These API endpoints are not documented in Z.ai's public API reference. They are used internally by the subscription management UI and work with both OAuth tokens and API keys."* — consistent with §2.1 (raw API key) and §2.7 (OAuth exchange).
- Their weekly detection: `plugins/zai/plugin.js:142` → `const weeklyTokenLimit = findLimit(limits, "TOKENS_LIMIT", 6);` — i.e. **weekly is read from the same `limits[]` array**, and is simply skipped when absent (`:143-158`). Their doc adds "`unit: 6, number: 7` — 7-day rolling period (weekly)"; the `number=7` detail is **third-party only** — first-party code pins `unit=6` and leaves `number` optional.
- They send `Authorization: Bearer <api_key>`; z.ai's own code says not to. **Both work live** (verified 2026-11-05: raw and `Bearer` forms returned identical payloads) — so z.ai's backend accepts either, but first-party code should copy the raw form.
- Their claim that the monthly `TIME_LIMIT` has **no** `nextResetTime` is contradicted by the live payload (§3.5).

---

## 7. This machine's actual z.ai state (values redacted)

**Present:**

| Path | What | Detail |
| --- | --- | --- |
| `~/.zshrc:232` | `export ZAI_API_KEY="<49-char key>"` | the only z.ai credential written by hand in a shell rc file; source of the `ZAI_API_KEY` env var observed in this session |
| `~/.claude/settings.json` | `env.ANTHROPIC_AUTH_TOKEN` (49 chars) + `env.ANTHROPIC_BASE_URL = "https://api.z.ai/api/anthropic"` + `ANTHROPIC_DEFAULT_{HAIKU,OPUS,SONNET}_MODEL = glm-5.3*` | Claude Code pointed at z.ai |
| `~/.claude.json` | `lastModelUsage` entries for `glm-5.3` and `glm-5.3-flash[1m]` (input/output/cache tokens + `costUSD`) | **no** z.ai provider entry, **no** key |
| `~/.zcode/v2/credentials.json` | 12 encrypted entries (`enc:v1:…`), incl. `oauth:zai:access_token`, `zcodejwttoken`, `oauth:active_provider`, `account-provider:coding-plan:account:zai-individual-coding-plan:account:<uuid>:api-key`, `project-key-location:v1:["https://api.z.ai","zai","<uuid>",null,1,"non-team"]` | ZCode is **logged in to z.ai** |
| `~/.zcode/v2/provider_config.json` | providers `Minimax` (`https://api.minimax.io/anthropic`), `account:zai-start-plan`, `new-provider` (`https://voidllm.escandell.cat/v1`) | no plaintext z.ai key |
| `~/.zcode/v2/setting.json` | `modelProviderFamilyModes.zai = "oauth"`, `providerFamilyConnectionSelections.zai = {"kind":"individual-coding-plan"}` | z.ai connected via OAuth |
| `~/.zcode/v2/coding-plan-cache.json` | `builtin:zai-coding-plan → available`; `builtin:bigmodel-coding-plan → unavailable (coding_plan_not_connected)` | |
| `~/.config/openusage/zai.json` | `{"apiKey": "<49-char key>"}` | third-party tool |
| `~/.config/glm-acp-agent/credentials.json` | `{"z_ai_api_key": "<49-char key>"}` | third-party tool (`glm-acp-agent` v1.6.1 by Stefan de Vogelaere) |
| `/Applications/ZCode.app` | ZCode desktop + `glm/zcode.cjs` CLI (v0.17.1) | first-party |
| `~/.codex/shell_snapshots/01a19303-….sh` | shell snapshot containing the z.ai env | incidental |

**Absent:** `~/.z.ai`, `~/.zhipuai`, `~/.config/zai`, `~/.glm` (all "No such file or directory"). No z.ai entries in the macOS keychain. No `Z_AI_API_KEY` / `GLM_API_KEY` in `~/.zshrc`, `~/.zprofile`, `~/.zshenv`, `~/.bashrc`, `~/.bash_profile`.

**Key fingerprints** (sha256[:12] of the value; values not shown) — four **distinct** 49-char z.ai keys are present, all belonging to the same account (all four return the identical quota payload):

| Source | len | sha256[:12] | prefix |
| --- | --- | --- | --- |
| `env ZAI_API_KEY` (`~/.zshrc`) | 49 | `522b077e5687` | `9f11` |
| `~/.claude/settings.json` `ANTHROPIC_AUTH_TOKEN` | 49 | `df2699b750f8` | `cd1e` |
| `~/.config/openusage/zai.json` `apiKey` | 49 | `f86b5da8d8f6` | `ce0b` |
| `~/.config/glm-acp-agent/credentials.json` `z_ai_api_key` | 49 | `a66569a14cc6` | `e6cf` |

**Account facts** (from `GET /api/biz/subscription/list`): two active `GLM Coding Lite` subscriptions (one annual, one monthly), plan tier reported by `quota/limit` as `level: "lite"`.

---

## 8. What remains unknown

1. **When does the weekly window appear?** The `unit=6` entry was absent for all four local keys on a legacy-plan account. Unverified: whether credits-based (post-2026-07-30) plans expose it, whether a *Standard* (pay-as-you-go) key exposes it, and whether OAuth-token auth returns a richer `limits[]` than an API key. (A third-party project claims weekly works with plain API keys; unconfirmed here.)
2. **Does `unit=6` come with `number=7`?** First-party code only constrains `unit`; `number=7` is a third-party claim. Not observable on this account.
3. **Absolute limits (`usage`/`currentValue`/`remaining`) for the 5-hour pool** were absent on this legacy account; the shape of a credits-based account's 5-hour entry (token counts vs. credit counts) is unverified.
4. **Does a Standard/PAYG API key return a quota payload at all**, and with what `level`? Unverified — all local keys are Coding Plan keys.
5. **`/api/monitor/usage/credit-usage/activity|usage-detail` and `/api/monitor/usage/model-performance-day`** — paths are first-party-confirmed but responses were never captured live.
6. **Team Plan specifics** — `?type=2` routing, `CREDIT_LIMIT` type, `X-Bigmodel-Authorization` project-token headers, and the team-project key format are read from source but not exercised live.
7. **Pay-as-you-go balance** — no API-key-reachable endpoint was found; `/api/biz/customer/getCustomerInfo` is 403 for API keys. Whether an OAuth/session token unlocks it (and whether a *balance* can be expressed as a %/reset at all) is unverified.
8. **Whether an official VS Code / JetBrains extension exists** outside the `zai-org` GitHub org — none found; not searched exhaustively on the marketplaces.
9. **Monthly MCP pool reset semantics** — live `nextResetTime` was 5 days out, while the plan's `nextRenewTime` was ~4 months out; the actual rule (1st of month vs. renewal date vs. rolling 30 days) is unverified. The third-party "resets on the 1st at 00:00 UTC" claim does not match the live value.
10. **Whether the quota endpoint tolerates `Bearer`** — it does today (verified), but first-party code explicitly says not to use it; the backend's long-term behaviour is unknown.
11. **Rate limits / caching on the quota endpoint** — not documented, not measured.

---

## 9. Source index

**First-party (z.ai)**
- <https://docs.z.ai/llms.txt> — full docs index (no quota/usage page exists)
- <https://docs.z.ai/openapi.json> — public OpenAPI spec (14 paths, no quota path)
- <https://docs.z.ai/devpack/overview.md> — plan tiers, 5-hour + weekly credits, credit multipliers
- <https://docs.z.ai/devpack/faq.md> — weekly quota rule, quota reset cards, supported-tool restriction
- <https://docs.z.ai/devpack/notice/api-key-management.md> — Standard vs Coding Plan vs Team keys and billing
- <https://docs.z.ai/devpack/notice/usage-revision.md> — legacy vs credits-based plans (5-hour + weekly, prompt vs credit)
- <https://docs.z.ai/devpack/extension/usage-query-plugin> — the `glm-plan-usage` plugin
- <https://docs.z.ai/devpack/tool/claude.md> — `ANTHROPIC_AUTH_TOKEN` / `ANTHROPIC_BASE_URL` for Claude Code
- <https://zcode.z.ai/en/docs/usage-stats> — ZCode Usage Stats: 5-hour pool, weekly quota, monthly MCP quota
- <https://github.com/zai-org/zai-coding-plugins> — plugin marketplace; `plugins/glm-plan-usage/skills/usage-query-skill/scripts/query-usage.mjs`
- <https://github.com/zai-org/ZCode> — ZCode agent harness source (paths cited in §6.2)
- <https://github.com/zai-org/zcode-plugins> — ZCode plugin marketplace
- `https://www.npmjs.com/package/@z_ai/coding-helper` (v0.1.1) — writes `Z_AI_API_KEY` into tool configs
- `/Applications/ZCode.app/Contents/Resources/app.asar`, `/Applications/ZCode.app/Contents/Resources/config/provider/zcode-builtin.json`, `/Applications/ZCode.app/Contents/Resources/glm/zcode.cjs` — locally installed first-party binaries

**Third-party (corroboration only)**
- <https://github.com/openusage-community/openusage> → `docs/providers/zai.md`, `plugins/zai/plugin.js` — same endpoint, `Bearer` auth, weekly lookup by `unit=6`

**Local paths referenced**
- `/Users/javi/.zshrc` (line 232), `/Users/javi/.claude/settings.json`, `/Users/javi/.claude.json`
- `/Users/javi/.zcode/v2/{credentials.json,provider_config.json,setting.json,coding-plan-cache.json,config.json}`
- `/Users/javi/.config/openusage/zai.json`, `/Users/javi/.config/glm-acp-agent/credentials.json`
- `/opt/homebrew/lib/node_modules/glm-acp-agent` (third-party CLI, uses `Z_AI_API_KEY`)
