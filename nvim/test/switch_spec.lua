local switch = require("mia.switch")
local temp = vim.fn.tempname()
local first, second = temp .. "/first", temp .. "/second"

vim.o.swapfile = false
vim.fn.mkdir(first, "p")
local function git(dir, args)
  local argv = { "git", "-C", dir }
  vim.list_extend(argv, args)
  local result = vim.system(argv, { text = true }):wait()
  assert(result.code == 0, result.stderr)
end

git(first, { "init", "-q", "-b", "main" })
git(first, { "config", "user.name", "Mia Test" })
git(first, { "config", "user.email", "test@example.invalid" })
vim.fn.writefile({ "one", "two", "three" }, first .. "/one.txt")
vim.fn.writefile({ "left", "right" }, first .. "/two.txt")
git(first, { "add", "." })
git(first, { "commit", "-qm", "fixture" })
git(first, { "worktree", "add", "-qb", "other", second })
first, second = vim.uv.fs_realpath(first), vim.uv.fs_realpath(second)
vim.fn.writefile({ "only here" }, first .. "/missing.txt")

local original = vim.fn.getcwd()
local notices = {}
vim.notify = function(message, level) notices[#notices + 1] = { message = message, level = level } end
package.loaded["mia.api"].exec = function(argv)
  assert(vim.deep_equal(argv, { "path", "other" }), vim.inspect(argv))
  return second .. "\n", nil
end

vim.cmd.cd(vim.fn.fnameescape(first))
vim.cmd.edit(vim.fn.fnameescape(first .. "/one.txt"))
vim.api.nvim_win_set_cursor(0, { 3, 0 })
vim.cmd.vsplit(vim.fn.fnameescape(first .. "/two.txt"))
vim.api.nvim_win_set_cursor(0, { 2, 0 })
local missing = vim.fn.bufadd(first .. "/missing.txt")
vim.fn.bufload(missing)

local result = assert(switch.worktree("other"))
assert(result.target == vim.uv.fs_realpath(second))
assert(result.moved == 2 and #result.kept == 1 and result.kept[1] == "missing.txt", vim.inspect(result))
assert(vim.fn.getcwd() == vim.uv.fs_realpath(second), "tab cwd did not move")
for _, win in ipairs(vim.api.nvim_tabpage_list_wins(0)) do
  local name = vim.api.nvim_buf_get_name(vim.api.nvim_win_get_buf(win))
  assert(name:sub(1, #second + 1) == second .. "/", "window kept old source: " .. name)
end
local cursors = {}
for _, win in ipairs(vim.api.nvim_tabpage_list_wins(0)) do
  cursors[vim.fn.fnamemodify(vim.api.nvim_buf_get_name(vim.api.nvim_win_get_buf(win)), ":t")] =
    vim.api.nvim_win_get_cursor(win)[1]
end
assert(cursors["one.txt"] == 3 and cursors["two.txt"] == 2, vim.inspect(cursors))

vim.cmd.only()
vim.cmd.edit(vim.fn.fnameescape(second .. "/one.txt"))
vim.api.nvim_buf_set_lines(0, 0, 1, false, { "unsaved" })
result = assert(switch.to(first, "first"))
assert(#result.dirty == 1 and result.dirty[1] == "one.txt", vim.inspect(result))
assert(vim.bo.modified and vim.api.nvim_get_current_line() == "unsaved", "switch discarded the modified buffer")
assert(vim.fn.getcwd() == vim.uv.fs_realpath(first), "cwd did not move past a safe dirty buffer")
assert(notices[#notices].level == vim.log.levels.WARN, "dirty switch did not warn")

assert(switch.to(first, "first")) -- staying put must not add a history entry
assert(not switch.to(temp .. "/gone"), "invalid destination was accepted")
result = assert(switch.back())
assert(result.target == second and vim.fn.getcwd() == second)
assert(vim.bo.modified and vim.api.nvim_get_current_line() == "unsaved", "back lost the unsaved edit")
assert(switch.back().target == first, "back did not retrace the earlier switch")
assert(not switch.back() and vim.fn.getcwd() == first, "back bounced between two worktrees")

assert(switch.to(second, "second"))
local editing_tab = vim.api.nvim_get_current_tabpage()
vim.cmd.tabnew()
assert(not switch.back(), "a new tab inherited another tab's worktree history")
vim.cmd.tabclose()
assert(vim.api.nvim_get_current_tabpage() == editing_tab)
assert(switch.back().target == first, "another tab consumed the original history")

assert(switch.to(second, "second"))
assert(vim.uv.fs_rename(first, temp .. "/away"))
assert(not switch.back() and vim.fn.getcwd() == second, "missing previous worktree changed cwd")
assert(vim.uv.fs_rename(temp .. "/away", first))
assert(switch.back().target == first, "failed return discarded history")

vim.bo.modified = false
vim.cmd("silent! %bwipeout!")
vim.cmd.cd(vim.fn.fnameescape(original))
vim.fn.delete(temp, "rf")
print("switch follows clean buffers and preserves dirty work")
