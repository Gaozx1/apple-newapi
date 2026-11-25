// Standard OpenAI tool relay for New API.
//
// Surfaces (one tool per model name; the body is the vendor's own JSON):
//   POST /v1/tools/search   {"query": "deepseek", "count": 3}
//   POST /v1/tools/fetch    {"url": "https://example.com/"}
//   POST /v1/tools/search2  ...
// The gateway serves the same route under /tools/v1/tools/:model as well.
//
// The vendor endpoint is the channel Base URL plus the same path,
// /v1/tools/<model>: the model name in the path is the tool name, and a channel
// model_mapping may send a declared model to the vendor's own tool id
// (ctx.upstreamModel then names the path).
//
// Billing covers a delivered call only. Every vendor call answers inside one
// HTTP response, so a submission is terminal immediately and nothing is polled.
// A 2xx reply is one billed call (usage fact calls = 1); a 2xx body carrying an
// OpenAI-style `error` object settles as FAILURE, so the host refunds the whole
// reservation, and a non-2xx status fails before this plugin sees the body.
// Administrators price each tool per call with a task expression such as
// u("calls") * 0.02 (USD per call) for the model in billing_setting.billing_expr,
// or per tool in billing_setting.plugin_billing_expr under tools::<model>.
//
// These two paths are served by the gateway's own DeepSeek tools relay unless a
// plugin route answers them first, so this plugin takes those requests over
// while it is enabled; disabling it restores the relay.

const TOOLS = ["search", "fetch", "search2", "ocr"];
const TOOL_PATH = "/v1/tools/";

export const meta = {
  apiVersion: 1,
  key: "tools",
  name: "OpenAI Tools",
  icon: "text",
  description: {
    en: "Relay standard OpenAI tool calls to the channel's /v1/tools endpoint",
    zh: "把标准 OpenAI 工具调用转发到渠道的 /v1/tools 接口",
  },
  version: "1.0.1",
  author: { name: "YG" },
  models: TOOLS,
  fetchMode: "per_task",
  upstreams: ["vendor", "new_api"],
  // One delivered call is one billable call, whatever the tool answered with.
  usageSchema: {
    calls: {
      type: "number",
      unit: "count",
      unitLabel: { en: "call", zh: "次" },
      description: { en: "Tool call unit price", zh: "工具调用单价" },
    },
  },
  routes: [
    {
      method: "POST",
      path: "/v1/tools/:model",
      type: "submit",
      action: "tool_call",
      decode: "toolCall",
      render: "toolResult",
      // The vendor answers synchronously, so every submission is terminal and
      // nothing is persisted or re-queryable.
      retainResult: false,
    },
  ],
  // The same tool is reachable through the Responses API: the requested model
  // names the tool, a plain-text input becomes its single query field, and the
  // reply is rendered as text.
  protocols: [{ name: "openai_responses", supports: ["stream", "sync", "background"] }],
};

function trimmed(value) {
  return value === undefined || value === null ? "" : String(value).trim();
}

function objectOf(value) {
  return value && typeof value === "object" && !Array.isArray(value) ? value : null;
}

// The declared spelling of a tool name; a case variant names the same tool.
function declaredTool(name) {
  const wanted = trimmed(name).toLowerCase();
  for (const tool of TOOLS) {
    if (tool === wanted) return tool;
  }
  return "";
}

// The name the vendor endpoint uses: the channel mapping's target when there is
// one, otherwise the declared spelling of the client's model.
function vendorTool(ctx) {
  const mapped = trimmed(ctx.upstreamModel);
  if (mapped !== "") return mapped;
  return declaredTool(ctx.model) || trimmed(ctx.model);
}

function toolPath(ctx) {
  const tool = vendorTool(ctx);
  if (tool === "") throw new Error("model is required");
  const base = trimmed(ctx.baseUrl).replace(/\/+$/, "");
  if (base === "") {
    throw new Error("channel Base URL is empty: set the upstream origin that serves " + TOOL_PATH);
  }
  // A New API channel runs the same plugin, which serves its routes under the
  // plugin's own prefix.
  const prefix = ctx.upstream && ctx.upstream.kind === "new_api" ? "/" + meta.key : "";
  return base + prefix + TOOL_PATH + encodeURIComponent(tool);
}

function vendorError(body) {
  const value = objectOf(body);
  if (!value) return "";
  if (typeof value.error === "string") return trimmed(value.error);
  const error = objectOf(value.error);
  if (!error) return "";
  return trimmed(error.message) || trimmed(error.type);
}

// The body member a plain-text Responses input fills for each tool.
const QUERY_FIELD = { search: "query", search2: "query", fetch: "url", ocr: "url" };

function parseJSONObject(text) {
  if (text === "" || text.charAt(0) !== "{") return null;
  try {
    return objectOf(JSON.parse(text));
  } catch {
    return null;
  }
}

