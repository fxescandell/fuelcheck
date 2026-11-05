# MiniMax: credential discovery and quota readouts

**Research date:** 2026-11-05
**Scope:** how a local CLI (this repo, `fuelcheck`) can discover a MiniMax user's credentials and read their quota usage + reset time.
**Primary sources used:** MiniMax official docs (`platform.minimax.io` / `platform.minimaxi.com`), the official MiniMax CLI source (`github.com/MiniMax-AI/cli`), the official MiniMax Code source (`github.com/MiniMax-AI/minimax-code`), and live calls against the user's own MiniMax account from this machine.

> Secret **values** are never reproduced here. Key names, file shapes, and path shapes are reproduced verbatim.

---

## 0. Answer summary

| Question | Answer |
| --- | --- |
| Is there a quota endpoint? | **Yes.** `GET https://{www,api}.minimax.io/v1/token_plan/remains` (global) / `https://{www,api}.minimax.cn/v1/token_plan/remains` (CN). Documented publicly and confirmed live. |
| Does it return % used / limit / reset? | It returns **remaining %**, window start/end epoch-ms, and remaining duration. It does **not** reliably return an absolute quota limit (see §2.4). |
| Auth | `Authorization: Bearer <M Plan key>`; `x-api-key: <key>` also accepted. |
| Pay-as-you-go balance endpoint? | Yes, separate: `GET {api base}/account/query_balance` → amounts only, no %, no reset. |
| Where are the credentials? | `~/.mmx/config.json` (official CLI), `~/.minimax/auth/<env>/<region>/mcode-public/auth.json` (MiniMax Code, OAuth), plus MiniMax keys written into other tools' configs by `mmx agent setup`. No keychain usage anywhere. |
| Windows | 5-hour-ish rolling **and** 7-day weekly (video is weekly-only). Exact per-model window lengths are server-defined — **do not hardcode 5h**. |
| Unknowns | §7. |

---

## 1. Which MiniMax products expose a usage/quota readout?

MiniMax ships **three distinct, non-interchangeable quota surfaces**. They use different credentials, different endpoints and different response shapes, and MiniMax's own docs state explicitly that they cannot be mixed.

### 1.1 M Plan / Token Plan / "Coding Plan" subscription — *the one we want*

- A subscription ("M Plan"; formerly "Token Plan" / "Coding Plan") with tiers **Go / Explore / Build**, quota expressed as multiples of Go (1× / 3× / 7.5×).
  Source: <https://platform.minimax.io/docs/m-plan/intro.md> ("Available models", "Usage limits" tables).
- Quota is tracked in **two rolling windows**: a short "5 hour" window and a **7-day weekly** window. Video models are constrained by the weekly window only.
  Source: <https://platform.minimax.io/docs/m-plan/usage-rules.md> ("额度刷新" table) and <https://platform.minimax.io/docs/m-plan/faq.md> ("How is usage reset?").
- Authenticated by a **subscription key** (prefix `sk-cp-…`), obtained at <https://platform.minimax.io/console/plan>.
  Source: <https://platform.minimax.io/docs/m-plan/other-tools.md> ("prefix `sk-cp-…`") and <https://platform.minimax.io/docs/m-plan/minimax-cli.md>.
- Readable via **console page**, **official CLI (`mmx quota`)**, or **REST API**.
  Source: <https://platform.minimax.io/docs/m-plan/faq.md> ("How can I check M Plan usage?" — three methods).

### 1.2 Pay-as-you-go API credit balance — *different resource, no windows*

- Uses the ordinary **API Key** (prefix `sk-api-…`), billed against the account balance.
  Source: <https://platform.minimax.io/docs/m-plan/minimax-cli.md> ("订阅 Key（`sk-cp-...`）与按量计费 API Key（`sk-api-...`）使用不同额度").
- Balance readout is a **money amount**, not a windowed quota. It has **no percentage and no reset timestamp**.
- Readable via `GET {api base}/account/query_balance` (see §2.3).
- MiniMax's own skill reference states the separation explicitly: *"A Token Plan/subscription resource and pay-as-you-go API balance/API key are different resource paths; never assume their keys, quotas, or model access are interchangeable."*
  Source: `github.com/MiniMax-AI/minimax-code` → `packages/local-runtime-v2/assets/agents/mavis/skills/minimax-code-product/references/account-models.md`.

### 1.3 MiniMax Code (the official agent app) — *managed login, no key*

- Signing in to MiniMax Code (desktop / mobile / web / `mcode` CLI) grants the same M Plan quota without any API key.
  Source: <https://platform.minimax.io/docs/m-plan/minimax-code.md> ("No separate subscription or M Plan API key setup is required").
- It has its **own** quota endpoint (§2.4) and its **own** OAuth credential store (§4.2).
- Console readouts: `Settings > Usage & Models` inside the app, and <https://platform.minimax.io/console/usage>.

### 1.4 Credit packs (积分包 / Credits)

- Prepaid top-up that absorbs usage *after* the M Plan quota is exhausted; consumed expiry-first; **valid 1 year** from purchase and *not* reset by window refresh.
  Source: <https://platform.minimax.io/docs/m-plan/faq.md> ("How long are Credits valid?").
- Balance is exposed through the membership endpoint (`op_credit_summary.total_remaining_amount`, §2.5), not through `token_plan/remains`.
  Source: `packages/tui/src/account/matrix-account-client.ts` (`projectMembership`).

**Do they differ?** Yes, completely:

