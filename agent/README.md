# koptan-agent

The CLI that runs inside a Koptan SelfService session pod. It turns prompts into
commits: it syncs the repository, lets a model change the code through a small
set of tools, then commits and pushes. Koptan picks the push up and builds and
deploys it like any Service.

```sh
koptan-agent serve            # HTTP API on $KOPTAN_AGENT_PORT (8080)
koptan-agent run "add a /health endpoint"
```

## API

All routes except `/healthz` need `Authorization: Bearer $KOPTAN_AGENT_TOKEN`.

- `GET /healthz`: `{ ok, busy }`.
- `GET /runs`: the last 50 runs, newest first.
- `POST /runs` with `{"prompt": "..."}`: runs it and streams server-sent events
  (`status`, `message`, `tool`, `tool_result`, then `done` with the commit or
  `error`). One run at a time; a second gets 409.

## Models

`KOPTAN_AI_PROVIDER` picks the provider:

- `anthropic`: the Messages API through `@anthropic-ai/sdk`. `KOPTAN_AI_MODEL`
  (e.g. `claude-opus-5-5`) and `KOPTAN_AI_API_KEY`.
- `openai-compatible`: any `/v1/chat/completions` server with
  `KOPTAN_AI_BASE_URL`: OpenAI, Azure OpenAI, OpenRouter, LiteLLM, or a local
  Ollama (`http://ollama:11434/v1`), vLLM or llama.cpp server. No key is needed
  for local servers.

Models without tool calling (`KOPTAN_AI_TOOLS=false`, or detected when the
server rejects tools) get a JSON edit plan instead, validated before it is applied.

## Safety

- File tools resolve every path inside the workspace; `..`, absolute paths,
  symlinks out of the repository and `.git` are refused.
- `run_command` exists only with `KOPTAN_AGENT_ALLOW_COMMANDS=true`; it runs
  with a minimal environment that holds no git or AI credentials.
- The git token reaches git only through `GIT_ASKPASS`; it is never put in a
  URL or a file in the repository, and it is masked in logs.

## Development

```sh
npm install
npm test
docker build -t koptan-agent .
```
