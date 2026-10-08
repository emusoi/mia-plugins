local temp = vim.fn.tempname()
local a, b = temp .. "/a", temp .. "/b"
for _, dir in ipairs({ a, b }) do
  vim.fn.mkdir(dir, "p")
  vim.system({ "git", "-C", dir, "init", "-q" }):wait()
end
package.loaded["mia.api"] = { exec = function() return "", nil end, query = function() return {}, nil end }

local mia = require("mia")
local left, arrived
mia.setup({ on_leave = function(from) left = from end, on_arrive = function(to) arrived = to end })

local switch = require("mia.switch")
vim.cmd.tcd(vim.fn.fnameescape(a))
assert(switch.to(b, "b"), "switch failed")
assert(left == vim.uv.fs_realpath(a), "on_leave got " .. tostring(left))
assert(arrived == vim.uv.fs_realpath(b), "on_arrive got " .. tostring(arrived))

left, arrived = nil, nil
switch.to(b, "b")
assert(left == nil and arrived == nil, "hooks fired for a switch to the same place")

mia.setup({})
assert(switch.to(a, "a"))
vim.fn.delete(temp, "rf")
print("hooks ok")