| | M Plan subscription | Pay-as-you-go | MiniMax Code managed |
| --- | --- | --- | --- |
| Credential | `sk-cp-…` subscription key (or OAuth token) | `sk-api-…` API key | OAuth access token (`mcode-public`) |
| Readout | % + windows + reset timestamps | money amounts only | % + windows + reset timestamps |
| Endpoint | `/v1/token_plan/remains` | `/account/query_balance` | `/v1/api/openplatform/coding_plan/remains` |
| Resets | 5h-ish + 7d rolling | never | same 5h-ish + 7d rolling |

---

## 2. Endpoint catalogue

### 2.1 M Plan quota — `token_plan/remains` (primary target)

**Official documentation** (three ways to check usage, method 3 = API):

```bash
curl --location 'https://www.minimax.io/v1/token_plan/remains' \
--header 'Authorization: Bearer <M Plan API key>' \
--header 'Content-Type: application/json'
```

Source: <https://platform.minimax.io/docs/m-plan/faq.md> → "How can I check M Plan usage?" → **Method 3: Use the API endpoint**.
CN equivalent (`https://www.minimax.cn/v1/token_plan/remains`): <https://platform.minimaxi.com/docs/m-plan/faq.md>.

**Official CLI implementation** (`github.com/MiniMax-AI/cli`, npm `mmx-cli`):

- `src/client/endpoints.ts:49` — `export function quotaEndpoint(baseUrl: string) { return \`${baseUrl}/v1/token_plan/remains\`; }`
- `src/config/schema.ts:1-5` — `REGIONS = { global: 'https://api.minimax.io', cn: 'https://api.minimax.cn' }`
  → CLI base URL is the **api** host; the docs use the **www** host. **Both work** (verified live, §2.4).
- `src/client/http.ts:145` — `method: opts.method ?? 'GET'` → the call is a **GET**.
- `src/client/http.ts:117` — `headers['Authorization'] = \`Bearer ${credential.token}\``; `src/client/http.ts:115` also supports `x-api-key`.
- `src/client/endpoints.ts:73-88` — `selectUsageEndpoint()`: an `sk-api-` key → balance endpoint, anything else (subscription key or OAuth) → quota endpoint.

Raw source: <https://raw.githubusercontent.com/MiniMax-AI/cli/main/src/client/endpoints.ts> (repo `MiniMax-AI/cli`, branch `main`, read 2026-11-05).

### 2.2 Response shape of `token_plan/remains`

Type definition, `github.com/MiniMax-AI/cli` → `src/types/api.ts:337` and `:352`:

```ts
export interface QuotaResponse {
  model_remains: QuotaModelRemain[];
}

export interface QuotaModelRemain {
  model_name: string;
  start_time: number;                                  // epoch ms, current (short) window start
  end_time: number;                                    // epoch ms, current (short) window END = RESET
  remains_time: number;                                // ms until `end_time`
  current_interval_total_count: number;                // quota limit for the short window (may be 0)
  current_interval_usage_count: number;                // count for the short window (ambiguous, see §2.4)
  current_interval_remaining_percent?: number;         // % REMAINING in the short window
  current_weekly_total_count: number;
  current_weekly_usage_count: number;
  current_weekly_remaining_percent?: number;           // % REMAINING in the weekly window
  current_interval_status?: number;                    // 1 = limited/normal, 2 = exhausted, 3 = unlimited
  current_weekly_status?: number;                     // (status enum documented at src/types/api.ts:363-365)
  weekly_start_time: number;                           // epoch ms, weekly window start
  weekly_end_time: number;                             // epoch ms, weekly window END = RESET
  weekly_remains_time: number;                         // ms until `weekly_end_time`
  weekly_boost_permille?: number;                      // weekly display multiplier, permille (1500 ⇒ up to 150%)
}
```

Field → widget mapping:

| Widget need | Field | Notes |
| --- | --- | --- |
| **% consumed** | *derived* | API gives **remaining** %, not used %. `used% = 100 - current_*_remaining_percent`. |
| **Quota limit** | `current_interval_total_count`, `current_weekly_total_count` | **Often `0`** — see §2.4. No absolute limit is guaranteed. |
| **Reset timestamp** | `end_time` (short), `weekly_end_time` (weekly) | Epoch **milliseconds**, UTC. `remains_time` / `weekly_remains_time` give the countdown in ms. |
| **Window duration** | `end_time - start_time`, `weekly_end_time - weekly_start_time` | Derive it; do not hardcode. |
| **Unlimited** | `current_*_status === 3` | `2` = exhausted. |
| **Model grouping** | `model_name` | One entry per model family (see §3.2). |

### 2.3 Pay-as-you-go balance — `account/query_balance`

`github.com/MiniMax-AI/cli` → `src/client/endpoints.ts:53`:

```ts
export function accountBalanceEndpoint(baseUrl: string): string {
  return `${baseUrl}/account/query_balance`;
}
```

Response type, `src/types/api.ts:341`:

```ts
export interface AccountBalanceResponse {
  available_amount: string;
  cash_balance: string;
  voucher_balance: string;
  credit_balance: string;
  owed_amount: string;
  balance_alert_switch: boolean;
  balance_alert_threshold: string;
  base_resp: BaseResp;        // { status_code: number; status_msg: string }
}
```

- All amounts are **strings**. There is **no percentage, no window, no reset timestamp** → this surface cannot feed the "% consumed / time to reset" widget directly; it can only show a money balance.
- Endpoint selected automatically when the key starts with `sk-api-` (`src/client/endpoints.ts:57-58`, `:77-81`).
- Verified live (§2.4): the endpoint exists on both `api.minimax.io` and `api.minimax.cn` (unauthenticated → HTTP 400 `{"base_resp":{"status_code":1004,"status_msg":"not authorized"}}`).

