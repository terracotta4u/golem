# Golem

An extensible personal AI agent that runs on your machine.

## Install

macOS or Linux:

```bash
curl -fsSL https://terracotta4u.com/golem/install.sh | sh 
```

## Get Started 

You can run Golem from the terminal:

```bash
golem serve
```

You should now be able to access the Golem web interface.

### LLM Setup

Out of the box, Golem has no built-in model. Install a provider extension, then set that provider's key:

```bash
golem extension add https://github.com/terracotta4u/golem-openrouter
export OPENROUTER_API_KEY=...
```

### Writing extensions

Install the Python SDK from PyPI and import it as `golem`:

```bash
pip install golem-agent-sdk
```

Source is in [`sdk/`](sdk/). The wire protocol is in [`docs/extensions.md`](docs/extensions.md).

