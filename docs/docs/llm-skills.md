---
id: llm-skills
title: LLM Skills Guide
sidebar_position: 8
---

# LLM Skills Guide

Sensor Hub includes skill files that teach AI assistants (like GitHub Copilot and Claude) how to interact with your Sensor Hub instance via the CLI tool. This enables natural-language control of your home monitoring system.

## Overview

A **skill** is a markdown document that instructs an LLM about available commands, expected output formats, and usage patterns. When installed, your AI assistant can:

- Query sensor readings and health status
- Add, enable, or disable sensors
- Manage alert rules and notifications
- Create, run and troubleshoot automations, such as a lights timer or a heating pair
- Create and manage API keys
- Configure application properties

## Prerequisites

1. **Sensor Hub CLI** installed and on your PATH (see [CLI Tool](./cli-tool))
2. **An API key** created from the web UI or CLI
3. **CLI configured** with `sensor-hub config init`

## Installing Skills

### Install for All Supported Assistants

```bash
sensor-hub skills install --all
```

This installs skill files for all supported LLM tools in their standard locations.

### Install for a Specific Assistant

```bash
# GitHub Copilot
sensor-hub skills install --target copilot

# Claude (Anthropic)
sensor-hub skills install --target claude
```

### View a Skill Before Installing

```bash
sensor-hub skills show --target copilot
```

### Installation Locations

| Assistant      | Location                                |
|----------------|-----------------------------------------|
| GitHub Copilot | `~/.copilot/skills/sensor-hub/SKILL.md` |
| Claude         | `~/.claude/skills/sensor-hub/SKILL.md`  |

## How It Works

Once installed, the skill file is automatically picked up by the assistant. You can then interact with your Sensor Hub using natural language.

### Example Conversations

**Checking sensor status:**

> "What's the health status of my sensors?"

The assistant will run `sensor-hub sensors health` and interpret the results for you.

**Getting temperature readings:**

> "Show me the temperature readings from the living room for the past week"

The assistant will construct the appropriate `sensor-hub readings between` command with the right date range and sensor filter.

**Managing alerts:**

> "Create an alert if the bedroom temperature goes above 28°C or below 15°C"

The assistant will identify the sensor, create the alert rule with the correct thresholds, and confirm the setup.

**Setting up automations:**

> "Turn the hallway lamp on at 7pm on weekdays and off again four hours later"

The assistant will find the lamp's ID and its capabilities, write the automation as JSON and create it with `sensor-hub automations create --file -`. For a heating pair, such as "heat the lounge below 16°C and stop at 20°C", it creates two reading-triggered automations and takes the re-arm margin from `sensor-hub automations margin-suggestion`.

**Explaining what happened:**

> "Why did the heating come on last night?"

The assistant will check `sensor-hub automations list` for status, then `sensor-hub automations runs <id>` for each run's trigger, steps and any error.

**Investigating issues:**

> "Are there any notifications I haven't seen yet?"

The assistant will check `sensor-hub notifications unread-count` and, if there are unread notifications, list them with `sensor-hub notifications list`.

### Tips for Effective Use

1. **Be specific about sensor names** — the assistant will match your description to actual sensor names
2. **Use natural date ranges** — "last week", "today", "past 24 hours" all work
3. **Ask follow-up questions** — the assistant maintains context across the conversation
4. **Command discovery** — the assistant can run `sensor-hub --help` and subcommand help to discover available options

## Supported Assistants

| Assistant          | Status      | Skill Target |
|--------------------|-------------|--------------|
| GitHub Copilot     | ✅ Supported | `copilot`    |
| Claude (Anthropic) | ✅ Supported | `claude`     |

## Security Considerations

- The skill file itself contains no secrets — only command patterns and documentation
- Authentication is handled by the API key in your `~/.sensor-hub.yaml` config file
- API keys inherit the permissions of the user who created them
- You can revoke an API key at any time from the web UI or CLI

## Troubleshooting

### Assistant doesn't recognise the skill

Ensure the skill file is installed in the correct location:

```bash
# Check Copilot skill
cat ~/.copilot/skills/sensor-hub/SKILL.md

# Check Claude skill
cat ~/.claude/skills/sensor-hub/SKILL.md
```

If the file is missing, re-run `sensor-hub skills install --target <target>`.

### Commands fail with authentication errors

Verify your CLI is configured correctly:

```bash
sensor-hub health
```

If this fails, run `sensor-hub config init` to reconfigure.

### Assistant runs wrong commands

The assistant uses `--help` flags to discover command syntax. If commands have changed after an upgrade, reinstall the skill:

```bash
sensor-hub skills install --all
```
