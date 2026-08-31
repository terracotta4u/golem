# Golem

An extensible personal AI agent that runs on your machine.

## Install

macOS or Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/terracotta4u/golem/main/install.sh | sh 
```

## Get Started 

You can run Golem from the terminal:

```bash
golem
```

### LLM Setup

By default Golem uses [OpenRouter](https://openrouter.ai/). You will need to set an `OPENROUTER_API_KEY` environment variable. 

### Telegram Setup

The quickest way to start talking to Golem is with Telegram:

```bash
golem extension add https://github.com/terracotta4u/golem-telegram
```

Telegram also requires you create a new bot, instructions can be found [here](https://core.telegram.org/bots/tutorial).

Once you have create a new bot set the `TELEGRAM_BOT_TOKEN` environment variable.