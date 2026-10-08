
local M = {}

function M.check()
  local health = vim.health
  health.start("mia")

  if vim.fn.executable("mia") ~= 1 then
    health.error("`mia` is not on PATH", { "make install in the mia checkout" })
    return
  end
  local version = vim.system({ "mia", "version" }, { text = true }):wait()
  health.ok("mia " .. vim.trim(version.stdout or "?"))

  local api = require("mia.api")
  local probe = vim.system({ "mia", "api", "dashboard" }, { text = true }):wait()
  if probe.code ~= 0 then
    health.warn("`mia api dashboard` failed here — not in a repository?", { vim.trim(probe.stderr or "") })
  else
    local ok, panel = pcall(vim.json.decode, probe.stdout)
    if ok and panel.version == api.IR_VERSION then
      health.ok(("panel version %d matches"):format(api.IR_VERSION))
    else
      health.error(("mia speaks panel version %s; this plugin understands %s"):format(
        tostring(ok and panel.version or "?"), tostring(api.IR_VERSION)), { "update one of them" })
    end
  end

  if vim.fn.executable("tmux") == 1 then
    health.ok("tmux found — sessions and agents work")
  else
    health.warn("tmux not found — `:Mia term` and agents need it", { "brew install tmux" })
  end
  if os.getenv("TMUX") then
    health.ok("inside tmux — shells and agents open in the outer client, not nested")
  else
    health.info("not inside tmux — shells open in a float")
  end
end

return M
