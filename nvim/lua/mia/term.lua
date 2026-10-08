local api = require("mia.api")

local M = {}

function M.run(argv, name, on_done, opts)
  opts = opts or {}
  name = name or ""
  local buf = vim.api.nvim_create_buf(false, true)
  vim.bo[buf].bufhidden = "wipe"
  local width = opts.width or math.min(120, math.floor(vim.o.columns * 0.9))
  local height = opts.height or math.min(36, math.floor(vim.o.lines * 0.85))
  local config = {
    relative = "editor",
    width = width,
    height = height,
    row = math.floor((vim.o.lines - height) / 2),
    col = math.floor((vim.o.columns - width) / 2),
    style = "minimal",
    border = opts.frameless and "none" or "rounded",
  }
  if not opts.frameless then config.title = (" terminal · %s "):format(name) end
  local win = vim.api.nvim_open_win(buf, true, config)

  local function close()
    if vim.api.nvim_win_is_valid(win) then vim.api.nvim_win_close(win, true) end
  end

  local job = { term = true, on_exit = function(_, code)
    if code == 0 then
      close()
    elseif vim.api.nvim_win_is_valid(win) then
      vim.cmd.stopinsert()
      vim.api.nvim_win_set_config(win, { footer = " Failed · q / Ctrl-Q close " })
    end
    if on_done then vim.schedule(function() on_done(code) end) end
  end }
  if os.getenv("TMUX_PANE") then job.env = { MIA_TMUX_PANE = os.getenv("TMUX_PANE") } end
  vim.fn.jobstart({ "mia", unpack(argv) }, job)

  vim.keymap.set({ "n", "t" }, "<C-q>", close, { buffer = buf, nowait = true })
  vim.keymap.set("n", "q", close, { buffer = buf, nowait = true })
  vim.cmd.startinsert()
  return { buf = buf, win = win, title = (" terminal · %s "):format(name) }
end

function M.open(worktree)
  local argv = { "shell" }
  if os.getenv("TMUX") == nil then table.insert(argv, "--popup") end
  if worktree and worktree ~= "" then table.insert(argv, worktree) end
  local name = worktree
  if not name or name == "" then
    local here = api.query("worktrees")
    local cwd = vim.fn.getcwd()
    for _, listing in ipairs(here or {}) do
      if listing.path == cwd then name = listing.name end
    end
    name = name or vim.fn.fnamemodify(cwd, ":t")
  end
  return M.run(argv, name)
end

return M
