local reports = {}
vim.health = {
  start = function(name) table.insert(reports, "start " .. name) end,
  ok = function(m) table.insert(reports, "ok " .. m) end,
  warn = function(m) table.insert(reports, "warn " .. m) end,
  error = function(m) table.insert(reports, "error " .. m) end,
  info = function(m) table.insert(reports, "info " .. m) end,
}
require("mia.health").check()
assert(reports[1] == "start mia", vim.inspect(reports))
assert(#reports >= 2, "health said nothing after starting")
print("health ok")
