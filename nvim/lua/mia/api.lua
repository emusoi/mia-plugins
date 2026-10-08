
local M = {}

M.IR_VERSION = 2

local function run(args)
  local command = { "mia" }
  vim.list_extend(command, args)
  local result = vim.system(command, { text = true }):wait()
  if result.code ~= 0 then
    local detail = ((result.stdout or "") .. (result.stderr or "")):gsub("%s+$", "")
    if detail == "" then detail = "mia " .. table.concat(args, " ") .. " failed" end
    return nil, detail
  end
  return result.stdout, nil
end

function M.query(name, target)
  local args = { "api", name }
  if target and target ~= "" then table.insert(args, target) end
  local out, err = run(args)
  if err then return nil, err end
  local ok, decoded = pcall(vim.json.decode, out)
  if not ok then return nil, "mia api " .. name .. " did not return JSON" end
  return decoded, nil
end

function M.panel(name, target)
  local data, err = M.query(name, target)
  if err then return nil, err end
  if data.version ~= M.IR_VERSION then
    return nil, ("this mia speaks panel version %s; the plugin understands %s — update one of them")
      :format(tostring(data.version), tostring(M.IR_VERSION))
  end
  return data, nil
end

function M.exec(argv)
  local out, err = run(argv)
  if err then return nil, err end
  return out, nil
end

return M
