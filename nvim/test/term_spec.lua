local argv, options
local original_jobstart = vim.fn.jobstart
vim.fn.jobstart = function(command, opts)
  argv, options = command, opts
  return 1
end
local term = require("mia.term")
local view = term.open("fixture")
assert(vim.deep_equal(argv, { "mia", "shell", "--popup", "fixture" }), vim.inspect(argv))
assert(options.term and vim.bo[view.buf].bufhidden == "wipe")
assert(vim.fn.maparg("<Esc>", "t") == "", "Escape must reach the shell")
vim.fn.maparg("<C-q>", "t", false, true).callback()
assert(not vim.api.nvim_win_is_valid(view.win), "Ctrl-Q did not close the popup")
assert(not vim.api.nvim_buf_is_valid(view.buf), "popup close retained the attachment buffer")
vim.fn.jobstart = original_jobstart
