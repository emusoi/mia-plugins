# mia-plugins

Private. The plugins that sit on top of [mia-core](https://github.com/emusoi/mia-core).
None of them is needed to use mia; each does nothing until `mia plugin enable <name>`.
Nothing here is installed by default.

## mia-plan

Plans whose steps are proven by checks, and lessons, per branch. What used to be
`mia plan`, `mia lesson`, `mia stack from-plan` (now `mia plan stack`) and the
dashboard's *needs proof* / *proven* sections.

    make build                                  # bin/mia-plan
    ln -s "$PWD/bin/mia-plan" ~/.config/mia/plugins/mia-plan
    mia plugin enable plan

Settings, in `mia config` (or `~/.config/mia/config.toml`):

    [plugin.plan.checks]               # what a step cites as @name
    test = ["go", "test", "./..."]

    [plugin.plan]
    stores = { plans = "wazo" }        # optional: keep plans/lessons/evidence in a mia-store-<name> helper
    editor = "nvim"                    # optional: else $VISUAL, $EDITOR

Records stay where mia-core kept them, under `.git/mia/plans`, `evidence` and
`lessons`, so existing plans read as they are. Checks run through
`mia env run`, inside the environment when one is up.

## mia-agent

Coding agents in your worktrees. `mia agent run [worktree] [--task <text>]
[agent]` starts one in a new window of the worktree's session; `mia agent ls`
shows every agent and whether it is working, waiting on you, finished or
idle, read from its screen. `attach`, `send -- <text>` and `stop` take a
worktree and, when it has more than one agent, a window. Needs mia-core with
`mia window read` and `send`.

    make build
    ln -s "$PWD/bin/mia-agent" ~/.config/mia/plugins/mia-agent && mia plugin enable agent

On the dashboard, a worktree whose agent is waiting moves to *waiting on
you* at the top, then *finished*, then *agents working*. `A` starts an agent
on the selected row, `w` lands in its window. While the dashboard is open the
plugin runs as `mia-agent serve` and redraws it the moment a hook reports.

Reading the screen can miss a short burst of work. `mia agent hooks install`
has Claude Code report its own state instead (prompt, tool use, a permission
question, done) through hooks in `~/.claude/settings.json`, and prints the
`notify` line that does the same for codex. `uninstall` removes only mia's
hooks.

Settings:

    [plugin.agent]
    default = "claude"                     # what `mia agent run` starts
    agents = ["claude", "codex", "cursor"]
    bin = { cursor = "cursor-agent" }      # when the command is not the name

## mia-gh (example, shell)

Each worktree's pull request on the dashboard: a status, a `pr` tab, an
*in review* section for open ones, and `O` (`mia gh-open`) to open it in the
browser. Needs `gh` and `jq`; one `gh pr list` per minute for the whole repo.

    ln -s "$PWD/gh/mia-gh" ~/.config/mia/plugins/mia-gh && mia plugin enable gh

## mia-notify (example, shell)

A desktop notification for what happens in your worktrees: osascript on a
Mac, notify-send elsewhere. Needs `jq`.

    ln -s "$PWD/notify/mia-notify" ~/.config/mia/plugins/mia-notify && mia plugin enable notify

    [plugin.notify]
    events = ["env.up", "worktree.created"]   # optional: only these
    command = "ping-me"                       # optional: given the title and the message

Writing your own: mia-core's `docs/plugins.md` is the protocol.

## nvim

The Neovim plugin, moved out of mia-core. `nvim/install.sh` writes a lazy.nvim
spec pointing at this checkout. `:Mia` opens the worktrees; with mia-plan
enabled, `:Mia plan`, `:Mia check` and `:Mia lesson` do what they say.

## Testing

    make test                                   # go test, then scripts/loop.sh against the mia on PATH
    ./nvim/test/run.sh                          # the Neovim suite, against the mia on PATH