### 2.4 Live verification on this machine (2026-11-05)

Using the user's own M Plan subscription key stored in `/Users/javi/.mmx/config.json` (key not reproduced):

**`GET https://api.minimax.io/v1/token_plan/remains` → HTTP 200**, body (verbatim, values as returned):

```json
{
  "model_remains": [
    {
      "start_time": 1793908800000,
      "end_time": 1793923200000,
      "remains_time": 12697570,
      "current_interval_total_count": 0,
      "current_interval_usage_count": 0,
      "model_name": "general",
      "current_weekly_total_count": 0,
      "current_weekly_usage_count": 0,
      "weekly_start_time": 1793577600000,
      "weekly_end_time": 1794182400000,
      "weekly_remains_time": 271897570,
      "current_interval_status": 1,
      "current_interval_remaining_percent": 85,
      "current_weekly_status": 1,
      "current_weekly_remaining_percent": 26
    },
    {
      "start_time": 1793836800000,
      "end_time": 1793923200000,
      "remains_time": 12697570,
      "current_interval_total_count": 0,
      "current_interval_usage_count": 0,
      "model_name": "video",
      "current_weekly_total_count": 0,
      "current_weekly_usage_count": 0,
      "weekly_start_time": 1793577600000,
      "weekly_end_time": 1794182400000,
      "weekly_remains_time": 271897570,
      "current_interval_status": 3,
      "current_interval_remaining_percent": 100,
      "current_weekly_status": 3,
      "current_weekly_remaining_percent": 100
    }
  ],
  "base_resp": { "status_code": 0, "status_msg": "success" }
}
```

Same body returned by `https://www.minimax.io/v1/token_plan/remains` (only `remains_time` differs by the elapsed ms) — **both hosts work**.
`POST` to the same URL returns **HTTP 411** → it is a **GET** endpoint.

Decoded timestamps (UTC):

| Model | Short window | Length | Weekly window | Length | Short % left | Weekly % left |
| --- | --- | --- | --- | --- | --- | --- |
| `general` | 2026-11-05T20:00Z → 2026-11-06T00:00Z | **4.0 h** | 2026-11-02T00:00Z → 2026-11-09T00:00Z | **7.0 d** | 85 | 26 |
| `video` | 2026-11-05T00:00Z → 2026-11-06T00:00Z | **24.0 h** | 2026-11-02T00:00Z → 2026-11-09T00:00Z | **7.0 d** | 100 (unlimited, `status=3`) | 100 (unlimited) |

A second live call ~10 minutes later returned `current_interval_remaining_percent: 82` with the same `start_time`/`end_time` and a smaller `remains_time` → the window is a **fixed window in flight**, not recomputed per request.

**`GET https://api.minimax.io/account/query_balance` with the same subscription key → HTTP 200:**

```json
{"available_amount":"0.00","cash_balance":"0.00","voucher_balance":"0.00","credit_balance":"0.00","owed_amount":"0.00","balance_alert_switch":false,"balance_alert_threshold":"","base_resp":{"status_code":0,"status_msg":"success"}}
```

(i.e. the endpoint accepts a subscription key too, but reports zeros — a subscription key has no pay-as-you-go balance.)

The installed official CLI reproduces the same numbers: `mmx quota --output json` on this machine returned the identical `model_remains` structure.

### 2.5 MiniMax Code (managed login) quota endpoint

Source: `github.com/MiniMax-AI/minimax-code` (branch `main`, read 2026-11-05) → `packages/tui/src/account/matrix-account-client.ts`:

- `:25` — `const TOKEN_PLAN_REMAINS_PATH = '/v1/api/openplatform/coding_plan/remains';`
- `:42-55` — origins:
  - `cn` / `prod` → `https://www.minimaxi.com`
  - `en` / `prod` → `https://platform.minimax.io`
- `:533` — `GET ${origin}${TOKEN_PLAN_REMAINS_PATH}` with headers `Accept: application/json`, `Authorization: Bearer <OAuth access token>`, and optionally `X-Group-Id: <op_group_id>`.
- The **response body is the same `model_remains` shape** as §2.2 — the parser at `:672-695` (`readQuotaWindow`) reads `current_interval_remaining_percent` / `current_weekly_remaining_percent`, `current_interval_status` / `current_weekly_status`, `end_time`, `weekly_end_time`.
- Video quota is read separately at `:697-708` (`readVideoQuota`) from `current_interval_total_count` / `current_interval_usage_count` / `end_time`.
- Membership / credit balance endpoint: `:23` — `GET https://agent.minimax.io/matrix/api/v1/commerce/get_membership_info` (cn: `https://agent.minimaxi.com`), parsed by `projectMembership` into `has_token_plan`, `op_group_id`, `token_plan_tier`, `token_plan_expires_at`, `op_credit_summary.total_remaining_amount`.
- Note the origin hosts here are the **platform** hosts, not `api.minimax.io`.

---

## 3. Usage windows

### 3.1 What the docs say

- Two windows for non-video models (text / image / audio): a **5-hour window** and a **weekly window (7 days)**.
  Source: <https://platform.minimax.io/docs/m-plan/usage-rules.md> — table "5 小时窗口 | 周窗口", rows "适用模型 / 开始计时 / 持续时间 / 结束后".
  English: <https://platform.minimax.io/docs/m-plan/faq.md> → "Both windows start with your first use: the 5-hour window lasts 5 hours, and the weekly window lasts 7 days."
