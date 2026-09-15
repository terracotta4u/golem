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
golem
```

You should now be able to access the Golem web interface.

### LLM Setup

Golem has no built-in model. Install a provider extension, then set that provider's key:

```sh
golem extension add https://github.com/terracotta4u/golem-openrouter
export OPENROUTER_API_KEY=...
```

Default conf still names `openrouter` and OpenRouter model ids. Other providers are separate extensions. 
