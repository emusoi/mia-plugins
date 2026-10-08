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

## mia-dev

Lend the main checkout's already-running dev server a worktree's files,
uncommitted work included, without a container. `mia dev <worktree>` puts
main's own changes aside in a stash and copies the worktree's tracked and
untracked files over main; ignored files (`node_modules`, `.env`) stay as
they are. `--follow` keeps copying every second. `mia dev off` gives main its
own files back. `v` on a dashboard row does the same.

    make build
    ln -s "$PWD/bin/mia-dev" ~/.config/mia/plugins/mia-dev && mia plugin enable dev

Don't commit in main while it is lent: `mia dev off` resets it.

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