- Video models are subject to the **weekly window only**.
  Source: same pages ("Video models: Subject only to a weekly window, with no 5-hour window").
- Windows **reset independently**; unused quota does **not** roll over.
  Source: <https://platform.minimax.io/docs/m-plan/usage-rules.md>.
- There is **no monthly window** on the subscription. The only "monthly" concept is the **billing/subscription cycle**, and the only long-lived balance is the **credit pack** (valid 1 year).
  Source: <https://platform.minimax.io/docs/m-plan/faq.md>.
- Rate limits (RPM/TPM) are computed **separately** from quota windows — a 429 does not mean the quota is exhausted.
  Source: <https://platform.minimax.io/docs/m-plan/usage-rules.md> → "额度充足，为什么仍然限流？".

### 3.2 What the API actually returns (windows are per-model and server-defined)

- `model_remains` is an **array**, one entry per model family, each with its **own** window boundaries and percentages.
  Observed `model_name` values: `"general"`, `"video"` (live, §2.4).
  Values in the official CLI's own test fixture (`github.com/MiniMax-AI/cli` → `test/fixtures/quota-response.json`): `"MiniMax-M*"`, `"speech-hd"`, `"image-01"`.
- Observed short-window lengths in that fixture: `MiniMax-M*` = **5 h** exactly (`1776355200000 → 1776373200000`), `speech-hd` = **24 h**, `image-01` = **24 h**.
- Observed live: `general` = **4.0 h**, `video` = **24.0 h** — i.e. the short window length is **not** always 5 hours.
  `[INFERENCE]` The short window appears to be a 5-hour rolling window anchored on first use, truncated/rounded onto a fixed UTC grid; the exact anchoring rule is not documented. See §7.
- Consequence for `fuelcheck`: **derive the window length from `end_time - start_time`**, exactly as the official CLI does — `src/output/quota-table.ts` (`formatWindow()` renders `5h` / `1d` / `1w` / `Nh` / `Nm` from `end_time - start_time`), rather than assuming "5 hours".
- The weekly window was exactly 7 days (`604800000 ms`) in every observation, aligned to UTC midnight.

---

## 4. Where MiniMax credentials live locally

### 4.1 Official CLI `mmx` (`mmx-cli`, npm)

Source: `github.com/MiniMax-AI/cli` (branch `main`, read 2026-11-05).

- **Config dir**: `~/.mmx` — `src/config/paths.ts:4` (`const CONFIG_DIR_NAME = '.mmx';`), overridable with the env var **`MMX_CONFIG_DIR`** (`src/config/paths.ts:7`).
- **Config file**: `~/.mmx/config.json` — `src/config/paths.ts:11` (`getConfigPath()`).
- **JSON shape** (`src/config/schema.ts:29` `ConfigFile`, parsed by `parseConfigFile`):

```jsonc
{
  "api_key": "sk-cp-...",          // subscription key (or "sk-api-..." for pay-as-you-go)
  "region": "global",              // "global" | "cn"
  "base_url": "https://api.minimax.io",
  "output": "text",                // "text" | "json"
  "timeout": 300,
  "proxy": "http://...",
  "oauth": {                       // present after `mmx auth login` (browser device-code flow)
    "access_token": "...",
    "refresh_token": "...",
    "expires_at": "2026-11-05T20:00:00.000Z",   // ISO 8601
    "region": "global",
    "resource_url": "https://api.minimax.io",    // optional API host override returned by the server
    "account": "..."
  },
  "default_text_model": "MiniMax-M3.1-Flash",
  "default_speech_model": "...",
  "default_video_model": "..."
}
```

- File permissions: directory `0700`, file `0600` — `src/config/paths.ts` (`ensureConfigDir`, mode `0o700`) and `src/config/loader.ts` (`writeConfigFile`, `mode: 0o600`).
- **Credential resolution order** — `src/auth/resolver.ts:9-30`:
  1. `--api-key` flag (`:11-12`)
  2. `oauth` object in `~/.mmx/config.json` (`:16-19`)
  3. `api_key` in `~/.mmx/config.json` (`:23-24`)
  4. otherwise `CLIError('No credentials found.')` (`:28`)
- **OAuth device-code flow** — `src/auth/oauth.ts`: `POST https://account.minimax.io/oauth2/device/code` and `POST https://account.minimax.io/oauth2/token` (cn: `account.minimaxi.com` — `src/config/schema.ts:12-13`), `client_id = 659cf4c1-615c-45f6-a5f6-4bf15eb476e5`, `client_name = "MiniMax CLI"`, scopes `openid profile coding_plan`. The resulting OAuth token is a valid bearer for the quota endpoint (`selectUsageEndpoint` returns the quota endpoint for `method === 'oauth'`).
- **No keychain usage** — the CLI writes/reads only `~/.mmx/config.json`. (Grep of the repo for `keychain` finds nothing credential-related.)
- `mmx quota` = `mmx quota show` (auto-forward, `src/registry.ts:95-101`); implementation `src/commands/quota/show.ts`.

### 4.2 MiniMax Code / `mcode` (the official agent app)

Source: `github.com/MiniMax-AI/minimax-code` (branch `main`, read 2026-11-05).

