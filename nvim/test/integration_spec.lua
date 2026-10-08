local temp = vim.fn.tempname()
local original_cwd, original_config = vim.fn.getcwd(), vim.env.XDG_CONFIG_HOME
vim.fn.mkdir(temp .. "/repo", "p")
vim.o.swapfile = false
vim.o.shadafile = "NONE"
vim.env.XDG_CONFIG_HOME = temp .. "/config"
vim.cmd.cd(vim.fn.fnameescape(temp .. "/repo"))

local function run(argv)
  local result = vim.system(argv, { text = true }):wait(10000)
  assert(result, "timed out: " .. table.concat(argv, " "))
  assert(result.code == 0, table.concat(argv, " ") .. ": " .. (result.stderr or "timed out"))
  return result.stdout
end
local function check()
  run({ "git", "init", "-q", "-b", "main" })
  run({ "git", "config", "user.email", "lab@example.invalid" })
  run({ "git", "config", "user.name", "mia lab" })
  vim.fn.writefile({ "fixture" }, "README.md")
  run({ "git", "add", "README.md" })
  run({ "git", "commit", "-qm", "Initial fixture" })
  run({ "mia", "adopt", "." })
  run({ "mia", "new", "feature" })

  local api = require("mia.api")
  local dash, err = api.panel("dashboard")
  assert(dash, err)
  assert(dash.version == 2, vim.inspect(dash))

  local trees = assert(api.query("worktrees"))
  local target
  for _, tree in ipairs(trees) do if tree.branch == "feature" then target = tree.path end end
  assert(target, "missing feature worktree")

  vim.cmd("silent! %bwipeout!")
  vim.cmd.edit("README.md")
  local source, source_buf = vim.fn.getcwd(), vim.api.nvim_get_current_buf()
  vim.fn.writefile({ 'prefix = "codex/"' }, ".git/mia/config.toml")
  local answer, notice
  vim.ui.input = function(_, callback) callback(answer) end
  vim.notify = function(message) notice = message end
  answer = "from-editor"
  require("mia").open("new")
  local created = vim.fn.getcwd()
  assert(created ~= source and run({ "git", "branch", "--show-current" }) == "codex/from-editor\n")
  assert(vim.api.nvim_buf_get_name(0) == created .. "/README.md", "source buffer did not follow creation")
  assert(#assert(api.query("worktrees")) == 3)
  require("mia").open("new from-editor")
  assert(vim.fn.getcwd() == created and #assert(api.query("worktrees")) == 3, "repeating new should open the existing worktree")
  require("mia").open("back")
  assert(vim.fn.getcwd() == source, "back did not return")
  require("mia").open("back")
  assert(vim.fn.getcwd() == source and notice:find("no previous worktree", 1, true), "empty back history should report without moving")
  assert(#assert(api.query("worktrees")) == 3, "records drifted after ordinary work")
end

local ok, err = pcall(check)
vim.cmd("silent! %bwipeout!")
vim.cmd.cd(vim.fn.fnameescape(original_cwd))
vim.env.XDG_CONFIG_HOME = original_config
vim.fn.delete(temp, "rf")
assert(ok, err)
