local api = require("mia.api")

local M = {}

function M.open(worktree, name)
  if not name or name == "" then
    local listing = { "lesson", "list", "--json" }
    if worktree and worktree ~= "" then table.insert(listing, worktree) end
    local out = api.exec(listing)
    local ok, names = pcall(vim.json.decode, out or "null")
    names = (ok and type(names) == "table") and names or {}
    table.insert(names, "+ new lesson…")
    vim.ui.select(names, { prompt = "lesson" }, function(choice)
      if not choice then return end
      if choice == "+ new lesson…" then
        vim.ui.input({ prompt = "lesson name: " }, function(typed)
          if typed and typed ~= "" then M.open(worktree, typed) end
        end)
        return
      end
      M.open(worktree, choice)
    end)
    return
  end
  local argv = { "lesson", "path" }
  if worktree and worktree ~= "" then table.insert(argv, worktree) end
  if name and name ~= "" then table.insert(argv, name) end
  local out, err = api.exec(argv)
  if err then
    vim.notify("mia: " .. err, vim.log.levels.ERROR)
    return
  end
  local path = out:gsub("%s+$", "")
  vim.cmd.edit(vim.fn.fnameescape(path))
  return path
end

return M