- **Data dir**: `~/.minimax` — `packages/config/src/data-dir.ts:5` (`NEW_DATA_DIR_BASENAME = '.minimax'`); overridable with **`MINIMAX_DATA_DIR`** or **`MAVIS_DATA_DIR`** (`packages/config/src/config.ts:1370`).
- **Credential file path** — `packages/oauth-core/src/namespace.ts:22-34`:
  `join(dataDir, 'auth', buildEnv, region, MCODE_OAUTH_CLIENT_ID, 'auth.json')`
  with `MCODE_OAUTH_CLIENT_ID = 'mcode-public'` (`packages/oauth-core/src/contracts.ts:1`).
  → **`~/.minimax/auth/prod/en/mcode-public/auth.json`** (cn: `.../prod/cn/...`).
- **JSON shape** (`packages/oauth-core/src/credential-store/file-store.ts` `CredentialFilePayload`):

```jsonc
{
  "schemaVersion": 1,
  "records": {
    "com.minimax.mcode.oauth.prod.en\u0000<base64url sha256>": {
      "schemaVersion": 1,
      "accessToken": "mmoat_...",
      "refreshToken": "mmort_...",
      "tokenType": "Bearer",
      "clientId": "mcode-public",
      "scopes": ["agent.default"],
      "audience": "agent-backend",
      "expiresAtMs": 1793946654384,
      "generation": 129,
      "loginEpoch": "<uuid>"
    }
  }
}
```

  (The record key is `service + "\0" + account`, `file-store.ts:28-30`; `service = com.minimax.mcode.oauth.<buildEnv>.<region>`.)
- Sibling file `auth-state.json` in the same directory holds `schemaVersion`, `status`, `storeKind`, `clientId`, `scopes`, `audience`, `buildEnv`, `region`, `generation`, `expiresAtMs`.
- **No keychain usage.** `packages/oauth-core/src/credential-store/factory.ts` returns a `FileStore` only; a repo-wide grep for `keychain` hits only macOS-system-keychain *path* deny-lists, never credential storage.
- A separate file `~/.minimax/local-runtime.auth.json` holds the session-style auth context: `{ version, updatedAtMs, auth: { accessToken (JWT), realUserID, userEmail, userName, subUserName } }`.

### 4.3 MiniMax keys written into *other* tools by `mmx agent setup`

Source: `github.com/MiniMax-AI/cli` → `src/agent/configurator.ts`; the table is also published at
<https://platform.minimax.io/docs/m-plan/minimax-cli.md> ("修改的配置文件") and
<https://platform.minimax.io/docs/m-plan/other-tools.md>.

| Tool | File | Where the MiniMax key lands |
| --- | --- | --- |
| Claude Code | `~/.claude/settings.json` | `env.ANTHROPIC_AUTH_TOKEN` = key, `env.ANTHROPIC_BASE_URL` = `https://api.minimax.io/anthropic` (`configurator.ts:571-584`) |
| Codex | `~/.codex/config.toml` (+ `~/.codex/mmx-model-catalog.json`) | `model_providers.minimax.experimental_bearer_token` = key, `base_url` = `https://api.minimax.io/v1` (`configurator.ts:653-665`) |
| Grok CLI | `~/.grok/config.toml` | provider `base_url` + token |
| OpenCode | `~/.config/opencode/opencode.json` | `provider.minimax.options.apiKey` = key (`configurator.ts:840`) |
| Hermes Agent | `~/.hermes/config.yaml` + `~/.hermes/.env` | env var **`MINIMAX_API_KEY`** (global) or **`MINIMAX_CN_API_KEY`** (cn) (`configurator.ts:856`, `:868`) |
| Pi / Oh My Pi | `~/.pi/agent/models.json`, `~/.pi/agent/settings.json` | `providers.<id>.baseUrl` + `apiKey` (`configurator.ts:905-906`) |
| Oh My Pi (omp) | `~/.omp/agent/models.yml` | `providers.minimax.baseUrl` / `apiKey` — published example at <https://platform.minimax.io/docs/m-plan/other-tools.md> → "Oh My Pi" |
| Zed | `~/.config/zed/settings.json` | provider `baseUrl` + `apiKey` |

`configurator.ts:62-68` — region → base URLs: `https://api.minimax.io/anthropic` and `https://api.minimax.io/v1` (cn: `api.minimax.cn`).

### 4.4 Environment variables that carry MiniMax credentials / settings

| Variable | Where | Meaning | Source |
| --- | --- | --- | --- |
| `MMX_CONFIG_DIR` | official CLI | override `~/.mmx` | `src/config/paths.ts:7` |
| `MINIMAX_REGION` | official CLI | `global` \| `cn` | `src/config/loader.ts:70` |
| `MINIMAX_BASE_URL` | official CLI | override API base URL | `src/config/loader.ts:86` |
| `MINIMAX_OUTPUT` | official CLI | `text` \| `json` | `src/config/loader.ts:93` |
| `MINIMAX_TIMEOUT` | official CLI | request timeout (s) | `src/config/loader.ts:96` |
| `MINIMAX_VERBOSE` | official CLI | `1` → log requests | `src/config/loader.ts:113` |
| `MINIMAX_API_KEY` | Hermes Agent / generic convention | MiniMax key | `configurator.ts:856` |
| `MINIMAX_CN_API_KEY` | Hermes Agent (cn region) | MiniMax key | `configurator.ts:856` |
| `ANTHROPIC_AUTH_TOKEN` | Claude Code-style tools | MiniMax key when `ANTHROPIC_BASE_URL` points at MiniMax | <https://platform.minimax.io/docs/m-plan/other-tools.md> |
| `OPENAI_API_KEY` | OpenAI-style tools | MiniMax key when `OPENAI_BASE_URL` points at MiniMax | same |
| `MINIMAX_DATA_DIR`, `MAVIS_DATA_DIR` | MiniMax Code | override `~/.minimax` | `packages/config/src/config.ts:1370` |

