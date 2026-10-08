#!/bin/sh
set -eu

here=$(cd "$(dirname "$0")" && pwd)
specs="${XDG_CONFIG_HOME:-$HOME/.config}/nvim/lua/plugins"

if [ ! -d "$specs" ]; then
	echo "no lazy.nvim spec directory at $specs" >&2
	echo "if you use a different plugin manager, point it at: $here" >&2
	exit 1
fi

cat > "$specs/mia.lua" <<LUA
local dir = "$here"

return {
  {
    dir = dir,
    name = "mia.nvim",
    lazy = false,
    enabled = vim.uv.fs_stat(dir) ~= nil,
    config = function()
      require("mia").setup()
    end,
  },
}
LUA

echo "wrote $specs/mia.lua"
echo "it needs \`mia\` on PATH: $(command -v mia || echo 'NOT FOUND — make install')"
