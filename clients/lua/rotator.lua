-- Minimal Rotator HTTP client for Lua
-- Requires: lua-socket, lua-cjson
-- Usage: lua rotator.lua [command]

local has_http, http = pcall(require, "socket.http")
if not has_http then
  io.stderr:write("error: lua-socket is required. Install with: luarocks install luasocket\n")
  os.exit(1)
end

local has_json, json = pcall(require, "cjson")
if not has_json then
  io.stderr:write("error: lua-cjson is required. Install with: luarocks install lua-cjson\n")
  os.exit(1)
end

local has_ltn12, ltn12 = pcall(require, "ltn12")
if not has_ltn12 then
  io.stderr:write("error: lua-socket is required. Install with: luarocks install luasocket\n")
  os.exit(1)
end

local base_url = os.getenv("ROTATOR_URL") or "http://localhost:8090/api/v1"

local commands = {
  status = function()
    local body, status = http.request(base_url .. "/status")
    if tonumber(status) ~= 200 then
      io.stderr:write("error: " .. tostring(status) .. "\n")
      return
    end
    print(json.decode(body))
  end,

  current = function()
    local body, status = http.request(base_url .. "/rotations/current")
    if tonumber(status) ~= 200 then
      io.stderr:write("error: " .. tostring(status) .. "\n")
      return
    end
    local rotation = json.decode(body)
    for _, item in ipairs(rotation.items) do
      io.write(string.format("%d. %s S%02dE%02d - %s [%s]\n",
        item.position,
        item.series_title,
        item.season_number,
        item.episode_number,
        item.episode_title,
        item.slot_kind
      ))
    end
  end,

  generate = function()
    local response_body = {}
    local _, status = http.request({
      url = base_url .. "/rotations/generate",
      method = "POST",
      headers = {["Content-Type"] = "application/json"},
      source = ltn12.source.string("{}"),
      sink = ltn12.sink.table(response_body)
    })
    if tonumber(status) ~= 201 then
      io.stderr:write("error: " .. tostring(status) .. "\n")
      return
    end
    print(table.concat(response_body))
  end,

  series = function()
    local body, status = http.request(base_url .. "/series")
    if tonumber(status) ~= 200 then
      io.stderr:write("error: " .. tostring(status) .. "\n")
      return
    end
    print(body)
  end
}

local cmd = arg[1] or "status"
if commands[cmd] then
  commands[cmd]()
else
  io.stderr:write("usage: lua rotator.lua {status|current|generate|series}\n")
  os.exit(2)
end