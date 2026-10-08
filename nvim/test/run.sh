#!/usr/bin/env bash
# Run the Neovim tests.
#
# Never invoke nvim by hand for these. Two traps make hand-running worse than
# useless, and both have cost real days:
#
#   * A relative `rtp+=nvim` breaks the moment a test changes directory, so the
#     plugin silently is not loaded and every test "passes".
#   * `-c luafile … -c qa!` exits 0 EVEN WHEN THE TEST THROWS, so a failure
#     looks like a success.
#
# So: an absolute runtimepath, pcall around the file, and :cq to carry a real
# exit code out.
set -uo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
plugin="$(cd "$here/.." && pwd)"
filter="${1:-}"

failed=0
for file in "$here"/*_spec.lua; do
  name="$(basename "$file" _spec.lua)"
  if [[ -n "$filter" && "$name" != *"$filter"* ]]; then continue; fi

  printf '%-24s' "$name"
  output="$(nvim --headless --clean \
    --cmd "set runtimepath^=$plugin" \
    --cmd "lua local ok, err = pcall(dofile, '$file'); if not ok then io.stderr:write(tostring(err) .. '\n'); vim.cmd('cq') end" \
    --cmd "qa!" 2>&1)"
  code=$?
  if [[ $code -ne 0 ]]; then
    echo "FAILED"
    echo "$output" | sed 's/^/    /'
    failed=1
  else
    echo "ok"
  fi
done

if [[ $failed -ne 0 ]]; then
  echo
  echo "FAILED"
  exit 1
fi
echo "all passed"
