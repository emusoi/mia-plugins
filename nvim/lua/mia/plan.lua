
local api = require("mia.api")

local M = {}

local GROUP = "mia-plan"

M.signs = {
  proven   = { text = "✔ ", hl = "DiagnosticOk" },
  attested = { text = "· ", hl = "DiagnosticHint" },
  stale    = { text = "~ ", hl = "DiagnosticWarn" },
  claimed  = { text = "? ", hl = "DiagnosticWarn" },
  open     = { text = "☐ ", hl = "DiagnosticInfo" },
}

local function define_signs()
  for state, sign in pairs(M.signs) do
    vim.fn.sign_define(GROUP .. "-" .. state, { text = sign.text, texthl = sign.hl })
  end
end

function M.place(buf, report)
  define_signs()
  vim.fn.sign_unplace(GROUP, { buffer = buf })
  for _, resolved in ipairs((report or {}).steps or {}) do
    local line = (resolved.step or {}).line
    if line and M.signs[resolved.state] then
      vim.fn.sign_place(0, GROUP, GROUP .. "-" .. resolved.state, buf, { lnum = line, priority = 10 })
    end
  end
end

function M.summary(report)
  local counts = {}
  for _, key in ipairs({ "proven", "attested", "stale", "claimed", "open" }) do
    counts[key] = report[key] or 0
  end
  local parts = { ("%d/%d proven"):format(counts.proven, report.total or 0) }
  for _, key in ipairs({ "attested", "stale", "claimed", "open" }) do
    if counts[key] > 0 then table.insert(parts, ("%d %s"):format(counts[key], key)) end
  end
  return table.concat(parts, " · ")
end

function M.step_at(report, line)
  for _, resolved in ipairs((report or {}).steps or {}) do
    if (resolved.step or {}).line == line then return resolved end
  end
  return nil
end

function M.query(worktree)
  local argv = { "plan" }
  if worktree and worktree ~= "" then table.insert(argv, worktree) end
  vim.list_extend(argv, { "show", "--json" })
  local out, err = api.exec(argv)
  if err then return nil, err end
  local ok, decoded = pcall(vim.json.decode, out)
  if not ok then return nil, "mia plan --json did not return JSON" end
  return decoded, nil
end

function M.open(worktree)
  local argv = { "plan", "path" }
  if worktree then table.insert(argv, worktree) end
  local _, err = api.exec(argv)
  if err then
    vim.notify("mia: " .. err, vim.log.levels.ERROR)
    return
  end
  local data
  data, err = M.query(worktree)
  if err then
    vim.notify("mia: " .. err, vim.log.levels.ERROR)
    return
  end

  vim.cmd.edit(vim.fn.fnameescape(data.path))
  local buf = vim.api.nvim_get_current_buf()
  M.place(buf, data.report)

  vim.b[buf].mia_plan = data
  vim.notify(("%s · %s"):format(data.branch, M.summary(data.report)))
  return { buf = buf, data = data }
end

function M.check()
  local buf = vim.api.nvim_get_current_buf()
  local data = vim.b[buf].mia_plan
  if not data then
    vim.notify("mia: this buffer is not a plan — `:Mia plan` opens one", vim.log.levels.ERROR)
    return
  end

  if vim.bo[buf].modified then
    vim.notify("mia: save the plan before running a check", vim.log.levels.WARN)
    return
  end

  local fresh, query_err = M.query(data.worktree)
  if not fresh or fresh.path ~= data.path then
    vim.notify("mia: " .. (query_err or "the branch changed — reopen its plan"), vim.log.levels.ERROR)
    return
  end
  data = fresh

  local line = vim.api.nvim_win_get_cursor(0)[1]
  local resolved = M.step_at(data.report, line)
  if not resolved then
    vim.notify("mia: no step on this line", vim.log.levels.WARN)
    return
  end
  if (resolved.step or {}).check == "" or not (resolved.step or {}).check then
    vim.notify(("mia: %q names no check — nothing to run"):format(resolved.step.text), vim.log.levels.WARN)
    return
  end

  local out, err = api.exec({ "plan", "check", data.worktree, resolved.step.text })
  if err then
    vim.notify("mia: " .. err, vim.log.levels.ERROR)
  elseif out and out ~= "" then
    vim.notify((out:gsub("%s+$", "")))
  end

  fresh = M.query(data.worktree)
  if fresh then
    vim.b[buf].mia_plan = fresh
    M.place(buf, fresh.report)
  end
end

return M
