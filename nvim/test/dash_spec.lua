local launched, switched
vim.fn.jobstart = function(argv, opts)
  launched = { argv = argv, opts = opts }
  return 1
end
package.loaded["mia.switch"] = { to = function(path) switched = path end }

require("mia.dash").open("dashboard")
assert(launched, "no terminal job was started")
assert(launched.argv[1] == "mia" and launched.argv[2] == "dash" and launched.argv[3] == "dashboard", vim.inspect(launched.argv))
assert(launched.argv[4] == "--pick-to" and launched.argv[5] ~= "", "no pick file")
assert(launched.opts.term == true, "not a terminal job")

vim.fn.writefile({ "/tmp/somewhere.longido" }, launched.argv[5])
launched.opts.on_exit(1, 0)
vim.wait(200, function() return switched ~= nil end)
assert(switched == "/tmp/somewhere.longido", "the editor did not switch to the picked worktree: " .. tostring(switched))

switched = nil
require("mia.dash").open("env", "longido")
assert(launched.argv[3] == "env" and launched.argv[4] == "longido", vim.inspect(launched.argv))
launched.opts.on_exit(1, 0)
vim.wait(200)
assert(switched == nil, "leaving a panel without picking switched the editor")

local plan = vim.fn.tempname() .. ".md"
vim.fn.writefile({ "# plan" }, plan)
require("mia.dash").open("dashboard")
vim.fn.writefile({ "edit " .. plan }, launched.argv[5])
launched.opts.on_exit(1, 0)
local real = vim.uv.fs_realpath(plan)
vim.wait(200, function() return vim.uv.fs_realpath(vim.api.nvim_buf_get_name(0)) == real end)
assert(vim.uv.fs_realpath(vim.api.nvim_buf_get_name(0)) == real, "an edit pick did not open the file in this editor: " .. vim.api.nvim_buf_get_name(0))
assert(switched == nil, "an edit pick switched worktrees")
print("dash ok")
