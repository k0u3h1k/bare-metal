# Unbound

**Run open-source AI models locally — with full system access, entirely on your machine.**

Unbound is a desktop/local CLI tool that downloads, hosts, and runs open-source AI models entirely on your machine. No cloud, no censorship, no restrictions. Models can be given full system permissions (shell access, file I/O, code execution, internet) with your explicit consent.

## Features

- 🚀 **Run any open-source model locally** — download from Hugging Face and run with one command
- 🔐 **Explicit permission system** — every system action requires your approval
- 💬 **Interactive chat** — talk to models in your terminal
- 🌐 **OpenAI-compatible API** — use any OpenAI client to interact with local models
- 📦 **Model management** — `pull`, `list`, `remove` commands for easy model lifecycle
- 🔧 **Full autonomy** — give models real power on your own hardware

## Quick Start

```bash
# Install with one command
curl -fsSL https://raw.githubusercontent.com/k0u3h1k/bare-metal/main/scripts/install.sh | sh

# Download and run a model
unbound run llama3.2:1b

# Or serve it as an API
unbound serve mistral:7b --port 8080
```

## Usage

### `unbound run <model-name>`

Downloads (if needed) and starts an interactive chat with a model.

```bash
unbound run llama3.2:1b
```

### `unbound serve <model-name>`

Starts a headless OpenAI-compatible API server.

```bash
unbound serve llama3.2:1b --port 8080
```

### `unbound pull <model-name>`

Downloads a model from Hugging Face Hub.

```bash
unbound pull meta-llama/Llama-3.2-1B-Instruct-GGUF
```

### `unbound list`

Lists locally cached models.

### `unbound remove <model-name>`

Removes a cached model.

## Permission System

Unbound's permission system is the core differentiator. Before a model can execute a shell command, read/write a file, or access the network, you see a prompt:

```
💻  [Model] wants to perform a shell command:
   Command: ls -la /home
   Reason: I need to see what files are available
   Allow? (y/N/a for always allow this session):
```

## Roadmap

- [ ] **Interactive TUI** with Bubble Tea for richer experience
- [ ] **llama.cpp / MLX integration** for actual model inference
- [ ] **Streaming responses** via SSE
- [ ] **Model fine-tuning**
- [ ] **Persistent agent memory**
- [ ] **Sandboxed execution environments**
- [ ] **Web UI** companion
- [ ] **Plugin system** for custom tools

## License

MIT