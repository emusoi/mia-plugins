
local plan = require("mia.plan")

local function assert_equal(got, want, what)
  if got ~= want then
    error(("%s: got %s, want %s"):format(what, vim.inspect(got), vim.inspect(want)), 2)
  end
end

local report = {
  total = 4, proven = 2, attested = 1, stale = 0, claimed = 0, open = 1,
  steps = {
    { state = "proven",   because = "@imports passed at this commit",
      step = { section = "Implementation", text = "the module still imports", check = "imports", ticked = true, line = 4 } },
    { state = "proven",   because = "@tests passed at this commit",
      step = { section = "Implementation", text = "the tests pass", check = "tests", ticked = true, line = 5 } },
    { state = "open",     because = "@broken failed at this commit",
      step = { section = "Implementation", text = "Something nobody ran", check = "broken", ticked = true, line = 6 } },
    { state = "attested", because = "a person ticked it; no command can check this one",
      step = { section = "Review", text = "a second pair of eyes", check = "", ticked = true, line = 9 } },
  },
}

for _, state in ipairs({ "proven", "attested", "stale", "claimed", "open" }) do
  local sign = plan.signs[state]
  if not sign or not sign.text or sign.text:gsub("%s", "") == "" then
    error("no sign text for state " .. state)
  end
end

local buf = vim.api.nvim_create_buf(false, true)
vim.api.nvim_buf_set_lines(buf, 0, -1, false, {
  "# Feature A", "", "## Implementation", "- [x] the module still imports @imports",
  "- [x] the tests pass @tests", "- [x] Something nobody ran @broken", "",
  "## Review", "- [x] a second pair of eyes",
})
plan.place(buf, report)

local placed = vim.fn.sign_getplaced(buf, { group = "mia-plan" })[1].signs
assert_equal(#placed, 4, "signs placed")

local by_line = {}
for _, sign in ipairs(placed) do by_line[sign.lnum] = sign.name end
assert_equal(by_line[4], "mia-plan-proven", "line 4")
assert_equal(by_line[5], "mia-plan-proven", "line 5")
assert_equal(by_line[6], "mia-plan-open", "line 6 — a failing check is open, not proven")
assert_equal(by_line[9], "mia-plan-attested", "line 9 — a review step nothing can check")

plan.place(buf, report)
assert_equal(#vim.fn.sign_getplaced(buf, { group = "mia-plan" })[1].signs, 4, "signs after a refresh")

local summary = plan.summary(report)
for _, want in ipairs({ "2/4 proven", "1 attested", "1 open" }) do
  if not summary:find(want, 1, true) then
    error(("the summary %q does not say %q"):format(summary, want))
  end
end

assert_equal(plan.step_at(report, 5).step.check, "tests", "step at line 5")
assert_equal(plan.step_at(report, 7), nil, "a blank line has no step")

local api = require("mia.api")
local target = "/tmp/other worktree"
local data = { path = "/tmp/other-plan.md", worktree = target, report = report }
local checks = {}
local queried
local fresh = data
plan.query = function(worktree)
  queried = worktree
  return fresh
end
api.exec = function(argv) table.insert(checks, argv); return "" end
vim.notify = function() end
vim.api.nvim_set_current_buf(buf)
vim.bo[buf].buftype = ""
vim.b[buf].mia_plan = data
vim.api.nvim_win_set_cursor(0, { 5, 0 })
vim.bo[buf].modified = true
plan.check()
assert_equal(#checks, 0, "unsaved plan cannot run a check")
vim.bo[buf].modified = false
plan.check()
assert_equal(queried, target, "query stays with the plan's worktree")
assert(vim.deep_equal(checks[1], { "plan", "check", target, "the tests pass" }), "check ran against the wrong worktree or step")
fresh = { path = "/tmp/another-branch.md", worktree = target, report = report }
plan.check()
assert_equal(#checks, 1, "switching branches must not run the old plan against the new one")
