-- Options and Keymaps Configuration
-- Combined configuration for vim options, keymaps, and diagnostics

-- ===========================================================================
-- LEADER KEYS
-- ===========================================================================
vim.g.mapleader = " "
vim.g.maplocalleader = " "

-- Disable netrw (oil.nvim replaces it)
vim.g.loaded_netrw = 1
vim.g.loaded_netrwPlugin = 1

-- ===========================================================================
-- VIM OPTIONS
-- ===========================================================================

-- Enable 24-bit color
vim.opt.termguicolors = true

-- Tab settings
vim.opt.expandtab = true
vim.opt.tabstop = 2
vim.opt.softtabstop = 2
vim.opt.shiftwidth = 2

-- Folding (using treesitter)
vim.opt.foldmethod = "expr"
vim.opt.foldexpr = "v:lua.vim.treesitter.foldexpr()"
vim.opt.foldtext = "v:lua.vim.treesitter.foldtext()"
vim.opt.foldlevelstart = 99

-- Status column and line numbers
vim.opt.cursorline = true
vim.opt.number = true
vim.opt.relativenumber = true
vim.opt.signcolumn = "yes"
vim.opt.statuscolumn = "%!v:lua.require'config.statuscol'.statuscolumn()"

-- Auto-reload files changed outside of nvim
vim.opt.autoread = true

local watchers = {}
local function watch_buffer(buf)
	local path = vim.api.nvim_buf_get_name(buf)
	if path == "" or vim.uv.fs_stat(path) == nil then
		return
	end
	local old = watchers[buf]
	if old then
		old:stop()
	end
	local w = vim.uv.new_fs_event()
	if not w then
		return
	end
	w:start(
		path,
		{},
		vim.schedule_wrap(function()
			w:stop()
			if vim.api.nvim_buf_is_valid(buf) then
				vim.api.nvim_buf_call(buf, function()
					vim.cmd("silent! checktime")
				end)
				watch_buffer(buf)
			end
		end)
	)
	watchers[buf] = w
end
vim.api.nvim_create_autocmd("BufReadPost", {
	callback = function(args)
		watch_buffer(args.buf)
	end,
})
vim.api.nvim_create_autocmd("BufDelete", {
	callback = function(args)
		local w = watchers[args.buf]
		if w then
			w:stop()
			watchers[args.buf] = nil
		end
	end,
})

-- ===========================================================================
-- DIAGNOSTICS
-- ===========================================================================

vim.diagnostic.config({
	signs = {
		text = {
			[vim.diagnostic.severity.ERROR] = "",
			[vim.diagnostic.severity.WARN] = "",
			[vim.diagnostic.severity.HINT] = "",
			[vim.diagnostic.severity.INFO] = "",
		},
	},
	update_in_insert = false,
	underline = true,
	severity_sort = true,
	float = {
		focusable = true,
		style = "minimal",
		border = "single",
		source = "always",
		header = "Diagnostic",
		prefix = "",
	},
})

-- ===========================================================================
-- KEYMAPS
-- ===========================================================================

-- NOTE: Split movement handled by vim-tmux-navigator plugin (Ctrl+hjkl)

-- Search for visual selection with *
vim.keymap.set("v", "*", 'y/\\V<C-R>"<CR>', { desc = "Search visual selection" })

-- Clear search highlight on Esc
vim.keymap.set("n", "<Esc>", "<cmd>nohlsearch<CR>")

-- Black hole register instead of cut
vim.keymap.set("", "<leader>d", '"_d', { desc = "Delete to black hole register" })

-- Clipboard operations
vim.keymap.set({ "n", "v" }, "<leader>y", '"+y', { desc = "Yank to clipboard" })
vim.keymap.set("n", "<leader>p", '"+p', { desc = "Paste from clipboard" })
vim.keymap.set("n", "<leader>P", '"+P', { desc = "Paste from clipboard (before)" })
vim.keymap.set("n", "<leader>yl", function()
	local relative_path = vim.fn.expand("%:.")
	if relative_path == "" then
		return
	end

	local line_no = vim.api.nvim_win_get_cursor(0)[1]
	local location = string.format("%s:%d", relative_path, line_no)

	vim.fn.setreg("+", location)
	vim.fn.setreg('"', location)
	vim.notify("Copied " .. location)
end, { desc = "Yank relative path with line" })

