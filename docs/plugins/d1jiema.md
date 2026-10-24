# 第一接码（d1jiema）任务插件

插件键 `d1jiema`｜版本 `1.0.1`｜厂商接口：`GET https://api.d1jiema.com/zc/data.php`

把第一接码（手机号接码平台）的接口接进 New API，对外暴露三个模型：`getPhone`（取号）、
`getMsg`（取码）、`send`（发短信）。渠道密钥就是第一接码「API 对接中心」里创建的 token。

---

## 1. 插件做什么

| 模型      | 厂商 `code` | 作用                            | 必需参数                |
| --------- | ----------- | ------------------------------- | ----------------------- |
| `getPhone` | `getPhone`  | 取一个号码（随机或指定）        | 无（建议带 `keyWord`）  |
| `getMsg`   | `getMsg`    | 读取该号码收到的短信            | `phone`、`keyWord`      |
| `send`     | `send`      | 用该号码发送短信                | `phone`、`toPhone`、`content` |

厂商接口是同步 GET：一次请求就在同一个 HTTP 响应里返回结果。因此插件在**提交阶段就完成任务**
（`immediate: SUCCESS/FAILURE`），不进入后台轮询，也不会占用轮询线程，查询结果不需要再调用任何查询接口。

计费只覆盖**真正交付**的那一次：`getMsg` 在短信还没到时会返回 `[尚未收到]…`，这类读取按失败结算并
**全额退回预扣额度**，所以可以放心地反复读取（取值直到读到验证码），只有返回了短信内容的那一次才计费。

---

## 2. 文件

| 文件                                        | 作用                                             |
| ------------------------------------------- | ------------------------------------------------ |
| `plugins/tasks/d1jiema/plugin.js`            | 插件本体（内置插件，随源码编译进二进制）          |
| `plugins/tasks/d1jiema/fixture.json`         | 钩子级回放用例（`new-api plugin test` 使用）      |
| `plugins/d1jiema_test.go`                    | 提交即完成、厂商报错识别、原生呈现的回归用例      |

依赖宿主支持任务插件 API v1 的以下扩展：`retainResult`、`unitLabel`、`upstreams`。
插件**不声明** `channelTypes`，因此只通过「任务插件」(61) 或「New API」(60) 渠道工作。

---

## 3. 安装与启用

### 3.1 随源码编译（推荐）

插件目录已经在仓库内，直接编译即可：

```bash
go build -o new-api .
./new-api plugin lint plugins/tasks/d1jiema/plugin.js
# plugin d1jiema@1.0.1 is valid
```

启动后进入 **后台 → 任务插件**，列表里会出现「第一接码」，默认随总开关一起启用。

### 3.2 后台上传

后台 → 任务插件 → **上传任务插件** → 选择 `plugins/tasks/d1jiema/plugin.js`。
上传的同名版本会覆盖内置版本，已有渠道绑定关系不变；删除该上传版本后会回落到内置版本。

### 3.3 全局开关

后台 → 任务插件右上角的「启用任务插件」是总开关（环境变量 `TASK_PLUGIN_ENABLED`，默认开启）。
关闭后所有任务插件（含本插件）停止服务。

---

## 4. 渠道配置

后台 → 渠道 → 新建渠道：

| 字段       | 值                                                                                  |
| ---------- | ----------------------------------------------------------------------------------- |
| 类型       | 任务插件                                                                            |
| 任务插件   | 第一接码（`d1jiema`）                                                               |
| API 地址   | 留空即使用插件默认的 `https://api.d1jiema.com`；也可填 `https://api.d1jiema.com` 或完整的 `https://api.d1jiema.com/zc/data.php` |
| 密钥       | 第一接码网页版 → 个人中心 → API 对接中心 → 创建 API Token，把生成的 token 填在这里   |
| 模型       | `getPhone`、`getMsg`、`send`（按需勾选，多个用英文逗号分隔）                         |
| 分组、代理 | 按需                                                                                |

密钥只保存在渠道 Key 里，插件把它作为 `token` 查询参数拼到厂商请求上，不写入日志。
建议按用途分渠道（例如取号/取码一个、发短信一个），便于单独限速和停用。

---

## 5. 计费配置

插件对每次调用上报一个用量事实：`calls = 1`，语义是「一次**交付成功**的接码调用」。
`getMsg` 在短信到达之前返回的读取不属于交付，按失败结算、不产生用量，因此不占用额度。

**方式一：按次定价（最简单）**

后台 → 系统设置 → 模型（Models）→ 定价：给 `getPhone`、`getMsg`、`send` 分别填写「模型价格」（美元 / 次），
例如 `getPhone` 0.05、`getMsg` 0.01、`send` 0.10。

**方式二：任务用量表达式**

