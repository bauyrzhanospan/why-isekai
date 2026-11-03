---
title: "Optional: notifications and chat with agents (Telegram, Slack, Discord, email, Matrix, ntfy)"
labels: [integration, optional, later]
---
Two layers, built in this order.

**1. Notifications (one way).** Events from the journal go to where the human already is:
- `decision.asked` — an agent asks you something (the most important one);
- `next.stale` — open work older than N days; `check.failed` — an agent commit broke the rules in CI;
- a daily/weekly digest: what agents did, what is open, what waits for you (the dashboard numbers from task 10).
Channels: Telegram bot, Slack, Discord, Matrix (webhooks), email (SMTP), ntfy.sh. Configured in `[notify]` per event; tokens through the secrets module.

**2. Chat as an alternative interface (two way).** Reply to a `decision.asked` message in Telegram/Slack/Discord → the answer is recorded exactly like an issue comment (task 9) and reaches the next agent session's brief. Later: `/brief`, `/open`, `/stats` commands in the bot.

**Security — non-negotiable**
- Only allow-listed users (by chat/user id) can answer; everyone else is ignored.
- An answer is data for the next session, never a command executed by isekai; the bot cannot run code, push, or read secrets.
- Messages carry commit text and Next items only: no diffs, no file contents, no secret values (spec §B.2). Clearly opt-in, with a list of what leaves the repository.

**Where it runs**: CI (on push / on schedule) or `isekai notify` / `isekai bot` run explicitly by the user — never from git hooks, never by default; the core CLI stays offline.

**Done when** an agent's question reaches the maintainer in Telegram, the reply appears in the next `isekai brief`, and a non-allow-listed reply is ignored, with tests on fake bot APIs.
