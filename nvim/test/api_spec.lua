local api = require("mia.api")
local command
vim.system = function(argv)
  command = argv
  return { wait = function() return { code = 0, stdout = '{"version":2}' } end }
end

api.query("env", "/tmp/a worktree")
assert(vim.deep_equal(command, { "mia", "api", "env", "/tmp/a worktree" }),
  "query and target must be separate arguments, including paths with spaces")
assert(api.panel("dashboard"))
assert(vim.deep_equal(command, { "mia", "api", "dashboard" }), "a panel is requested by name")