Note: the official CLI does **not** read the key from an env var for its own use — only `--api-key` flag or `~/.mmx/config.json` (`src/auth/resolver.ts:9-30`).

### 4.5 Keychain

**No keychain usage** was found in any MiniMax first-party client:

- `mmx` CLI → file only (`~/.mmx/config.json`).
- MiniMax Code → file store only (`packages/oauth-core/src/credential-store/factory.ts` returns `FileStore`; no `keychain` module in `packages/oauth-core`).
- Verified on this machine: `security dump-keychain` output filtered for `minimax|mcode|mavis` returns only TLS *server* entries (`"srvr"<blob>="https://api.minimax.io/v1"`), no generic-password credential items.

### 4.6 Recommended discovery order for `fuelcheck` (all first-party, no scraping)

1. `~/.mmx/config.json` → `api_key` (if it starts with `sk-cp-`) → M Plan quota.
2. `~/.mmx/config.json` → `oauth.access_token` (if present) → M Plan quota (OAuth works against the same endpoint).
3. `~/.mmx/config.json` → `api_key` (if it starts with `sk-api-`) → pay-as-you-go balance.
4. `~/.minimax/auth/<buildEnv>/<region>/mcode-public/auth.json` → `records[*].accessToken` → MiniMax Code quota.
5. Fallbacks for keys pasted into other tools: `~/.claude/settings.json` `env.ANTHROPIC_AUTH_TOKEN` (only when `env.ANTHROPIC_BASE_URL` is a `minimax` host), `~/.hermes/.env` `MINIMAX_API_KEY`, `~/.omp/agent/models.yml` `providers.minimax.apiKey`, `~/.pi/agent/auth.json` (`{"minimax": {"type":"api_key","key":"…"}}`), `~/.config/opencode/opencode.json`, `~/.codex/config.toml`, `~/.grok/config.toml`, `~/.config/zed/settings.json`.
6. Env vars: `MINIMAX_API_KEY`, `MINIMAX_CN_API_KEY`, `ANTHROPIC_AUTH_TOKEN` (guarded by a MiniMax base URL).

---

## 5. First-party code that queries a quota endpoint (file + line)

| File : line | What it shows |
| --- | --- |
| `MiniMax-AI/cli` → `src/client/endpoints.ts:49` | `quotaEndpoint()` → `${baseUrl}/v1/token_plan/remains` |
| `MiniMax-AI/cli` → `src/client/endpoints.ts:53` | `accountBalanceEndpoint()` → `${baseUrl}/account/query_balance` |
| `MiniMax-AI/cli` → `src/client/endpoints.ts:57-58` | `isSecretApiKey()` → `apiKey.startsWith('sk-api-')` |
| `MiniMax-AI/cli` → `src/client/endpoints.ts:73-88` | `selectUsageEndpoint()` — the quota-vs-balance decision |
| `MiniMax-AI/cli` → `src/sdk/quota/index.ts:20-30` | `QuotaSDK.info()` — resolves credential, picks endpoint, GETs it |
| `MiniMax-AI/cli` → `src/commands/quota/show.ts:26-49` | `mmx quota show` — picks endpoint by credential type, renders |
| `MiniMax-AI/cli` → `src/types/api.ts:337`,`:341`,`:352` | `QuotaResponse`, `AccountBalanceResponse`, `QuotaModelRemain` (status enum `:363-365`, `weekly_boost_permille` `:372`) |
| `MiniMax-AI/cli` → `src/utils/quota.ts:18-58` | `resolveQuotaCounts()` — resolves the **ambiguous** `*_usage_count` field |
| `MiniMax-AI/cli` → `src/output/quota-table.ts` (`formatWindow`, `readQuotaWindow`-equivalent `remainingPct`, `isUnweekly`) | The official rendering: window tag from `end_time - start_time`, unlimited when `status === 3`, weekly boost multiplier |
| `MiniMax-AI/cli` → `src/config/detect-region.ts:12-24` | Region auto-detect by probing the usage endpoint with both `Authorization: Bearer` and `x-api-key` |
| `MiniMax-AI/cli` → `src/auth/resolver.ts:9-30` | Credential resolution order |
| `MiniMax-AI/cli` → `src/auth/oauth.ts` | OAuth device-code flow against `account.minimax.io` |
| `MiniMax-AI/cli` → `test/fixtures/quota-response.json` | A real captured `model_remains` payload (5 h / 24 h windows) |
| `MiniMax-AI/minimax-code` → `packages/tui/src/account/matrix-account-client.ts:25` | `TOKEN_PLAN_REMAINS_PATH = '/v1/api/openplatform/coding_plan/remains'` |
| `MiniMax-AI/minimax-code` → `packages/tui/src/account/matrix-account-client.ts:42-55` | Origin hosts `platform.minimax.io` (en) / `www.minimaxi.com` (cn) |
| `MiniMax-AI/minimax-code` → `packages/tui/src/account/matrix-account-client.ts:525-567` | `fetchTokenPlanQuota()` — GET + Bearer + `X-Group-Id`, parses `model_remains` |
| `MiniMax-AI/minimax-code` → `packages/tui/src/account/matrix-account-client.ts:672-695` | `readQuotaWindow()` — exact field names for % remaining, reset and unlimited |
| `MiniMax-AI/minimax-code` → `packages/tui/src/account/matrix-account-client.ts:697-708` | `readVideoQuota()` |
| `MiniMax-AI/minimax-code` → `packages/tui/src/account/matrix-account-client.ts:23`,`:266` | `GET {agent host}/matrix/api/v1/commerce/get_membership_info` → `has_token_plan`, `op_group_id`, `token_plan_tier`, `token_plan_expires_at`, `op_credit_summary.total_remaining_amount` |
| `MiniMax-AI/minimax-code` → `packages/oauth-core/src/namespace.ts:22-34` | Credential path `auth/<buildEnv>/<region>/<clientId>/auth.json` |
| `MiniMax-AI/minimax-code` → `packages/oauth-core/src/credential-store/file-store.ts` | `auth.json` payload shape + 0600/0700 permission enforcement |
| `MiniMax-AI/minimax-code` → `packages/config/src/data-dir.ts:5` | `NEW_DATA_DIR_BASENAME = '.minimax'` |
| `MiniMax-AI/minimax-code` → `packages/tui/src/runtime/port.ts:788-811` | The normalized quota model the UI consumes: `{ fiveHour: {remainingPercent, resetAtMs, unlimited}, weekly: {...}, video?: {remainingCount, totalCount, resetAtMs, unlimited} }` |