在任务插件计费里给这三个模型写表达式，例如 `u("calls") * 0.05`，其中 `calls` 就是插件声明的
「接码调用单价」字段。注意宿主不允许任务表达式使用固定价格表达式；若模型的计费模式设为「表达式」
却没有填写表达式，提交会直接返回 `model_price_error`。

表达式只对交付成功的调用求值。若表达式里还带一个与 `u("calls")` 无关的常数项（例如
`0.0001 + u("calls") * 0.25`），那部分常数会在每次调用时照收，包括尚未收到的读取；想要「读不到就
一分钱不花」，把常数项去掉即可。

**计费行为**

| 情况                                              | 结果                                             |
| ------------------------------------------------- | ------------------------------------------------ |
| 厂商返回正常内容                                  | 立即成功，按上面的配置结算                        |
| `getMsg` 返回 `[尚未收到]…`                       | 立即失败（`尚未收到短信（本次不计费）`），**全额退回预扣额度**，可以继续读 |
| 厂商返回 `ERROR:...`（token 错误、无可用号码等）   | 任务立即失败，**本次不计费**（预扣额度退回）      |
| 参数校验失败（缺 `phone` / `keyWord` 等）          | 400，请求不会发到厂商，不计费                     |

---

## 6. 调用方式

### 6.1 原生路由（脚本 / Agent 推荐）

网关地址 + `/d1jiema/<模型>`，用 New API 令牌鉴权：`Authorization: Bearer sk-...`。

**取号**

```bash
curl -X POST https://<你的网关>/d1jiema/getPhone \
  -H "Authorization: Bearer sk-xxxx" \
  -H "Content-Type: application/json" \
  -d '{"keyWord":"毛竹"}'
```

```json
{ "code": "ok", "operation": "getPhone", "task_id": "task_xxx", "phone": "13012345678", "raw": "13012345678" }
```

**取码**（短信到达前会返回「尚未收到」，该次不计费，按间隔继续读即可）

```bash
curl -X POST https://<你的网关>/d1jiema/getMsg \
  -H "Authorization: Bearer sk-xxxx" \
  -H "Content-Type: application/json" \
  -d '{"phone":"13012345678","keyWord":"毛竹"}'
```

```json
{
  "code": "ok",
  "operation": "getMsg",
  "task_id": "task_xxx",
  "phone": "13012345678",
  "message": "【毛竹】验证码9876",
  "pending": false,
  "raw": "【毛竹】验证码9876"
}
```

尚未收到时：`{"code":"error",...,"pending":true,"raw":"尚未收到…[尚未收到]","reason":"尚未收到短信（本次不计费），可继续读取：…"}` ——
这一**不计费**（预扣额度全额退回），继续按 3～5 秒的间隔读下去即可；返回短信内容的那一次才结算费用
（厂商侧另有「每 1000 次请求扣 0.01～0.2 元」的请求费，与网关计费无关）。

**发短信**

```bash
curl -X POST https://<你的网关>/d1jiema/send \
  -H "Authorization: Bearer sk-xxxx" \
  -H "Content-Type: application/json" \
  -d '{"phone":"16200000000","toPhone":"1069000000000","content":"hello"}'
```

```json
{ "code": "ok", "operation": "send", "task_id": "task_xxx", "message": "<厂商返回内容>", "raw": "<厂商返回内容>" }
```

**字段说明**

| 模型       | 字段       | 必填 | 说明                                                                 |
| ---------- | ---------- | ---- | -------------------------------------------------------------------- |
| `getPhone` | `keyWord`  | 否   | 短信关键词，一般是短信签名（黑括号里的字，如【毛竹】→ `毛竹`）。建议填写 |
| `getPhone` | `phone`    | 否   | 指定号码；不填随机取号                                                |
| `getPhone` | `province` | 否   | 省份，取值同 APP                                                      |
| `getPhone` | `cardType` | 否   | `实卡` / `虚卡` / `全部`，默认 `全部`                                  |
| `getMsg`   | `phone`    | 是   | 要取码的号码                                                          |
| `getMsg`   | `keyWord`  | 是   | 关键词，填错会一直查不到短信                                          |
| `send`     | `phone`    | 是   | 发送方号码（从 `getPhone` 取到的号码）                                 |
| `send`     | `toPhone`  | 是   | 接收方号码                                                            |
| `send`     | `content`  | 是   | 短信内容                                                              |
| `send`     | `projId`   | 否   | 项目 ID                                                               |

请求体里可以带 `"model"` 字段，但必须和路由对应的模型一致（例如 `/d1jiema/getMsg` 只能写 `getMsg`），
否则返回 400。手机号会自动去掉空格、`-` 和 `+86` 前缀。

**失败返回**

```json
{ "code": "error", "operation": "getPhone", "task_id": "task_xxx", "reason": "ERROR:没有可用的号码" }
```

