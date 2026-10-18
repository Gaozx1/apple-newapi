// 第一接码 (d1jiema) SMS verification-code relay plugin for New API.
//
// Vendor API (GET, UTF-8): https://api.d1jiema.com/zc/data.php
//   code=getPhone    take a number (random, or the one named by `phone`)
//   code=getMsg      read the SMS a number received
//   code=send        send an SMS from a number
//   code=leftAmount  query the balance (kept out of the model list on purpose)
//
// Every vendor call answers synchronously inside one HTTP response, so a
// submission is always terminal immediately: the plugin declares no query
// route and the host never polls it. The channel Key is the API token created
// in the vendor console (个人中心 -> API对接中心 -> 创建 API Token).
//
// Billing covers a delivered result only. The vendor answers
// "[尚未收到] ..." while the SMS has not arrived yet, and that answer is an
// ordinary retry of the same read, not a delivery: it settles as FAILURE so the
// host refunds the whole reservation, and a caller that keeps reading until the
// code lands pays for the one read that returned it. A paid retry loop is what
// the vendor's own rate note ("每请求1000次") is about, so keep re-reading
// instead of re-taking a number.
//
// Surfaces:
//   POST /d1jiema/getPhone  {"model":"getPhone","keyWord":"毛竹","phone":"130xxxxxxxx"}
//   POST /d1jiema/getMsg    {"model":"getMsg","phone":"130xxxxxxxx","keyWord":"毛竹"}
//   POST /d1jiema/send      {"model":"send","phone":"162xxxxxxxx","toPhone":"1069xxxxxxxx","content":"hi"}
//   POST /v1/responses      {"model":"getMsg","input":"{\"phone\":\"130...\",\"keyWord\":\"毛竹\"}"}

export const meta = {
  apiVersion: 1,
  key: "d1jiema",
  name: "第一接码",
  icon: "text",
  description: {
    en: "Take phone numbers and read or send their SMS through the d1jiema API",
    zh: "通过第一接码接口取号、取码、发短信",
  },
  version: "1.0.1",
  author: { name: "YG" },
  website: "https://www.d1jiema.com/",
  baseUrl: "https://api.d1jiema.com",
  models: ["getPhone", "getMsg", "send"],
  fetchMode: "per_task",
  // The same plugin can drive a second New API gateway that installed it.
  upstreams: ["vendor", "new_api"],
  // One delivered operation is one billable call. A getMsg read that has not
  // received anything yet settles as FAILURE and is refunded in full, so each
  // model carries its own price (or task expression).
  usageSchema: {
    calls: {
      type: "number",
      unit: "count",
      unitLabel: { en: "call", zh: "次" },
      description: { en: "SMS relay call unit price", zh: "接码调用单价" },
    },
  },
  routes: [
    { method: "POST", path: "/d1jiema/getPhone", type: "submit", decode: "decodeGetPhone", render: "renderResult", retainResult: false },
    { method: "POST", path: "/d1jiema/getMsg", type: "submit", decode: "decodeGetMsg", render: "renderResult", retainResult: false },
    { method: "POST", path: "/d1jiema/send", type: "submit", decode: "decodeSend", render: "renderResult", retainResult: false },
  ],
  protocols: [{ name: "openai_responses", supports: ["stream", "sync", "background"] }],
};

const OPERATIONS = ["getPhone", "getMsg", "send"];
const PARAM_NAMES = ["keyWord", "phone", "province", "cardType", "toPhone", "projId", "content"];
const CARD_TYPES = ["实卡", "虚卡", "全部"];
// The operation resolves from its own params when `model` is absent; a bare
// `input` string carries the SMS keyword (getPhone/getMsg) or the SMS body
// (send) instead of a JSON object.
const MESSAGE_PARAM = { getPhone: "keyWord", getMsg: "keyWord", send: "content" };
const VENDOR_PATH = "/zc/data.php";

function trimmed(value) {
  return value === undefined || value === null ? "" : String(value).trim();
}

function objectOf(value) {
  return value && typeof value === "object" && !Array.isArray(value) ? value : {};
}

// The channel base URL may be the vendor origin or the full data.php endpoint.
function endpoint(baseUrl) {
  const base = trimmed(baseUrl).replace(/\/+$/, "");
  if (base === "") return "";
  return /\/zc\/data\.php$/i.test(base) ? base : base + VENDOR_PATH;
}

