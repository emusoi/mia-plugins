
local mia = require("mia")
local cli_verbs = {}
for line in (vim.system({ "mia", "help" }, { text = true }):wait().stdout or ""):gmatch("[^\n]+") do
  local verb = line:match("^%s+mia (%S+)")
  if verb then table.insert(cli_verbs, verb) end
end
assert(#cli_verbs > 10, "mia help listed no verbs")

local function has(list, want)
  for _, item in ipairs(list) do if item == want then return true end end
  return false
end

local editor_only = { dashboard = true, worktrees = true,
  env = true, term = true, switch = true, back = true }

for name in mia.views():gmatch("[^,%s]+") do
  if not editor_only[name] and not has(cli_verbs, name) then
    error(("`:Mia %s` exists in the editor but there is no such mia command"):format(name))
  end
end

if not mia.views():find("dashboard", 1, true) then
  error("the dashboard is not among the views")
end

mia.setup()
local commands = vim.api.nvim_get_commands({})
if not commands.Mia then error(":Mia was not registered") end
if commands.Mia.nargs ~= "*" then
  error("`:Mia` takes no arguments, so `:Mia env feature-a` cannot work")
end