原生路由的 HTTP 状态始终是 200，成功/失败看 `code` 字段。

### 6.2 `/v1/responses`

三个模型都已在 `POST /v1/responses` 上认领，支持 `sync`（默认）、`stream: true`、`background: true`。
参数来源优先级：**请求体顶层字段 > `metadata` > `input`**。`input` 可以是 JSON 字符串（推荐）；
当 `input` 是纯文本时，`getPhone`/`getMsg` 把它当作 `keyWord`，`send` 把它当作 `content`。

```bash
curl -X POST https://<你的网关>/v1/responses \
  -H "Authorization: Bearer sk-xxxx" \
  -H "Content-Type: application/json" \
  -d '{"model":"getMsg","input":"{\"phone\":\"13012345678\",\"keyWord\":\"毛竹\"}"}'
```

也可以把参数平铺在请求体顶层：

```bash
curl -X POST https://<你的网关>/v1/responses \
  -H "Authorization: Bearer sk-xxxx" -H "Content-Type: application/json" \
  -d '{"model":"getPhone","keyWord":"毛竹","cardType":"虚卡"}'
```

- 结果文本在 `output[0].content[0].text`（厂商原文）；`metadata` 按可用情况带 `operation`、`phone`、`pending`。
- 失败时返回 `status: "failed"`（HTTP 200），`error.message` 是通用文案，具体原因看任务日志的 `fail_reason`。
- 模型名可以用渠道「模型重定向」映射别名，插件会保留客户端看到的模型名。

---

## 7. 上游网关串联（可选）

插件声明了 `upstreams: ["vendor", "new_api"]`，因此可以把本插件接到另一台**也安装了该插件**的 New API 网关上：

- 渠道类型：New API；密钥：上游网关的令牌；模型：`getPhone`、`getMsg`、`send`；
  在「上游任务插件」里勾选 `d1jiema`。
- 此时插件会请求上游网关的 `/d1jiema/<模型>`（带 `Authorization: Bearer <上游令牌>`），
  并解析上游返回的同款 JSON 信封。

---

## 8. 厂商接口注意事项

- **频率与额外扣费**：同一账号或 IP 每 1000 次请求，平台会额外扣账户余额 0.01～0.2 元（与 New API 计费无关）。
  请控制调用频率，轮询取码建议 3～5 秒一次。
- **token 长期有效**：创建一次即可一直使用，不要频繁更换；换 token 后必须同步更新渠道 Key。
- **keyWord 很关键**：必须是短信里真实出现的字词（通常是【】中的签名）。填错会一直查不到短信。
- **自动释放与拉黑**：平台已自动释放号码、自动拉黑重复的随机号码，无需调用，因此插件不暴露 `release`/`block`。
- **发短信限制**：不能向个人手机号发送信息，发送垃圾信息会被封号。
- **余额查询**：厂商的 `code=leftAmount` 未封装成模型；如需可在插件中按同样模式增加一个模型与原生路由。

---

## 9. 验证

```bash
go run . plugin lint plugins/tasks/d1jiema/plugin.js
# plugin d1jiema@1.0.1 is valid

go run . plugin test plugins/tasks/d1jiema/plugin.js --fixture plugins/tasks/d1jiema/fixture.json
# plugin fixture passed: 28/28 cases

go test ./plugins/ -run TestD1jiema -v
```

---

## 10. 故障排查

| 现象                                              | 原因 / 处理                                                     |
| ------------------------------------------------- | --------------------------------------------------------------- |
| `ERROR:token错误，请重新通过API登录`              | 渠道 Key 不是有效 token（或已被更换）→ 重新创建并更新渠道 Key    |
| 一直 `pending: true`                              | 号码尚未收到短信：继续读取（每次「尚未收到」都不计费）；若长期收不到，检查 `keyWord` 是否与短信方括号里的名称一致 |
| 每次查询都在扣钱（`$0.25/次` 一堆）               | v1.2.22 及更早版本把「尚未收到」当成功结算；v1.2.23 起该次按失败全额退回（表达式里的常数项仍会照收，见第 5 节） |
| 400 `phone is required` / `keyWord is required`   | 必填参数缺失，见第 6.1 节字段表                                  |
| 400 `this route serves only model "getMsg"`       | 原生路由请求体里的 `model` 与路由不一致                          |
| 400 `model_price_error`                           | 没有给该模型配置按次价格或任务用量表达式                         |
| 400 `plugin_usage_invalid`                        | 计费表达式与插件的用量字段不匹配（本插件只提供 `calls`）         |
| 任务日志 `fail_reason: ERROR:...`                 | 厂商拒绝（如无可用号码），本次不计费，可直接重试                 |
| 401 / `Invalid token`                             | 调用方用的是 New API 令牌，不是第一接码 token（渠道 Key 才是）   |
| 500 `internal plugin route error`                 | v1.2.21 及更早版本的宿主缺陷：本插件用「无请求体的 GET」提交，宿主关闭空的 `req.Body` 会 panic。v1.2.22 已修复（`relay/channel/api_request.go` 增加 nil 判空），升级即可 |

