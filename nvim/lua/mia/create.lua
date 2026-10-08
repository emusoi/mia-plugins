local api = require("mia.api")
local M = {}

function M.worktree(branch, on_created)
  if not branch or vim.trim(branch) == "" then return end
  local out, err = api.exec({ "new", vim.trim(branch), "--json" })
  if err then
    vim.notify("mia: " .. err, vim.log.levels.ERROR)
    return
  end
  local ok, record = pcall(vim.json.decode, out)
  if not ok or type(record) ~= "table" or type(record.path) ~= "string" or type(record.name) ~= "string" then
    vim.notify("mia: new did not return a worktree record", vim.log.levels.ERROR)
    return
  end
  if on_created then on_created() end
  return require("mia.switch").to(record.path, record.name)
end

function M.prompt(on_created)
  vim.ui.input({ prompt = "New worktree branch: " }, function(branch)
    M.worktree(branch, on_created)
  end)
end

return M
