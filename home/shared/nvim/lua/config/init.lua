-- Main configuration entry point
-- This file registers lze handlers and loads all plugin configurations

-- ===========================================================================
-- OPTIONS AND KEYMAPS
-- ===========================================================================
require("config.opts_and_keys")

-- ===========================================================================
-- COLORSCHEME
-- ===========================================================================
local colorschemeName = nixCats("colorscheme") or "catppuccin"
vim.cmd.colorscheme(colorschemeName)

-- ===========================================================================
-- TREESITTER
-- ===========================================================================
-- highlight and indent are enabled by default in newer nvim-treesitter

-- ===========================================================================
-- REGISTER LZE HANDLERS
-- ===========================================================================

-- NOTE: Register custom handlers before loading plugins
-- These must be registered before any lze.load() calls

-- for_cat handler: Makes category checking cleaner
-- Instead of: enabled = nixCats('category')
-- Use: for_cat = 'category'
if nixCats then
	require("lze").register_handlers({
		for_cat = {
			spec_field = "for_cat",
			handler = function(plugin)
				local cat = plugin.for_cat
				if type(cat) == "string" then
					return nixCats(cat) == true
				end
				return true
			end,
		},
	})
end

-- LSP handler from lzextras: Allows defining LSPs as lze specs
-- This handler triggers lspconfig setup on the correct filetypes
require("lze").register_handlers(require("lzextras").lsp)

-- ===========================================================================
-- LOAD PLUGINS
-- ===========================================================================

require("config.plugins")

-- ===========================================================================
-- LOAD LSP CONFIGURATIONS
-- ===========================================================================

if nixCats("general.always") or nixCats("lsp") then
	require("config.LSPs")
end
