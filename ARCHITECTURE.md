# Unbound CLI — Architecture

## Overview

Unbound is a desktop/local CLI tool that downloads, hosts, and runs open-source AI models entirely on the user's machine — no cloud, no censorship, no restrictions. Models get full system permissions (shell access, file I/O, code execution, internet) with explicit user consent.

## Architecture Diagram

```
+---------------------------+
|      unbound CLI (cobra)   |
+---------------------------+
       |         |        |
  +----+   +----+---+ +--+---+
  | run |   | serve | | pull |  ... (list, remove)
  +--+---+   +---+---+ +--+---+
     |           |        |
     v           v        v
+--------+ +---------+ +---------+
| console| | server  | | model   |
| (TUI)  | | (API)   | | (mgr)  |
+--------+ +---------+ +---------+
     |           |        |
     v           v        v
+----------------------------------+
|           pkg/                    |
|  +----------+  +-----------+     |
|  | perms    |  | shell     |     |
|  | (consent)|  | (exec)    |     |
|  +----------+  +-----------+     |
|  +----------+  +-----------+     |
|  | config   |  | model     |     |
|  | (viper)  |  | (llama)   |     |
|  +----------+  +-----------+     |
+----------------------------------+
```

## Package Structure

```
bare-metal/
├── main.go                  # Entry point, dispatches to cobra
├── cmd/
│   ├── run/                 # `unbound run <model>` — interactive chat
│   ├── serve/               # `unbound serve <model>` — API server
│   ├── pull/                # `unbound pull <model>` — download models
│   ├── list/                # `unbound list` — list cached models
│   └── remove/              # `unbound remove <model>` — delete models
├── pkg/
│   ├── model/               # Model download, caching, lifecycle, inference
│   ├── server/              # OpenAI-compatible API server
│   ├── permissions/          # User consent and permission prompting
│   ├── shell/               # Shell execution with permission gates
│   └── console/             # Interactive terminal chat TUI
└── internal/
    └── config/              # Configuration management (viper)
```

## Key Design Decisions

### 1. Go + Cobra for CLI
Go produces a single-binary cross-platform executable. Cobra provides standard CLI patterns (commands, flags, help text, autocompletion).

### 2. Permission System
Every system-level action (shell command, file I/O, network access, code execution) requires explicit user consent. The permission system uses interactive prompts with `y/N/a` (allow once / deny / allow always). A `--no-permissions` flag enables sandboxed mode. An `UNBOUND_ALLOW_ALL` env var enables automation use.

### 3. Model Management
Models are downloaded from Hugging Face Hub and cached in `~/.unbound/models/`. The `model` package handles resolution of model aliases to HF repo IDs, streaming downloads with progress, and loading/unloading for inference.

### 4. OpenAI-Compatible API
The `server` package implements an HTTP server at `/v1/chat/completions` following the OpenAI API schema. This allows integration with any OpenAI-compatible client (LangChain, Open Interpreter, custom UIs, etc.).

### 5. Interactive Console
The console uses a line-by-line terminal interface with slash commands (`/allow`, `/deny`, `/exit`, `/help`). Future versions will integrate Bubble Tea or Charm for a richer TUI.

## Data Flow

### `unbound run <model>`
1. CLI resolves model name → checks local cache
2. If not cached, downloads from Hugging Face (with progress)
3. Loads model via llama.cpp bindings
4. Opens interactive chat session
5. For each user message: run inference → display response
6. When model requests a system action: prompt user for consent → execute or deny

### `unbound serve <model>`
1. Same model loading as `run`
2. Starts HTTP server on configurable host:port
3. Listens for OpenAI-compatible requests
4. Routes `/v1/chat/completions` to model inference
5. Applies same permission gating per request

## Future Enhancements

- [ ] **Bubble Tea / Charm TUI** for richer interactive experience
- [ ] **llama.cpp bindings** (Go wrapper or CGO) for actual model inference
- [ ] **Streaming responses** via SSE for /v1/chat/completions
- [ ] **Model fine-tuning** support
- [ ] **Persistent agent memory** across sessions
- [ ] **Sandboxed execution** environments (Docker/nsjail)
- [ ] **Web UI** companion app
- [ ] **Plugin system** for custom tools and functions