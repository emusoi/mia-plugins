local calls, opened, selected_from = {}, nil, nil
package.loaded["mia.api"] = {
  exec = function(argv)
    table.insert(calls, argv)
    if argv[2] == "list" then return '["postgres-locks","tmux-from-inside"]', nil end
    return "/tmp/lessons/" .. argv[#argv] .. ".md", nil
  end,
  query = function() return {}, nil end,
}
vim.ui.select = function(items, _, on_choice) selected_from = items; on_choice(items[2]) end
vim.cmd.edit = function(path) opened = path end

require("mia.lesson").open(nil, nil)
assert(selected_from and selected_from[1] == "postgres-locks" and selected_from[3] == "+ new lesson…", vim.inspect(selected_from))
assert(opened == "/tmp/lessons/tmux-from-inside.md", "picking a lesson did not open it: " .. tostring(opened))

print("lesson ok")
