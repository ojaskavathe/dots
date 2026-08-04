-- Main configuration entry point
-- This file registers lze handlers and loads all plugin configurations

-- ===========================================================================
-- NIX BRIDGE
-- ===========================================================================
-- nix-wrapper-modules exposes settings through an info plugin
-- (vim.g.nix_info_plugin_name). This shim keeps the old nixCats("cat") call
-- sites and the `for_cat` lze handler working unchanged. There's a single baked
-- build with every category on, so category lookups are always true; only
-- `colorscheme` returns a real value from nix.
if not _G.nixCats then
	local info = require(vim.g.nix_info_plugin_name)
	_G.nixCats = function(key)
		if key == "colorscheme" then
			return info("catppuccin", "settings", "colorscheme")
		end
		return true
	end
end

-- ===========================================================================
-- OPTIONS AND KEYMAPS
-- ===========================================================================
require("config.opts_and_keys")
require("config.tmux")

-- ===========================================================================
-- COLORSCHEME
-- ===========================================================================
local colorschemeName = nixCats("colorscheme") or "catppuccin"
vim.cmd.colorscheme(colorschemeName)

-- ===========================================================================
-- TREESITTER
-- ===========================================================================
vim.api.nvim_create_autocmd("FileType", {
	callback = function(args)
		pcall(vim.treesitter.start, args.buf)
	end,
})

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
