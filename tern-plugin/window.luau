local visible = true
local usage = nil
local command = "plugin.ai-usage.detail"

local function epoch(iso)
	local y, m, d, h, min, sec = string.match(iso, "^(%d+)-(%d+)-(%d+)T(%d+):(%d+):(%d+)")
	if not y then return nil end
	local year, month = tonumber(y), tonumber(m)
	if month <= 2 then year -= 1 end
	local era = year // 400
	local yoe = year - era * 400
	local doy = (153 * (if month > 2 then month - 3 else month + 9) + 2) // 5 + tonumber(d) - 1
	local days = era * 146097 + yoe * 365 + yoe // 4 - yoe // 100 + doy - 719468
	local value = days * 86400 + tonumber(h) * 3600 + tonumber(min) * 60 + tonumber(sec)
	local sign, oh, om = string.match(iso, "([+-])(%d%d):(%d%d)$")
	if sign then value += (if sign == "+" then -1 else 1) * (tonumber(oh) * 3600 + tonumber(om) * 60) end
	return value
end

local function remaining(value)
	if type(value) ~= "string" then return "—" end
	local timestamp = epoch(value)
	if not timestamp then return "—" end
	local seconds = math.max(0, timestamp - os.time())
	if seconds < 3600 then return tostring(seconds // 60) .. "m" end
	if seconds < 86400 then return string.format("%dh %02dm", seconds // 3600, seconds // 60 % 60) end
	return string.format("%dd %02dh", seconds // 86400, seconds // 3600 % 24)
end

local function add_provider_segments(segments, name, result)
	if type(result) ~= "table" or not result.ok then
		table.insert(segments, {text = name .. " error", tone = "error", command = command})
		return
	end
	local windows = result.windows or {}
	if #windows == 0 then
		table.insert(segments, {text = name .. " sin cuota", tone = "muted", command = command})
		return
	end
	for index, window in windows do
		local prefix = if index == 1 then name .. " " else ""
		local label = if window.kind == "corta" then "C" elseif window.kind == "semanal" then "S" else tostring(window.kind or "cuota")
		local text, tone
		if window.unlimited then
			text, tone = prefix .. label .. ":∞", "muted"
		else
			local used = window.used_percent
			if type(used) ~= "number" then
				text, tone = prefix .. label .. ":error", "error"
			else
				tone = if used <= 30 then "ok" elseif used <= 65 then "warn" else "error"
				text = string.format("%s%s:%.0f%%", prefix, label, used)
			end
		end
		table.insert(segments, {text = text, tone = tone, command = command})
		if not window.unlimited and type(window.used_percent) == "number" then
			table.insert(segments, {text = remaining(window.resets_at), icon = "clock", tone = "ai-usage-reset", command = command})
		end
	end
end

tern.chrome.status(function(_pane)
	local segments = {{text = if visible then "Cuotas" else "Cuotas: ocultas", icon = "chart", tone = "muted", command = "plugin.ai-usage.toggle"}}
	if not visible then return segments end
	if type(usage) == "table" and type(usage.snapshot) == "table" then
		local first = true
		for _, name in {"Codex", "MiniMax", "z.ai"} do
			if usage.snapshot[name] then
				if not first then table.insert(segments, {text = "│", tone = "muted"}) end
				add_provider_segments(segments, name, usage.snapshot[name])
				first = false
			end
		end
		if usage.fetched and os.time() - usage.fetched > (usage.interval or 60) * 2 then
			table.insert(segments, {text = "Datos antiguos", tone = "warn", command = command})
		end
	end
	if type(usage) ~= "table" then
		table.insert(segments, {text = "AI Usage: consultando…", tone = "muted"})
	elseif usage.problem then
		table.insert(segments, {text = "AI Usage: error", tone = "error", command = command})
	end
	return segments
end)

local function refresh()
	local ok, data = pcall(tern.kv.get, "usage")
	usage = if ok then data else {problem = "No se pudo leer el estado"}
	tern.chrome.refresh()
	tern.timer(5000, refresh)
end

local function toggle()
	visible = not visible
	tern.chrome.refresh()
end
tern.files.button({id = "toggle", title = "Mostrar u ocultar AI Usage", icon = "chart", run = toggle})
tern.command({id = "toggle", title = "Mostrar u ocultar AI Usage", run = toggle})
tern.command({id = "detail", title = "Abrir detalle de AI Usage", run = function(cx)
	local target = cx.session:focused()
	if not target then return end
	local binary = (tern.getenv("HOME") or "") .. "/go/bin/ai-usage"
	if not tern.fs.exists(binary) then binary = "ai-usage" end
	local pane = cx.layout:split(target, "right", {command = string.format("%q --watch", binary)})
	if not pane then cx:toast("error", "AI Usage", "No se pudo abrir el detalle.") end
end})
tern.command({id = "settings", title = "Abrir ajustes de AI Usage", run = function(cx)
	local dir = tern.getenv("XDG_CONFIG_HOME")
	if not dir or dir == "" then dir = (tern.getenv("HOME") or "") .. "/.config" end
	cx:open(dir .. "/ai-usage/config.json", "beside")
end})
refresh()
