---
title: "Skins: any developer tells their agent how the dashboard should look"
labels: [ux, dashboard, phase-2]
---
Builds on task 10. Design and agent instructions: [docs/skins.md](../../docs/skins.md).

**Idea.** The dashboard's look is a *skin*: a folder of HTML and CSS with data bindings, under `.isekai/ui/skins/<name>/`. The isekai program never designs (no LLM). The **Change look** button turns the developer's description into a prompt for their own agent, and the agent builds, checks and applies the skin by following docs/skins.md.

**Work**
- [ ] `isekai stats --format json`: the data contract, schema version 1.
- [ ] Runtime: `data-bind`, `data-each`, `data-if`, `data-bind-var`, `data-bind-attr`. Text only, never HTML. Built-in `<isekai-radar>`, `<isekai-hero>`, `<isekai-settings>`, `<isekai-look-button>`.
- [ ] Built-in skins `isekai` (game) and `corporate`, rebuilt on the runtime.
- [ ] `isekai ui new | check | use | list`. The check covers: no network, no scripts, required slots, contrast, 390 px layout, 2 MB limit.
- [ ] Dashboard: the **Change look** dialog lists installed skins and has a "describe it" box. It generates the agent prompt, with a copy button.
- [ ] A skill for agents (Claude Code plugin from task 6, plus an AGENTS.md pointer) that loads docs/skins.md when asked to change the look.
- [ ] Later: `isekai ui add <git-url>` and a gallery of community skins.

**Rule.** Original work only: genre conventions are fine, but no copied game or brand assets (see docs/skins.md).

**Done when** a developer types "make it look like a dark MOBA HUD" into Change look, pastes the prompt into Claude Code, and gets a working, validated skin applied to the dashboard, with no change to the isekai binary.