// Responses requests carry the tool body either as a JSON-object input or as the
// plain text the tool queries for.
function toolParams(tool, text) {
  const parsed = parseJSONObject(text);
  if (parsed) return parsed;
  const params = {};
  params[QUERY_FIELD[tool] || "query"] = text;
  return params;
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

function resultText(data) {
  const value = objectOf(data) || {};
  if (typeof value.body === "string") return trimmed(value.body) || "the tool returned no content";
  if (value.body !== undefined && value.body !== null) return JSON.stringify(value.body);
  return trimmed(value.error) || "the tool returned no content";
}

export function buildSubmitRequest(ctx) {
  if (!objectOf(ctx.requestBody)) throw new Error("requestBody must be a JSON object");
  const token = trimmed(ctx.apiKey);
  if (token === "") {
    throw new Error("channel key is empty: paste the upstream API token into the channel Key field");
  }
  const url = toolPath(ctx);
  const headers = { Authorization: "Bearer " + token };
  const body = ctx.requestBody;
  if (vendorTool(ctx) === "ocr") {
    const files = ctx.files || [];
    if (files.length === 1 && files[0].field === "file") {
      const parts = [{ name: "file", fileRef: files[0].ref }];
      for (const field of ["purpose", "pdf_dpi"]) {
        if (body[field] !== undefined) parts.push({ name: field, value: body[field] });
      }
      return { url, method: "POST", headers, bodyType: "multipart", parts };
    }
    const source = trimmed(body.file) || trimmed(body.url);
    // Some OCR vendors also accept a local path. Never send a path from an API caller.
    if (!/^https?:\/\/[^\s/?#\\]+(?:[/?#][^\s\\]*)?$/i.test(source)) {
      throw new Error("OCR requires an uploaded file or an absolute HTTP(S) URL");
    }
    const ocr = { ...body, url: source };
    delete ocr.file;
    headers["Content-Type"] = "application/json";
    return { url, method: "POST", headers, body: ocr };
  }
  headers["Content-Type"] = "application/json";
  return { url, method: "POST", headers, body };
}

export function parseSubmitResponse(ctx, resp) {
  const tool = vendorTool(ctx);
  const taskId = "tool_" + utils.uuid();
  const body = resp ? resp.body : null;
  const failure = vendorError(body);
  if (failure !== "") {
    // Nothing was delivered: fail the task so the host refunds the reservation.
    return {
      taskId: taskId,
      taskData: { tool: tool, error: failure },
      immediate: { status: "FAILURE", progress: "100%", reason: failure },
    };
  }
  return {
    taskId: taskId,
    taskData: { tool: tool, body: body },
    immediate: { status: "SUCCESS", progress: "100%" },
  };
}

export function extractUsage(_ctx) {
  return { calls: 1 };
}

export function extractUsageOnComplete(_ctx, _result, _data) {
  return { calls: 1 };
}

// Required v1 polling hooks. Every submission is already terminal, so no task
// this plugin creates is ever polled; the hooks stay for contract compatibility
// and report UNKNOWN rather than inventing a status.
export function buildQueryRequest(_ctx) {
  throw new Error("tools submissions complete at submission time and are never polled");
}

export function parseTaskResult(_ctx, _body) {
  return { status: "UNKNOWN" };
}

function failureReason(task, data) {
  return (
    trimmed(task && task.fail_reason) ||
    trimmed(data && data.error) ||
    "the tool call failed"
  );
}

export const native = {
  toolCall: function (ctx) {
    const tool = declaredTool((ctx.params || {}).model);
    if (tool === "") throw new Error("model must be one of " + TOOLS.join(", "));
    if (tool === "ocr" && ctx.body && ctx.body.kind === "multipart") {
      const files = ctx.body.files || [];
      if (files.length !== 1 || files[0].field !== "file") {
        throw new Error("OCR requires exactly one file part named file");
      }
      const fields = ctx.body.fields || {};
      const requestBody = {};
      for (const field of ["purpose", "pdf_dpi"]) {
        if (fields[field] && fields[field].length) requestBody[field] = fields[field][0];
      }
      return { kind: "submit", model: tool, action: "tool_call", requestBody };
    }
    if (!ctx.body || ctx.body.kind !== "json") throw new Error("JSON body required");
    const body = objectOf(ctx.body.value);
    if (!body) throw new Error("request body must be a JSON object");
    const claimed = trimmed(body.model);
    if (claimed !== "" && declaredTool(claimed) !== tool) {
      throw new Error('this route serves only model "' + tool + '"');
    }
    return { kind: "submit", model: tool, action: "tool_call", requestBody: body };
  },
  toolResult: function (ctx, task) {
    const data = objectOf(task.data) || {};
    if (String(task.status || "").toUpperCase() === "SUCCESS") {
      const body = data.body;
      // The vendor's own payload is the reply; a plain string keeps its shape
      // under a result member instead of turning into an invalid JSON body.
      if (body !== undefined && body !== null && typeof body === "object") return body;
      return { result: body === undefined || body === null ? "" : body };
    }
    return { error: { message: failureReason(task, data), type: "tool_error", code: "tool_call_failed" } };
  },
  error: function (ctx, error) {
    return { error: { message: trimmed(error && error.message), type: "tool_error" } };
  },
};

export const protocols = {
  openai_responses: {
    decodeRequest: function (ctx) {
      if (!ctx.body || ctx.body.kind !== "json") throw new Error("JSON body required");
      const body = objectOf(ctx.body.value);
      if (!body) throw new Error("request body must be a JSON object");
      const tool = declaredTool(ctx.upstreamModel) || declaredTool(ctx.model);
      if (tool === "") throw new Error("model must be one of " + TOOLS.join(", "));
      return {
        kind: "submit",
        // Keep the client-facing name (a channel mapping alias must survive).
        model: trimmed(ctx.model) || tool,
        action: "tool_call",
        requestBody: toolParams(tool, responsesText(body.input)),
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
      const data = objectOf(task && task.data) || {};
      const result = {
        output: [
          {
            type: "message",
            status: "completed",
            role: "assistant",
            content: [{ type: "output_text", text: resultText(data), annotations: [], logprobs: [] }],
          },
        ],
      };
      if (trimmed(data.tool) !== "") result.metadata = { tool: trimmed(data.tool) };
      return result;
    },
  },
};