function queryString(pairs) {
  const parts = [];
  for (const pair of pairs) {
    if (trimmed(pair[1]) === "") continue;
    parts.push(encodeURIComponent(pair[0]) + "=" + encodeURIComponent(String(pair[1])));
  }
  return parts.join("&");
}

function normalizePhone(value) {
  const cleaned = trimmed(value).replace(/[\s-]/g, "");
  const country = /^(\+?86)(1\d{10})$/.exec(cleaned);
  return country ? country[2] : cleaned;
}

function operationOf(action, upstreamModel, model) {
  const candidates = [action, upstreamModel, model];
  for (const candidate of candidates) {
    const name = trimmed(candidate);
    if (OPERATIONS.indexOf(name) >= 0) return name;
  }
  throw new Error("unknown d1jiema operation; expected one of " + OPERATIONS.join(", "));
}

function protocolOperation(ctx) {
  const upstream = trimmed(ctx.upstreamModel);
  if (OPERATIONS.indexOf(upstream) >= 0) return upstream;
  const declared = trimmed(ctx.model);
  if (OPERATIONS.indexOf(declared) >= 0) return declared;
  throw new Error("model must be one of " + OPERATIONS.join(", "));
}

function normalizeParams(operation, source) {
  const input = objectOf(source);
  const value = function (name) {
    return trimmed(input[name]);
  };
  const params = {};

  if (operation === "getPhone") {
    const cardType = value("cardType");
    if (cardType !== "" && CARD_TYPES.indexOf(cardType) < 0) {
      throw new Error("cardType must be one of 实卡, 虚卡, 全部");
    }
    params.keyWord = value("keyWord");
    params.phone = normalizePhone(value("phone"));
    params.province = value("province");
    params.cardType = cardType === "" ? "全部" : cardType;
    return params;
  }

  if (operation === "getMsg") {
    params.phone = normalizePhone(value("phone"));
    params.keyWord = value("keyWord");
    if (params.phone === "") throw new Error("phone is required: pass the number returned by getPhone");
    if (params.keyWord === "") throw new Error("keyWord is required: use the SMS signature, for example 毛竹");
    return params;
  }

  params.phone = normalizePhone(value("phone"));
  params.toPhone = value("toPhone");
  params.projId = value("projId");
  params.content = value("content");
  if (params.phone === "") throw new Error("phone is required: pass the number returned by getPhone");
  if (params.toPhone === "") throw new Error("toPhone is required");
  if (params.content === "") throw new Error("content is required");
  return params;
}

function copyParams(source, target) {
  for (const name of PARAM_NAMES) {
    if (Object.prototype.hasOwnProperty.call(source, name) && trimmed(source[name]) !== "") {
      target[name] = source[name];
    }
  }
}

