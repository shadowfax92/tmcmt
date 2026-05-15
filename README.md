<div align="center">

# 📎 tmcmt

**Comment on tmux pane output and feed it back to the agent.**

*Select, annotate, stack. Flush when you're done — scroll position never moves.*

</div>

You're watching a `claude` or `codex` agent produce a long reply in a tmux pane. You want to scroll up, pick out specific passages, push back on each one, and send the whole annotated reply as your next prompt — without losing your place in the scrollback every time you touch the keyboard. `tmcmt` gives you exactly that loop: commented chunks accumulate in a per-pane draft file on disk, and nothing touches the agent's prompt until you explicitly flush.

- 📎 **Chunks stack into a draft** — select a passage, type a comment, repeat. The draft grows on disk, not in the pane.
- 🧭 **Scroll position is preserved** — the pane's PTY is never written to during the add loop, so copy mode stays put.
- ✍️ **nvim popup for every edit** — your keybindings, your config, your muscle memory. No custom TUI to learn.
- 🔁 **Review pass before flush** — open the whole accumulated draft in nvim, reorder, polish, or delete chunks before sending.
- 📬 **Bracketed-paste injection** — the whole draft arrives as one coherent message, not N fragments to stitch together.
- 💾 **Recoverable drafts** — active drafts survive restarts, and flushed drafts are archived as incremental `.md` files.
- 🧹 **Stale-draft GC** — active drafts for dead panes are cleaned up automatically on every add/flush.
- 📡 **Multicast review** — `S` lets you pick several coding panes, remembers them, and sends one reviewed draft to all of them.
- ⚡ **Direct selection send** — `s` sends the selected text raw to your remembered coding panes, with no draft or editor.
- 🔧 **Scriptable primitive** — `tmcmt send` pipes stdin straight into one or more panes for automation.

---

## How it works

```
┌─ agent pane (copy mode, scrolled up) ───┐
│ the agent's reply…                       │
│ cursor at line 147 of scrollback         │   never moves during the loop
└──────────────────────────────────────────┘
           ▲                    ▲
           │ c                  │ c
           │                    │
           ▼                    ▼
   ┌─ popup: nvim ─┐     ┌─ popup: nvim ─┐
   │ type comment  │     │ type comment  │
   │ :wq           │     │ :wq           │
   └───────────────┘     └───────────────┘
           │                    │
           ▼                    ▼
   append → ~/.local/state/tmcmt/drafts/<pane>.md ← append
                         │
                      (later)
                         │  C = flush
                         ▼
          ┌─ popup: nvim ─┐
          │ review draft  │
          │ reorder/edit  │
          │ :wq           │
          └───────────────┘
                         │
                         ▼
          paste whole draft into pane (bracketed paste, no Enter)
                         │
                         ▼
          archive draft in ~/.local/state/tmcmt/drafts/done/
```

- **`c`** in copy mode → nvim popup, type comment, `:wq`. Chunk appended to draft. **Pane untouched.**
- **`s`** in copy mode → sends the selected text raw to remembered coding panes. If none are remembered/live, opens the target selector first.
- **`C`** in copy mode → nvim popup opens on the accumulated draft for a final review. `:wq` pastes into the current pane.
- **`S`** in copy mode → tmux popup opens a multi-select list of detected coding panes, then nvim opens the same draft for review. `:wq` pastes into every selected pane and remembers those targets for next time.
- **`I`** in copy mode → opens the target selector and updates the remembered coding panes without sending anything.

You hit Enter yourself when the prompt looks right.

## Install

Requires Go 1.24+, tmux 3.2+, and `nvim` (or any `$EDITOR`, but `nvim`/`vim` get insert-mode-on-open). Interactive target selection for `s`, `S`, and `I` requires `fzf`; non-interactive multicast works with `--targets` or `--reuse`.

```sh
git clone <repo-url> tmcmt
cd tmcmt
make install    # builds and copies to ~/bin/tmcmt
```

Make sure `~/bin` is on your `PATH`.

## tmux bindings

Add to `~/.tmux.conf` and reload (`tmux source-file ~/.tmux.conf`):