---

## 11. 实现要点（维护者）

- **一次提交即一个终态任务**：`parseSubmitResponse` 直接返回 `immediate: SUCCESS/FAILURE`，
  任务落库时已是终态，因此宿主不会轮询它。
- **原生路由声明 `retainResult: false`**：结果在提交响应里一次性返回，不再保留 `task.data`
  （任务行仍保留，用于计费与日志）。
- **`buildQueryRequest` / `parseTaskResult` 是 v1 必备导出**：本插件没有任何查询路由，
  若被误调用会明确报错 / 返回 `UNKNOWN`，不会伪造任务状态。
- **用量契约**：`extractUsage` 与 `extractUsageOnComplete` 都返回 `{ calls: 1 }`；
  字段 `calls` 在 `meta.usageSchema` 中声明为按次计价（单位 `次`）。
- **未交付的读取不算交付**：`getMsg` 的响应里出现 `[尚未收到]` / `尚未收到` 时，`parseSubmitResponse`
  返回 `immediate: FAILURE`，宿主按「立即失败」处理并**全额退回**预扣额度（`任务 quota = 0`），
  因此反复读取不产生费用；`controller/plugin_native_e2e_test.go` 的
  `TestD1jiemaUndeliveredReadIsRefunded` 断言了「用户额度不变」这条契约。
  若上游是另一个网关（`upstream.kind = new_api`）且旧版插件用 `pending: true` 表示未收到，
  `readOutcome` 也会把它识别为未交付。
- **无请求体的提交**：厂商参数全部放在 query string 里，`buildSubmitRequest` 返回 `method: "GET"` 且不带
  `body`。宿主组装上游请求时 `req.Body` 为 nil，因此必须容忍空请求体（v1.2.22 起在
  `relay/channel/api_request.go` 的 `doRequest` 中判空）；`controller/plugin_native_e2e_test.go`
  的 `TestD1jiemaBodylessNativeSubmitCompletesImmediately` 覆盖了这条路径（钱包与订阅两种出资方式）。

---

## 12. 内置「接码」页面

本仓库的界面上多了一个侧边栏入口 **接码**（路由 `/sms-code`，在「通用」分组里紧跟在「离线图像」后面），
不用写脚本就能直接对上面三个模型取号、取码、发短信。

**它做什么**

| 面板         | 使用的模型 | 说明                                                                 |
| ------------ | ---------- | -------------------------------------------------------------------- |
| 取号         | `getPhone` | 填短信关键词、可选指定号码/省份/卡类型，取到一个号码                  |
| 读取短信     | `getMsg`   | 号码和关键词自动带入；可单次读取，也可打开「自动轮询」：没收到时继续等，收到短信即停且只结算这一条 |
| 发送短信     | `send`     | 用取到的号码向平台号码发送短信                                        |

取号成功后返回的号码会自动填进另外两个面板，三个面板共用同一个短信关键词。
读取面板在短信到达前会显示「尚未收到」，这类读取**不计费**（见第 5 节），所以自动轮询可以一直开着。

**API 密钥**

页面顶部的「API 密钥」从你自己的令牌列表里选（默认选第一个启用中的），所有调用都以
`Authorization: Bearer sk-...` 发到插件的原生路由 `/d1jiema/<模型>`，因此计费、限速和日志都记在这个令牌名下。
没有可用令牌时页面会提示先去「API 密钥」页创建一个。

**前置条件**

1. d1jiema 插件已启用（后台 → 任务插件）；
2. 已建立类型为「任务插件」的渠道，密钥是第一接码 token，模型包含 `getPhone`、`getMsg`、`send`；
3. 令牌所属分组的模型列表包含这三个模型，且它们已配置价格。

**构建**

前端源码改动后需要重新构建才会出现在界面上：

```bash
cd web && bun install && bun run build   # 产出 web/dist
cd .. && go build -o new-api .           # web/dist 会被嵌入二进制
```

**排查**

| 现象                                    | 处理                                                             |
| --------------------------------------- | ---------------------------------------------------------------- |
| 取号/取码按钮一直是灰的                 | 没有启用中的 API 密钥，或密钥正在加载 → 检查「API 密钥」页        |
| 401 `令牌无效`                          | 令牌被禁用/删除，重新选择一个                                          |
| 服务端返回「无可用渠道」                | 渠道未配置、未启用，或该分组的模型列表里没有这三个模型            |
| 原始返回里是厂商 `ERROR:...`            | 见第 10 节；厂商拒绝不会计费                                       |

