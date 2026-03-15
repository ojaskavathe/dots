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