```tmux
# c = append a commented selection to this pane's draft (no paste)
bind-key -T copy-mode-vi c send-keys -X copy-pipe-no-clear \
  "tmcmt draft add --pane '#{pane_id}'"

# s = send selected text raw to remembered/new coding panes
bind-key -T copy-mode-vi s send-keys -X copy-pipe-no-clear \
  "tmcmt selection send --pane '#{pane_id}'"

# C = flush the draft: review in nvim popup, paste into pane, archive
bind-key -T copy-mode-vi C send-keys -X copy-pipe-no-clear \
  "tmcmt draft flush --pane '#{pane_id}'"

# S = multicast the draft: select remembered/new coding panes, review, paste, archive
bind-key -T copy-mode-vi S send-keys -X copy-pipe-no-clear \
  "tmcmt draft multicast --pane '#{pane_id}'"

# I = update remembered coding panes without sending
bind-key -T copy-mode-vi I run-shell \
  "tmcmt selection targets --pane '#{pane_id}'"
```

The selection-consuming bindings use `copy-pipe-no-clear` so copy mode stays active and scroll position is preserved.

## Quick Start

Inside a tmux pane running `claude` or `codex`:

1. **`M-w`** (or whatever opens your copy mode) and scroll up to something the agent said.
2. **`v`** select a passage.
3. **`c`** — nvim popup opens with an empty comment area on top and your selection rendered as a commented-out preview below a `TMCMT-SELECTION-BELOW` separator.
4. Type your comment, **`:wq`**. Status bar flashes `tmcmt: 1 chunk in draft — C to flush`.
5. Scroll somewhere else, select, **`c`** again. Counter ticks up.
6. For a quick one-off context send, select text and press **`s`**. If target panes are not remembered yet, pick them in the `fzf` popup first.
7. Press **`I`** any time in copy mode to update the remembered coding panes without sending text.
8. When the whole reply is assembled, press **`C`** for the current pane or **`S`** to multicast.
9. With **`S`**, pick one or more detected coding panes in the `fzf` popup. Previously selected targets for this source pane are listed first, marked, and preselected.
10. nvim opens on the full draft. Reorder, edit, or delete chunks freely.
11. **`:wq`** — draft pastes into the selected prompt or prompts via bracketed paste, then moves to `drafts/done/`. No auto-Enter.
12. Review in the live prompt and hit **Enter** yourself when it looks right.

If you exit nvim without changes (`:q!` or `:wq` on an unmodified file), the chunk is treated as cancelled — nothing is appended.

## Commands

```sh
tmcmt draft add   --pane <id>    # append a commented chunk (bound to `c`)
tmcmt draft flush --pane <id>    # review + paste + archive draft (bound to `C`)
tmcmt draft multicast --pane <id> # select target panes, review, paste, archive
tmcmt draft show  --pane <id>    # print current draft to stdout
tmcmt draft clear --pane <id>    # discard current draft
tmcmt draft list                 # list all drafts with pane status
tmcmt selection send --pane <id>    # send raw stdin to remembered/new coding panes
tmcmt selection targets --pane <id> # update remembered coding panes
tmcmt cat        [-n N]           # print recent flushed sessions
tmcmt ls         [-n N]           # alias for cat
tmcmt send        --pane <id>    # paste stdin → pane(s), repeatable
tmcmt --version
```

`--pane` defaults to the current pane, so from a shell inside the agent pane you can just run `tmcmt draft show` without arguments.

### Flush flags

| Flag | Description |
|------|-------------|
| `--no-review` | Skip the nvim review pass, paste the draft as-is |
| `--send` | Press Enter after pasting (auto-submit) |
| `--dry-run` | Print the final payload to stdout instead of pasting |

### Send flags

| Flag | Description |
|------|-------------|
| `--pane <id>` | Target pane id (default: current pane); repeat or comma-separate for multicast |
| `--enter` | Press Enter after pasting |

### Multicast flags

| Flag | Description |
|------|-------------|
| `--targets <ids>` | Bypass the selector and send to comma-separated pane ids |
| `--reuse` | Reuse remembered live targets for this source pane |
| `--all-panes` | Show all panes in the selector, not only detected coding panes |
| `--send` | Press Enter in every target pane after pasting |
| `--dry-run` | Print target summary and payload without opening the editor, pasting, archiving, or updating remembered targets |

### Selection commands

