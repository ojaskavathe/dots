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
	w:start(path, {}, vim.schedule_wrap(function()
		w:stop()
		if vim.api.nvim_buf_is_valid(buf) then
			vim.api.nvim_buf_call(buf, function()
				vim.cmd("silent! checktime")
			end)
			watch_buffer(buf)
		end
	end))
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

-- Select all
vim.keymap.set("n", "<leader>sa", "ggVG", { desc = "Select all" })

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