function parseJSONObject(text) {
  if (text === "" || text.charAt(0) !== "{") return null;
  try {
    const parsed = JSON.parse(text);
    return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

function responsesText(input) {
  if (typeof input === "string") return input.trim();
  if (!Array.isArray(input)) return "";
  const texts = [];
  for (const item of input) {
    if (typeof item === "string") {
      texts.push(item);
      continue;
    }
    if (!item || typeof item !== "object" || Array.isArray(item)) continue;
    const parts = item.content === undefined ? [item] : Array.isArray(item.content) ? item.content : [item.content];
    for (const part of parts) {
      if (typeof part === "string") {
        texts.push(part);
        continue;
      }
      if (!part || typeof part !== "object" || Array.isArray(part)) continue;
      if (typeof part.text === "string") texts.push(part.text);
    }
  }
  return texts.join("\n").trim();
}

// Responses requests carry the vendor params either as top-level fields, in
// `metadata`, or inside a JSON-string `input`; a plain-text `input` is the
// keyword/body shortcut.
function protocolParams(body, operation) {
  const params = {};
  copyParams(objectOf(body.metadata), params);
  const text = responsesText(body.input);
  const parsed = parseJSONObject(text);
  if (parsed) copyParams(parsed, params);
  copyParams(body, params);
  const messageParam = MESSAGE_PARAM[operation];
  if (!parsed && text !== "" && trimmed(params[messageParam]) === "") params[messageParam] = text;
  return params;
}

function decodeNative(ctx, operation) {
  if (!ctx.body || ctx.body.kind !== "json") throw new Error("JSON body required");
  const body = ctx.body.value;
  if (!body || typeof body !== "object" || Array.isArray(body)) throw new Error("request body must be a JSON object");
  const claimed = trimmed(body.model);
  if (claimed !== "" && claimed !== operation) throw new Error('this route serves only model "' + operation + '"');
  return { kind: "submit", model: operation, action: operation, requestBody: normalizeParams(operation, body) };
}

function vendorText(body) {
  if (body === undefined || body === null) return "";
  if (typeof body === "string") return body.trim();
  if (typeof body === "object") {
    const keys = ["message", "msg", "data", "result", "text", "content"];
    for (const key of keys) {
      if (typeof body[key] === "string" && body[key].trim() !== "") return body[key].trim();
    }
    return JSON.stringify(body);
  }
  return trimmed(body);
}

function isVendorError(text) {
  return /^ERROR\s*[:：]/i.test(text);
}

// The vendor says "[尚未收到]" (or "尚未收到...") while the SMS has not been
// delivered. The read is then a retry, never a billable delivery.
function isNotReceived(text) {
  return text.indexOf("尚未收到") >= 0 || /\[\s*未收到\s*\]/.test(text);
}

function notReceivedReason(text) {
  const detail = trimmed(text);
  return detail === "" ? "尚未收到短信（本次不计费），可继续读取" : "尚未收到短信（本次不计费），可继续读取：" + detail;
}

function summarize(operation, text, params) {
  const data = { operation: operation, raw: text };
  if (operation === "getPhone") {
    if (/^\+?\d{6,20}$/.test(text)) data.phone = text;
    else if (text !== "") data.message = text;
    return data;
  }
  if (operation === "getMsg") {
    data.phone = normalizePhone((params || {}).phone);
    if (text !== "") data.message = text;
    data.pending = text.indexOf("未收到") >= 0;
    return data;
  }
  if (text !== "") data.message = text;
  return data;
}

// The vendor answers plain text; a second New API gateway answers with this
// plugin's own JSON envelope instead. `notReceived` marks a read that has not
// delivered anything yet, which is never billable.
function readOutcome(ctx, resp) {
  const body = resp ? resp.body : null;
  if (!(ctx.upstream && ctx.upstream.kind === "new_api")) {
    const text = vendorText(body);
    return { ok: !isVendorError(text), text: text, reason: text, notReceived: isNotReceived(text) };
  }
  const payload = objectOf(body);
  const text = trimmed(payload.raw) !== "" ? trimmed(payload.raw) : trimmed(payload.message);
  const ok = trimmed(payload.code) === "ok";
  const reason = trimmed(payload.reason) !== "" ? trimmed(payload.reason) : trimmed(payload.message) || text;
  // A gateway running an older copy of this plugin reports an undelivered read
  // as `pending` instead of failing it; treat both shapes the same way.
  const notReceived = payload.pending === true || isNotReceived(text) || isNotReceived(reason);
  return { ok: ok, text: text, reason: reason, notReceived: notReceived };
}

function resultText(data) {
  const value = objectOf(data);
  return trimmed(value.raw) || trimmed(value.message) || "第一接码未返回内容";
}

function resultMetadata(data) {
  const value = objectOf(data);
  const metadata = { operation: trimmed(value.operation) };
  if (trimmed(value.phone) !== "") metadata.phone = trimmed(value.phone);
  if (value.pending !== undefined) metadata.pending = value.pending ? "true" : "false";
  return metadata;
}

export function buildSubmitRequest(ctx) {
  const operation = operationOf(ctx.action, ctx.upstreamModel, ctx.model);
  const params = normalizeParams(operation, ctx.requestBody);
  const token = trimmed(ctx.apiKey);
  if (token === "") throw new Error("channel key is empty: paste the d1jiema API token into the channel Key field");

  if (ctx.upstream && ctx.upstream.kind === "new_api") {
    // The upstream gateway answers this plugin's own native route.
    return {
      url: ctx.baseUrl + "/d1jiema/" + operation,
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: "Bearer " + token },
      body: params,
      action: operation,
    };
  }

  const pairs = [
    ["code", operation],
    ["token", token],
  ];
  for (const name of PARAM_NAMES) {
    if (trimmed(params[name]) !== "") pairs.push([name, params[name]]);
  }
  return { url: endpoint(ctx.baseUrl) + "?" + queryString(pairs), method: "GET", headers: {}, action: operation };
}

export function parseSubmitResponse(ctx, resp) {
  const operation = operationOf(ctx.action, ctx.upstreamModel, ctx.model);
  const outcome = readOutcome(ctx, resp);
  const taskId = "d1j_" + utils.uuid();
  if (!outcome.ok) {
    const reason = outcome.reason !== "" ? outcome.reason : "第一接码拒绝了本次请求";
    return {
      taskId: taskId,
      taskData: { operation: operation, error: reason },
      immediate: { status: "FAILURE", progress: "100%", reason: reason },
    };
  }
  if (operation === "getMsg" && outcome.notReceived) {
    // Nothing delivered yet: fail the read so the host refunds the whole
    // reservation. The caller reads again until the code arrives, and only the
    // read that returns it is billed.
    const reason = notReceivedReason(outcome.text !== "" ? outcome.text : outcome.reason);
    return {
      taskId: taskId,
      taskData: {
        operation: operation,
        phone: normalizePhone((ctx.requestBody || {}).phone),
        pending: true,
        raw: outcome.text,
        error: reason,
      },
      immediate: { status: "FAILURE", progress: "100%", reason: reason },
    };
  }
  return {
    taskId: taskId,
    taskData: summarize(operation, outcome.text, ctx.requestBody),
    immediate: { status: "SUCCESS", progress: "100%" },
  };
}

export function extractUsage(_ctx) {
  return { calls: 1 };
}

export function extractUsageOnComplete(_task, _result, _data) {
  return { calls: 1 };
}

// Required v1 polling hooks. Every submission is already terminal, so no task
// this plugin creates is ever polled; the hooks stay for contract
// compatibility and report UNKNOWN rather than inventing a status.
export function buildQueryRequest(_ctx) {
  throw new Error("d1jiema submissions complete at submission time and are never polled");
}

export function parseTaskResult(_ctx, _body) {
  return { status: "UNKNOWN" };
}

export const native = {
  decodeGetPhone: function (ctx) {
    return decodeNative(ctx, "getPhone");
  },
  decodeGetMsg: function (ctx) {
    return decodeNative(ctx, "getMsg");
  },
  decodeSend: function (ctx) {
    return decodeNative(ctx, "send");
  },
  renderResult: function (ctx, task) {
    const data = objectOf(task.data);
    const ok = String(task.status || "").toUpperCase() === "SUCCESS";
    const result = { code: ok ? "ok" : "error", operation: trimmed(data.operation), task_id: trimmed(task.task_id) };
    if (trimmed(data.phone) !== "") result.phone = trimmed(data.phone);
    if (trimmed(data.message) !== "") result.message = trimmed(data.message);
    if (data.pending !== undefined) result.pending = !!data.pending;
    if (trimmed(data.raw) !== "") result.raw = trimmed(data.raw);
    if (!ok) result.reason = trimmed(task.fail_reason) || trimmed(data.error) || "第一接码拒绝了本次请求";
    return result;
  },
  error: function (ctx, error) {
    return { code: "error", message: trimmed(error && error.message), http_status: error ? error.httpStatus : 400 };
  },
};

export const protocols = {
  openai_responses: {
    decodeRequest: function (ctx) {
      if (!ctx.body || ctx.body.kind !== "json") throw new Error("JSON body required");
      const body = ctx.body.value;
      if (!body || typeof body !== "object" || Array.isArray(body)) throw new Error("request body must be a JSON object");
      const operation = protocolOperation(ctx);
      return {
        kind: "submit",
        // Keep the client-facing name (a channel mapping alias must survive).
        model: trimmed(ctx.model) || operation,
        action: operation,
        requestBody: normalizeParams(operation, protocolParams(body, operation)),
      };
    },
    renderEvents: function (ctx, task, previousState) {
      const status = String((task && task.status) || "UNKNOWN").toUpperCase();
      const state = { status: status };
      if (status !== "SUCCESS") return { events: [], state: state, done: false };
      const repeated = previousState && previousState.status === status;
      const events = repeated ? [] : [{ type: "output", data: resultText(task && task.data) }];
      return { events: events, state: state, done: true };
    },
    renderFinal: function (ctx, task) {
      const data = objectOf(task && task.data);
      return {
        output: [
          {
            type: "message",
            status: "completed",
            role: "assistant",
            content: [{ type: "output_text", text: resultText(data), annotations: [], logprobs: [] }],
          },
        ],
        metadata: resultMetadata(data),
      };
    },
  },
};
