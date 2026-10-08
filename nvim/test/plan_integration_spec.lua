local here = debug.getinfo(1, "S").source:sub(2):match("(.*)/nvim/test/")
local temp = vim.fn.tempname()
local original_cwd, original_config = vim.fn.getcwd(), vim.env.XDG_CONFIG_HOME
vim.fn.mkdir(temp .. "/repo", "p")
vim.fn.mkdir(temp .. "/config/mia/plugins", "p")
vim.o.swapfile = false
vim.o.shadafile = "NONE"
vim.env.XDG_CONFIG_HOME = temp .. "/config"
vim.uv.fs_symlink(here .. "/bin/mia-plan", temp .. "/config/mia/plugins/mia-plan")
vim.cmd.cd(vim.fn.fnameescape(temp .. "/repo"))

local function run(argv)
  local result = vim.system(argv, { text = true }):wait(10000)
  assert(result and result.code == 0, table.concat(argv, " ") .. ": " .. (result and result.stderr or "timed out"))
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
  run({ "mia", "plugin", "enable", "plan" })
  vim.fn.writefile({ "[plugin.plan.checks]", 'smoke = ["sh", "-c", "exit 0"]' }, ".git/mia/config.toml")

  local api = require("mia.api")
  local target
  for _, tree in ipairs(assert(api.query("worktrees"))) do if tree.branch == "feature" then target = tree.path end end
  assert(target, "missing feature worktree")

  require("mia").setup()
  local plan = require("mia.plan")
  local view = assert(plan.open(target), "targeted plan did not open")
  assert(view.data.worktree == target and view.data.branch == "feature", "opened the wrong plan: " .. vim.inspect(view.data))
  vim.api.nvim_buf_set_lines(view.buf, 0, -1, false, {
    "# Feature", "", "## Implementation", "- [ ] fixture command passes @smoke",
  })
  vim.cmd.write()
  vim.api.nvim_win_set_cursor(0, { 4, 0 })
  plan.check()
  local proven = assert(plan.query(target))
  assert(proven.report.proven == 1, vim.inspect(proven.report))
  assert(assert(plan.query()).report.total == 0, "check or plan was written to the current branch")
  assert(vim.fn.sign_getplaced(view.buf, { group = "mia-plan" })[1].signs[1].name == "mia-plan-proven")
  assert(require("mia").views():find("lesson", 1, true), ":Mia lesson is not a view once set up")
end

local ok, err = pcall(check)
vim.cmd("silent! %bwipeout!")
vim.cmd.cd(vim.fn.fnameescape(original_cwd))
vim.env.XDG_CONFIG_HOME = original_config
vim.fn.delete(temp, "rf")
assert(ok, err)