`tmcmt selection send --pane <source>` reads stdin and sends it raw to remembered live targets for that source pane. If no remembered target is live, it opens the same coding-pane selector used by multicast, stores the chosen panes, then sends.

`tmcmt selection targets --pane <source>` only opens the selector and stores the chosen panes. It is meant for the `I` binding.

### Done sessions

Successful flushes are archived under `~/.local/state/tmcmt/drafts/done/`.
Use `tmcmt cat` to print the latest archived session, or `tmcmt cat -n 5`
to print the five most recent sessions. Each printed session starts with the
full archive path:

```text
==> /Users/you/.local/state/tmcmt/drafts/done/42-000003.md <==
```

`tmcmt ls` is an alias for the same output.

## Draft format

Draft files live at `~/.local/state/tmcmt/drafts/<pane-id>.md` (pane id with the `%` stripped — e.g. `%42` → `42.md`). Each chunk is appended as:

    <your comment>

    ```
    <selected terminal text>
    ```

    (blank line)

Chunks just accumulate — no separators, no metadata. When flushed, the raw file content is the payload and then moves to `~/.local/state/tmcmt/drafts/done/<pane-id>-000001.md`, incrementing per pane.

## Why draft-first?

The obvious design is "select → comment → paste immediately, re-enter copy mode" — and it's wrong. The paste bumps the pane back to the prompt, claude-code redraws, and even if you re-enter copy mode you land at the bottom of the scrollback, not where you were reading. You lose your place every cycle.

Draft accumulation fixes this structurally:

- The pane's PTY is **never written to** during the `c` loop. Copy mode stays active, scroll position is preserved, selections clear automatically so you can make the next one without pressing Escape.
- Bonus wins: you see the whole reply together before sending, you can reorder/polish/delete, and the draft survives a crash or accidental popup close.
- Cost paid: one extra keystroke (`C` to flush) and one extra nvim popup for the review. Both feel cheap.

## How it handles…

- **Cancellation** — `:q!` or `:wq` without edits → chunk cancelled, nothing appended.
- **Empty comment** — allowed. You get a selection-only chunk.
- **Flushed drafts** — successful flushes are kept under `drafts/done/` as incremental `.md` files, capped at the newest 1000 archive files.
- **Remembered targets** — selected target panes for multicast and direct selection sends are stored per source pane under `~/.local/state/tmcmt/targets.json`. Stale pane ids are ignored on reuse.
- **Stale drafts** — every `add`/`flush` walks the active drafts dir and deletes files whose pane id no longer exists in `tmux list-panes -a`.
- **ANSI in selections** — CSI (color) and OSC (hyperlinks, titles) sequences are stripped before the selection hits your draft.
- **Multi-line content** — pasted with bracketed paste (`-p`), so claude-code treats it as one message instead of submitting each line.

## Scripting

`tmcmt send` is a straight-up "pipe text into a pane's prompt" primitive:

```sh
# dump a log into a running claude pane
tail -100 build.log | tmcmt send --pane %42

# with auto-enter
echo "rerun the last test" | tmcmt send --pane %42 --enter

# multicast stdin directly
echo "rerun the last test" | tmcmt send --pane %42 --pane %47 --enter

# multicast the current source draft without opening the selector
tmcmt draft multicast --pane %12 --targets %42,%47

# reuse the last live targets selected for this source pane
tmcmt draft multicast --pane %12 --reuse

# send raw stdin through the remembered/new target selector
echo "look at this stack trace" | tmcmt selection send --pane %12

# update remembered direct-send/multicast targets without sending
tmcmt selection targets --pane %12
```

## Layout

```
tmcmt/
├── main.go
├── cmd/            # cobra subcommands (draft_add, draft_flush, send, ...)
├── internal/
│   ├── tmux/       # tmux CLI wrappers (run, paste, popup, display-message)
│   ├── proc/       # process snapshot + descendant walking for coding-pane discovery
│   ├── targets/    # remembered multicast target panes
│   ├── draft/      # per-pane draft file CRUD + GC
│   ├── chunk/      # nvim compose template build/parse
│   └── sanitize/   # ANSI CSI + OSC strip
└── Makefile
```

---

> Personal tool built for my own workflow — I live inside tmux panes running claude/codex all day and got tired of losing scroll position. Feel free to fork and adapt.