-- Select all
vim.keymap.set("n", "<leader>sa", "ggVG", { desc = "Select all" })

local function unescape_text(text)
	text = text:gsub("\\r\\n", "\n")
	text = text:gsub("\\u([dD][89aAbB]%x%x)\\u([dD][c-fC-F]%x%x)", function(high, low)
		local high_code = tonumber(high, 16)
		local low_code = tonumber(low, 16)
		local codepoint = 0x10000 + (high_code - 0xD800) * 0x400 + (low_code - 0xDC00)
		return vim.fn.nr2char(codepoint, true)
	end)
	text = text:gsub("\\u(%x%x%x%x)", function(hex)
		local codepoint = tonumber(hex, 16)
		if codepoint >= 0xD800 and codepoint <= 0xDFFF then
			return "\\u" .. hex
		end
		return vim.fn.nr2char(codepoint, true)
	end)

	local escapes = {
		['"'] = '"',
		["'"] = "'",
		["\\"] = "\\",
		["/"] = "/",
		b = "\b",
		f = "\f",
		n = "\n",
		r = "\n",
		t = "\t",
	}

	return (text:gsub("\\(.)", function(char)
		return escapes[char] or "\\" .. char
	end))
end

vim.api.nvim_create_user_command("UnescapeText", function(opts)
	local lines = vim.api.nvim_buf_get_lines(0, opts.line1 - 1, opts.line2, false)
	if #lines == 0 then
		return
	end

	local text = table.concat(lines, "\n")
	local unescaped = unescape_text(text)
	local new_lines = vim.split(unescaped, "\n", { plain = true })

	vim.api.nvim_buf_set_lines(0, opts.line1 - 1, opts.line2, false, new_lines)
end, { range = "%", desc = "Unescape backslash text" })

vim.keymap.set("n", "<leader>su", "<cmd>UnescapeText<CR>", { desc = "Unescape text" })
vim.keymap.set("x", "<leader>su", ":UnescapeText<CR>", { desc = "Unescape text" })

-- Tab keymaps
vim.keymap.set("n", "<leader><Tab>n", "<cmd>tabnew<cr>", { desc = "New tab" })
vim.keymap.set("n", "<leader><Tab>c", "<cmd>tabclose<cr>", { desc = "Close tab" })

-- Diagnostic keymaps
vim.keymap.set("n", "<leader>e", vim.diagnostic.open_float, { desc = "Show diagnostic" })
vim.keymap.set("n", "[d", vim.diagnostic.goto_prev, { desc = "Previous diagnostic" })
vim.keymap.set("n", "]d", vim.diagnostic.goto_next, { desc = "Next diagnostic" })

-- ===========================================================================
-- SESSION MANAGEMENT
-- ===========================================================================

local session_dir = vim.fn.stdpath("state") .. "/sessions/"
vim.fn.mkdir(session_dir, "p")

local function session_file()
	return session_dir .. vim.fn.getcwd():gsub("[/\\]", "%%") .. ".vim"
end

local function should_manage_session()
	-- skip when nvim was opened with file arguments (e.g. $EDITOR tempfile)
	return vim.fn.argc() == 0
end

vim.api.nvim_create_autocmd("VimLeavePre", {
	callback = function()
		if should_manage_session() then
			vim.cmd("mksession! " .. vim.fn.fnameescape(session_file()))
		end
		-- Force-stop LSP clients to avoid blocking on graceful shutdown
		for _, client in ipairs(vim.lsp.get_clients()) do
			client.stop(true)
		end
	end,
})

vim.api.nvim_create_autocmd("VimEnter", {
	nested = true,
	callback = function()
		if not should_manage_session() then
			return
		end
		local f = session_file()
		if vim.fn.filereadable(f) == 1 then
			vim.cmd("source " .. vim.fn.fnameescape(f))
		end
	end,
})