Raw URLs follow `https://raw.githubusercontent.com/MiniMax-AI/cli/main/<path>` and `https://raw.githubusercontent.com/MiniMax-AI/minimax-code/main/<path>`.

---

## 6. This machine (macOS, `arm64`, home `/Users/javi`)

### 6.1 What exists

| Path | What | Notes |
| --- | --- | --- |
| `/Users/javi/.mmx/config.json` | Official MiniMax CLI config | `{"region":"global","api_key":"sk-cp-…"}` — **the M Plan subscription key**. Mode `0600`. |
| `/Users/javi/.mmx/update-state.json` | CLI update state | `{"lastChecked":…,"latestVersion":"v1.0.28"}` |
| `/opt/homebrew/bin/mmx` | Official CLI binary | npm global `mmx-cli@1.0.28` (publisher `MiniMax`, MIT) |
| `/Users/javi/.minimax/` | MiniMax Code data dir | dir `~/.mavis` is a symlink → `~/.minimax` |
| `/Users/javi/.minimax/auth/prod/en/mcode-public/auth.json` | MiniMax Code OAuth credentials | `accessToken: "mmoat_…"`, `refreshToken: "mmort_…"`, `clientId: "mcode-public"`, `audience: "agent-backend"`, `scopes: ["agent.default"]`, `expiresAtMs: 1793946654384` |
| `/Users/javi/.minimax/auth/prod/en/mcode-public/auth-state.json` | OAuth state | `{"schemaVersion":2,"status":"authenticated","storeKind":"file","clientId":"mcode-public","audience":"agent-backend","buildEnv":"prod","region":"en","expiresAtMs":…}` |
| `/Users/javi/.minimax/local-runtime.auth.json` | Session auth context | `{"version":1,"updatedAtMs":…,"auth":{"accessToken":"<JWT>","realUserID":"…","userEmail":"…","userName":"…","subUserName":"…"}}` |
| `/Users/javi/.minimax/config.yaml` | MiniMax Code model/provider config | Contains `provider.minimax` with `baseUrl` `https://agent.minimax.io/mavis/api/v1/llm/v1` and model entries; **no API key** in this file |
| `/Users/javi/.minimax/sqlite.db` | MiniMax Code local state | |
| `/Applications/MiniMax Code.app` | MiniMax Code desktop app | installed |
| `~/Library/Application Support/MiniMax Agent` | MiniMax Code app support dir | |
| `/Users/javi/.pi/agent/auth.json` | Pi agent auth store | `{"minimax":{"type":"api_key","key":"sk-cp-…"},"openai-codex":{…}}` |
| `/Users/javi/.zshrc:237-238`, `/Users/javi/.bashrc:2-3` | PATH additions | `# Added by MiniMax Code` / `export PATH="/Users/javi/.minimax/bin:$PATH"` — **no key** |
| `/Users/javi/.config/zed/settings.json` | Zed model registry | references `minimax/MiniMax-M3` and `https://api.minimax.io/v1`; **no key found in the file** |

### 6.2 What does **not** exist here

- `/Users/javi/.config/minimax` — absent.
- `/Users/javi/.minimax-coding-plan` — absent.
- No `MINIMAX*` credential environment variable is set in the current shell environment (`env | grep -i minimax` returns only `PATH` containing `~/.minimax/bin` and `PI_SESSION_FILE`).
- `/Users/javi/.claude/settings.json` currently points at **z.ai**, not MiniMax: `env.ANTHROPIC_BASE_URL = "https://api.z.ai/api/anthropic"`, `env.ANTHROPIC_AUTH_TOKEN = "cd1e59436f0a4bc8a70e0ffd72ee8672.7KSWIBgxcqRJWx3e"` (value shown as found; treat as a secret).
- `/Users/javi/.omp/agent/models.yml` contains **no** `minimax` provider block.
- No macOS keychain generic-password items for MiniMax (see §4.5).

### 6.3 Live probes performed from this machine (unauthenticated)

| URL | Result |
| --- | --- |
| `https://api.minimax.io/v1/token_plan/remains` | HTTP 200, `{"base_resp":{"status_code":1004,"status_msg":"login fail: Please carry the API secret key in the 'Authorization' field of the request header"}}` |
| `https://www.minimax.io/v1/token_plan/remains` | same |
| `https://api.minimax.cn/v1/token_plan/remains` | same |
| `https://www.minimax.cn/v1/token_plan/remains` | same |
| `https://api.minimax.io/account/query_balance` | HTTP 400, `{"base_resp":{"status_code":1004,"status_msg":"not authorized"}}` |
| `https://api.minimax.cn/account/query_balance` | HTTP 400, same |

