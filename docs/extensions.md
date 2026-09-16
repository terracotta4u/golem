# Extension protocol

Golem runs installed Python packages as subprocesses. Each child gets `GOLEM_URL` (the local server) and `GOLEM_TOKEN`. All `/v1` requests, in both directions, use `Authorization: Bearer <token>` and `Content-Type: application/json`.

There are two roles. A process can do both.

- **Channel:** call Golem (Telegram, etc.).
- **Provider:** Golem calls you for chat, structured output, or embeddings.

## Channel API (extension → Golem)

`GET /v1/health` — `{"ok": true}` when the token is valid. Use this to wait until Golem is listening.

`POST /v1/conversations/{id}/turns`

```json
{"channel": "telegram", "text": "hello"}
```

`202` → `{"id": "<turn-id>"}`. `{id}` is the conversation id (a Telegram chat id, a Slack thread, etc.).

`GET /v1/turns/{id}` — SSE (`text/event-stream`). Events:

```
event: log
data: {"line":"[read] ...","name":"read","args":"...","result":"..."}

event: done
data: {"text":"assistant reply"}

event: error
data: {"error":"..."}
```

The stream ends on `done` or `error`.

## Registration (extension → Golem)

After binding a loopback HTTP server, register. Re-registering the same `name` replaces the previous callback (new port after restart).

`POST /v1/extensions/register`

```json
{
  "name": "golem-openrouter",
  "callback_url": "http://127.0.0.1:9",
  "capabilities": [
    {
      "kind": "provider",
      "id": "openrouter",
      "chat": true,
      "structured": true,
      "embed": true
    }
  ]
}
```

`200` → `{"ok": true}`.

`callback_url` must be `http` with host `127.0.0.1`, `localhost`, or `::1`. No userinfo. Golem posts to the advertised routes (`/v1/chat`, `/v1/chat/structured`, `/v1/embed`).

`kind` is required. `kind: "provider"` also requires `id` (the name used in conf `default_model.provider` or `memory.embedding.provider`) and at least one of `chat`, `structured`, or `embed`. `structured` counts as chat. `chat` is optional when `embed` is set (an embeddings-only backend). Unknown kinds are stored and listed; Golem does not call them.

Embeddings-only example:

```json
{"kind": "provider", "id": "local-embed", "embed": true}
```

Two live extensions cannot share a provider `id` (`409`).

`POST /v1/extensions/heartbeat`

```json
{"name": "golem-openrouter"}
```

`200` → `{"ok": true}`. Registration expires **30 seconds** after the last successful register or heartbeat. Heartbeat must run on its own timer, not on the thread that handles `/v1/chat` — a long model call must not look like a dead process. Expiry, `golem extension remove`, and stopping the process drop hub entries. After expiry, register again; heartbeat on an unknown name is `404`.

`GET /v1/extensions` — live registrations (`name`, `callback_url`, `capabilities`).

## Callback API (Golem → extension)

Same bearer token. Model is sent on every call.

### `POST /v1/chat`

Request:

```json
{
  "model": "openai/gpt-4o-mini",
  "messages": [
    {"role": "user", "content": "read foo.go"}
  ],
  "tools": [
    {
      "name": "read",
      "description": "read a file",
      "parameters": {"type": "object"}
    }
  ]
}
```

`tools` is omitted when empty. Message fields: `role`, `content`, `tool_calls`, `tool_call_id`. A tool call is `{"id","type","function":{"name","arguments"}}` (`arguments` is a JSON string).

`200` response is one message (assistant text and/or `tool_calls`).

### `POST /v1/chat/structured`

Request:

```json
{
  "model": "openai/gpt-4o-mini",
  "messages": [{"role": "user", "content": "extract"}],
  "schema": {
    "name": "memories",
    "strict": true,
    "schema": {"type": "object"}
  }
}
```

`200` → `{"data": { ... }}` where `data` is the JSON value matching `schema`.

If the model cannot do structured output, respond with a non-200 body:

```json
{"error": {"code": "unsupported_format", "message": "no json schema"}}
```

Golem then falls back to plain chat.

### `POST /v1/embed`

Request:

```json
{
  "model": "openai/text-embedding-3-small",
  "texts": ["hello", "world"]
}
```

`200` → `{"vectors": [[0.1, 0.2], [0.3, 0.4]]}` in input order.

### Errors

```json
{"error": {"code": "optional_code", "message": "human readable"}}
```

Any `error` object is a failure. Only `code: "unsupported_format"` is special.
