local function tmux_pane()
	if vim.env.TMUX == nil or vim.env.TMUX == "" then
		return nil
	end
	if vim.env.TMUX_PANE == nil or vim.env.TMUX_PANE == "" then
		return nil
	end
	return vim.env.TMUX_PANE
end

local function start_detached_job(cmd)
	if vim.fn.executable(cmd[1]) ~= 1 then
		return
	end
	vim.fn.jobstart(cmd, { detach = true })
end

local function set_tmux_pane_option(name, value)
	local pane = tmux_pane()
	if pane == nil then
		return
	end

	local cmd = { "tmux", "set-option", "-pt", pane }
	if value == nil then
		vim.list_extend(cmd, { "-u", name })
	else
		vim.list_extend(cmd, { name, value })
	end
	start_detached_job(cmd)
end

local function register_nvim_server()
	if tmux_pane() == nil then
		return
	end

	local server = vim.v.servername
	if server == nil or server == "" then
		local ok, started = pcall(vim.fn.serverstart)
		if not ok then
			return
		end
		server = started
	end

	set_tmux_pane_option("@nvim_server", server)
	set_tmux_pane_option("@nvim_equalize_pane", nil)
end

vim.api.nvim_create_autocmd("VimEnter", {
	callback = register_nvim_server,
})

vim.api.nvim_create_autocmd("VimLeavePre", {
	callback = function()
		set_tmux_pane_option("@nvim_equalize_pane", nil)
		set_tmux_pane_option("@nvim_server", nil)
	end,
})

vim.schedule(register_nvim_server)
