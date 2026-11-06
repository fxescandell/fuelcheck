local interval = 60
local last = nil

local function config_path()
	local dir = tern.getenv("XDG_CONFIG_HOME")
	if not dir or dir == "" then
		dir = (tern.getenv("HOME") or "") .. "/.config"
	end
	return dir .. "/ai-usage/config.json"
end

local function read_interval()
	local ok, text = pcall(tern.fs.read, config_path())
	if ok then
		local valid, cfg = pcall(tern.json.decode, text)
		if valid and type(cfg) == "table" and type(cfg.interval_seconds) == "number" and cfg.interval_seconds > 0 then
			return cfg.interval_seconds
		end
	end
	return 60
end

local poll
local function publish(problem)
	local ok, err = pcall(tern.kv.set, "usage", {
		snapshot = last and last.snapshot,
		fetched = last and last.fetched,
		interval = interval,
		problem = problem,
	})
	if not ok then tern.log.warn("AI Usage: no se pudo guardar el estado", tostring(err)) end
	tern.timer(interval * 1000, poll)
end

poll = function()
	interval = read_interval()
	local home = tern.getenv("HOME") or ""
	local candidates = {home .. "/go/bin/ai-usage", "ai-usage", home .. "/.local/bin/ai-usage", "/opt/homebrew/bin/ai-usage", "/usr/local/bin/ai-usage"}
	for _, binary in candidates do
		local ok = pcall(tern.process.run, {binary, "--json"}, {timeout_ms = 45000}, function(result)
			local valid, data = pcall(tern.json.decode, result.stdout)
			if valid and type(data) == "table" and next(data) ~= nil then
				last = {snapshot = data, fetched = os.time()}
				publish(nil)
			else
				publish("No se pudo consultar ai-usage; comprueba la configuración")
			end
		end)
		if ok then return end
	end
	publish("No se encuentra ai-usage; comprueba la instalación")
end

local ok, saved = pcall(tern.kv.get, "usage")
if ok and type(saved) == "table" and saved.snapshot then
	last = {snapshot = saved.snapshot, fetched = saved.fetched}
end
poll()
