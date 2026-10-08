
local M = { views_from = {} }
local panels = {
  worktrees = "dashboard",
  dashboard = "dashboard",
  env       = "env",
  stack     = "stack",
  window    = "blocks",
  machine   = "machines",
}
local verbs = {
  new = true, switch = true, adopt = true, rm = true,
  up = true, down = true, stack = true,
}
do
  local result = vim.system({ "mia", "__complete", "verbs" }, { text = true }):wait()
  if result.code == 0 then
    for verb in (result.stdout or ""):gmatch("[^\n]+") do verbs[verb] = true end
    for _, taken in ipairs({ "dash", "shell", "api", "env", "shell-init" }) do verbs[taken] = nil end
  end
end

local function report(out, err)
  if err then
    vim.notify("mia: " .. err, vim.log.levels.ERROR)
    return false
  end
  if out and out:gsub("%s+", "") ~= "" then vim.notify(out:gsub("%s+$", "")) end
  return true
end
function M.open(argv)
  local words = {}
  for word in tostring(argv or ""):gmatch("%S+") do table.insert(words, word) end
  local what = words[1] or "dashboard"
  local rest = { unpack(words, 2) }

  if panels[what] then
    return require("mia.dash").open(panels[what], rest[1])
  end

  if M.views_from[what] then
    return M.views_from[what](rest)
  end
  if what == "term" or what == "terminal" then
    return require("mia.term").open(rest[1])
  end
  if what == "switch" then
    return require("mia.switch").worktree(rest[1])
  end
  if what == "back" then
    return require("mia.switch").back()
  end
  if what == "new" then
    if #rest == 0 then return require("mia.create").prompt() end
    if #rest == 1 and rest[1] ~= "--stack" then
      return require("mia.create").worktree(rest[1])
    end
  end
  if verbs[what] then
    return report(require("mia.api").exec({ what, unpack(rest) }))
  end

  vim.notify(("mia: no view called %q — try %s"):format(what, M.views()), vim.log.levels.ERROR)
end
function M.add_view(name, open)
  M.views_from[name] = open
end

function M.views()
  local names = {}
  for name in pairs(panels) do table.insert(names, name) end
  for _, name in ipairs({ "term", "back" }) do table.insert(names, name) end
      for name in pairs(M.views_from) do table.insert(names, name) end
  for name in pairs(verbs) do table.insert(names, name) end
  table.sort(names)
  return table.concat(names, ", ")
end

function M.setup(opts)
  opts = opts or {}
  M.add_view("plan", function(rest) return require("mia.plan").open(rest[1]) end)
  M.add_view("check", function() return require("mia.plan").check() end)
  M.add_view("lesson", function(rest)
    local worktree, name = nil, table.concat(rest, " ")
    if rest[1] and require("mia.api").exec({ "path", rest[1] }) then
      worktree, name = rest[1], table.concat(rest, " ", 2)
    end
    return require("mia.lesson").open(worktree, name)
  end)
  require("mia.switch").hooks = { on_leave = opts.on_leave, on_arrive = opts.on_arrive }
  vim.api.nvim_create_user_command("Mia", function(args)
    M.open(args.args)
  end, {
    nargs = "*",
    complete = function()
      local names = {}
      for name in pairs(panels) do table.insert(names, name) end
      for _, name in ipairs({ "term", "back" }) do table.insert(names, name) end
      for name in pairs(M.views_from) do table.insert(names, name) end
      for name in pairs(verbs) do table.insert(names, name) end
      table.sort(names)
      return names
    end,
    desc = "mia worktrees, stacks and environments",
  })
  vim.cmd([[cnoreabbrev <expr> mia getcmdtype() == ':' && getcmdpos() == 4 ? 'Mia' : 'mia']])
  return M
end

return M
