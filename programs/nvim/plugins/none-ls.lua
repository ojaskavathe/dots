local null_ls = require("null-ls")

null_ls.setup({
	sources = {
		-- nix
		null_ls.builtins.formatting.nixfmt,

		-- lua
		null_ls.builtins.formatting.stylua,

		-- js, ts
		null_ls.builtins.formatting.prettier,

		-- clang
		null_ls.builtins.formatting.clang_format,
	},
})

vim.keymap.set("n", "<leader>gf", vim.lsp.buf.format, {})
