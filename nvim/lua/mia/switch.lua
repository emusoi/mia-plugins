local api = require("mia.api")

local M = {}

local function root(dir)
  local result = vim.system({ "git", "-C", dir, "rev-parse", "--show-toplevel" }, { text = true }):wait()
  if result.code ~= 0 then return nil end
  local path = (result.stdout or ""):gsub("%s+$", "")
  return vim.uv.fs_realpath(path) or path
end

local function under(path, parent)
  return path:sub(1, #parent + 1) == parent .. "/"
end

local function files_in(parent)
  local files = {}
  for _, buf in ipairs(vim.api.nvim_list_bufs()) do
    if vim.api.nvim_buf_is_loaded(buf) and vim.bo[buf].buftype == "" then
      local name = vim.api.nvim_buf_get_name(buf)
      if name ~= "" and under(name, parent) then
        files[#files + 1] = {
          buf = buf,
          relative = name:sub(#parent + 2),
          modified = vim.bo[buf].modified,
        }
      end
    end
  end
  return files
end

local function windows_by_buffer()
  local windows = {}
  for _, win in ipairs(vim.api.nvim_list_wins()) do
    if vim.api.nvim_win_is_valid(win) then
      local buf = vim.api.nvim_win_get_buf(win)
      windows[buf] = windows[buf] or {}
      windows[buf][#windows[buf] + 1] = { win = win, cursor = vim.api.nvim_win_get_cursor(win) }
    end
  end
  return windows
end

M.hooks = {}

function M.to(target, label, opts)
  target = vim.uv.fs_realpath(target) or target
  if vim.fn.isdirectory(target) == 0 then
    vim.notify("mia: no directory at " .. target, vim.log.levels.ERROR)
    return nil
  end

  local from = root(vim.fn.getcwd())
  if from and from ~= target and M.hooks.on_leave then M.hooks.on_leave(from) end
  local moved, kept, dirty = 0, {}, {}
  if from and from ~= target then
    local showing = windows_by_buffer()
    for _, file in ipairs(files_in(from)) do
      local destination = target .. "/" .. file.relative
      if file.modified then
        dirty[#dirty + 1] = file.relative
      elseif vim.fn.filereadable(destination) == 0 then
        kept[#kept + 1] = file.relative
      else
        local replacement = vim.fn.bufadd(destination)
        vim.fn.bufload(replacement)
        for _, view in ipairs(showing[file.buf] or {}) do
          if vim.api.nvim_win_is_valid(view.win) then
            vim.api.nvim_win_set_buf(view.win, replacement)
            local last = vim.api.nvim_buf_line_count(replacement)
            vim.api.nvim_win_set_cursor(view.win, { math.min(view.cursor[1], last), view.cursor[2] })
          end
        end
        if vim.api.nvim_buf_is_valid(file.buf) then
          vim.api.nvim_buf_delete(file.buf, { force = false })
        end
        moved = moved + 1
      end
    end
  end

  vim.cmd.tcd(vim.fn.fnameescape(target))
  if from ~= target and M.hooks.on_arrive then M.hooks.on_arrive(target) end
  if from and from ~= target and not (opts and opts.record == false) then
    local history = vim.t.mia_worktree_history or {}
    history[#history + 1] = from
    vim.t.mia_worktree_history = history
  end
  local summary = { "switched to " .. (label or vim.fn.fnamemodify(target, ":t")) }
  if moved > 0 then summary[#summary + 1] = moved .. (moved == 1 and " file followed" or " files followed") end
  if #kept > 0 then summary[#summary + 1] = #kept .. " absent there: " .. table.concat(kept, ", ") end
  if #dirty > 0 then summary[#summary + 1] = #dirty .. " unsaved, left open: " .. table.concat(dirty, ", ") end
  vim.notify("mia: " .. table.concat(summary, " · "), #dirty > 0 and vim.log.levels.WARN or vim.log.levels.INFO)
  return { target = target, moved = moved, kept = kept, dirty = dirty }
end

function M.back()
  local history = vim.t.mia_worktree_history or {}
  local previous = history[#history]
  if not previous then
    vim.notify("mia: no previous worktree in this tab", vim.log.levels.INFO)
    return nil
  end
  local result = M.to(previous, nil, { record = false })
  if result then
    table.remove(history)
    vim.t.mia_worktree_history = history
  end
  return result
end

function M.worktree(name)
  if not name or name == "" then
    vim.notify("mia: switch needs a worktree name", vim.log.levels.ERROR)
    return nil
  end
  local target, err = api.exec({ "path", name })
  if err then
    vim.notify("mia: " .. err, vim.log.levels.ERROR)
    return nil
  end
  return M.to(target:gsub("%s+$", ""), name)
end

return M
