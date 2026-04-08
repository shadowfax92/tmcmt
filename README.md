# tmcmt

Accumulate commented chunks of tmux pane output into a per-pane draft, then
paste the whole draft into the pane's prompt with one keystroke.

Built for long-form replies to running `claude` / `codex` agents from inside
their own tmux pane, without losing copy-mode scroll position.

## Why

You're looking at a long agent reply in a tmux pane. You scroll up through
it in copy mode, want to push back on specific passages. The nvim sidekick
plugin solves this for files; `tmcmt` does the same for terminal output in
tmux — and it accumulates multiple commented selections into one coherent
reply, never touching the pane's PTY until you explicitly flush.

## Install

    make install   # copies to ~/bin/tmcmt

## tmux bindings

Add to `~/.tmux.conf`:

    bind-key -T copy-mode-vi c send-keys -X copy-pipe-no-clear \
      "tmcmt draft add --pane '#{pane_id}'"
    bind-key -T copy-mode-vi C send-keys -X copy-pipe-no-clear \
      "tmcmt draft flush --pane '#{pane_id}'"

## Usage

In a pane running `claude` or `codex`:

1. **`M-w`** — enter copy mode, scroll up to where the agent said something
   you want to reply to.
2. **`v`**, select a passage, then **`c`** — nvim popup opens with an empty
   comment area above a read-only preview of your selection. Type your
   comment, `:wq`. Nothing is pasted yet — the chunk is appended to a draft
   file on disk.
3. Scroll somewhere else, select, **`c`** again. Repeat as many times as
   needed. Your scroll position is preserved the entire time because the
   pane's PTY never receives any input.
4. When done, press **`C`** — nvim popup opens with the whole accumulated
   draft for a final review. Reorder, delete, or polish freely. `:wq`
   pastes the draft into the agent's prompt via bracketed paste (no Enter
   — review it in the live prompt before sending).
5. Hit Enter in the pane when the prompt looks right.

## Subcommands

    tmcmt draft add   --pane <id>    # append chunk (bound to `c`)
    tmcmt draft flush --pane <id>    # review + paste draft (bound to `C`)
    tmcmt draft show  --pane <id>    # print current draft to stdout
    tmcmt draft clear --pane <id>    # discard current draft
    tmcmt draft list                 # list all drafts with pane status
    tmcmt send        --pane <id>    # paste stdin → pane (scriptable)

Flags on `draft flush`:

    --no-review    skip the nvim review pass, paste draft as-is
    --send         press Enter after pasting (auto-submit)
    --dry-run      print final payload to stdout instead of pasting

## Draft storage

    ~/.local/state/tmcmt/drafts/<pane-id>.md

Drafts are per-pane and garbage-collected on every `add` / `flush` — if the
pane they belong to no longer exists, the draft file is deleted.

## Cancelling a chunk mid-compose

Exit nvim with `:q!` (or `:wq` without editing anything). If the chunk file
is byte-identical to the template we wrote, the chunk is treated as
cancelled and nothing is appended to the draft.

## Design

See `~/llm/projects/tmux-agent-comment/DESIGN.md` for the full design doc
and `DISCUSSION.md` for the back-and-forth that got us here.