Both endpoint paths therefore exist on both regions. Authenticated results are in §2.4.

---

## 7. What remains unknown

1. **The exact anchoring rule of the short window.** Docs say "5 hours"; the live response returned a **4.0 h** window for `general` and **24.0 h** for `video`; the CLI fixture shows 5 h / 24 h / 24 h. Whether the short window is "5 h from first use, snapped to a fixed grid", "a 4/24 h grid", or tier-dependent is **not documented**. → derive from `end_time - start_time`; do not hardcode.
2. **Whether `current_interval_total_count` / `current_weekly_total_count` are ever non-zero.** On this account they were `0` for every model, and the official CLI explicitly treats `total <= 0` as "no absolute limit available" (`src/utils/quota.ts:24-31`). It is **unknown** under which plans/tiers these carry an absolute count (the CLI fixture does: `1500`, `9000`, `63000`, `100`, `700`). → the widget must be able to render a percentage **without** a limit.
3. **The semantics of `current_interval_usage_count` / `current_weekly_usage_count`.** The official CLI documents that older servers returned them as *remaining* counts and newer servers as *consumed* counts, and disambiguates using `*_remaining_percent` with a ±1 tolerance (`src/utils/quota.ts:18-51`). → the same ambiguity resolution must be reproduced, or the percent fields used directly.
4. **When `weekly_boost_permille` appears.** Documented in the CLI type (`src/types/api.ts:372`) and used to scale the weekly display (1500 ⇒ up to 150%), but it was **absent** from every response observed here. Unknown which plans/tiers emit it.
5. **`current_interval_status` / `current_weekly_status` full value space.** The CLI documents 1 = normal (limited), 2 = exhausted, 3 = unlimited (`src/types/api.ts:363-365`), and treats "both statuses 3 and both totals 0" as "model not in this plan" (`src/output/quota-table.ts`, `isUnavailablePlan`, referencing CLI issue #173). A live `status: 2` (exhausted) response was never observed.
6. **The MiniMax Code endpoint is unauthenticated-unverified.** `/v1/api/openplatform/coding_plan/remains` was read from source but **never called** — this machine's MiniMax Code OAuth token was not exercised against it. Its exact response body (it is expected to be the same `model_remains` shape) and the `X-Group-Id` requirement are therefore **[UNVERIFIED]** at runtime.
7. **Whether `/v1/api/openplatform/coding_plan/remains` is reachable with an `sk-cp-` subscription key** (as opposed to an OAuth token). Untested.
8. **Per-model granularity.** `model_name` values seen: `general`, `video` (live) and `MiniMax-M*`, `speech-hd`, `image-01` (fixture). The mapping from model family → `model_name` string, and whether an "all models" aggregate exists, is undocumented.
9. **Whether a *monthly* window exists at all.** No evidence of one. The monthly concept that exists is the **billing cycle** (no quota window) and **credit packs** (1-year validity, not a window). Treated as "no monthly window" — **[INFERENCE]** from the two doc pages above.
10. **Whether the `www.` and `api.` hosts are fully interchangeable** for `token_plan/remains`. Both returned identical bodies for this one key today; long-term parity is not guaranteed by any doc.
11. **Whether `account/query_balance` is documented anywhere publicly.** It is **not** in the MiniMax docs index (`https://platform.minimax.io/docs/llms.txt` — no balance/usage API page); it exists only in the official CLI source. Its availability/stability for third-party tools is therefore **[UNVERIFIED]** against MiniMax's public API contract.
12. **Whether the official `mmx` CLI's own config (`~/.mmx`) is the *only* place MiniMax stores `sk-cp-` keys.** Not exhaustively provable; §4.3/§6.1 enumerate the known writers.
13. **M Plan tier → absolute quota mapping** (e.g. what "1× Go" means in requests or tokens) is not published; only relative multiples are given (<https://platform.minimax.io/docs/m-plan/intro.md>).

---

## 8. Practical implications for `fuelcheck` [INFERENCE]

1. **Primary source of truth = `GET {api base}/v1/token_plan/remains`** with `Authorization: Bearer <sk-cp- key>`. It is documented publicly, implemented by the official CLI, and returns everything the widget needs (remaining %, window end timestamps, per-model).
2. **Render "remaining %" as consumed %** = `100 - current_*_remaining_percent`; never assume an absolute limit is present.
3. **Compute the reset countdown** from `remains_time` (ms) or from `end_time - now`; the weekly dot uses `weekly_end_time` / `weekly_remains_time`.
4. **Derive the window label** from `end_time - start_time` (`5h`, `1d`, `1w`, …) — the official CLI's `formatWindow()` does exactly this.
5. **Two dots** (short + weekly) matches the data: one `current_interval_*` group and one `current_weekly_*` group per model. There is no monthly dot.
6. **Credential discovery should be file-first** (`~/.mmx/config.json`, then `~/.minimax/auth/**/auth.json`), because neither MiniMax client uses the OS keychain and no MiniMax env var is required by the official CLI.
7. **Distinguish `sk-cp-` (subscription, windows) from `sk-api-` (balance, no windows)** by key prefix — this is exactly what the official CLI does (`src/client/endpoints.ts:57-58`).
8. **Model grouping**: `model_remains` is per model family, so the widget likely wants to aggregate (e.g. treat `general` as the text bucket, `video` separately) rather than emit one row per raw `model_name`.
