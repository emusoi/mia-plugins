local M = {}

function M.open(panel, target)
  local pick = vim.fn.tempname()
  local argv = { "dash", panel or "dashboard" }
  if target and target ~= "" then table.insert(argv, target) end
  vim.list_extend(argv, { "--pick-to", pick })
  return require("mia.term").run(argv, panel == "dashboard" and "worktrees" or panel, function(code)
    if code ~= 0 then return end
    local ok, lines = pcall(vim.fn.readfile, pick)
    vim.fn.delete(pick)
    if not ok or not lines or not lines[1] or lines[1] == "" then return end
    local path = lines[1]:match("^edit (.+)$")
    if path then
      vim.cmd.edit(vim.fn.fnameescape(path))
      return
    end
    require("mia.switch").to(lines[1])
  end, { frameless = true, width = math.min(vim.o.columns - 2, 106), height = math.min(vim.o.lines - 2, 36) })
end

return M
