local null_ls = require("null-ls")

null_ls.setup({
	sources = {
		-- nix
		null_ls.builtins.formatting.nixfmt,

		-- lua
		null_ls.builtins.formatting.stylua,

		-- js, ts
		null_ls.builtins.formatting.prettier,

		-- python
		null_ls.builtins.diagnostics.mypy.with({
			extra_args = function()
				local virtual = os.getenv("VIRTUAL_ENV") or os.getenv("CONDA_PREFIX") or "/usr"
				return { "--python-executable", virtual .. "/bin/python3" }
			end,
		}),

		-- clang
		null_ls.builtins.formatting.clang_format,
	},
})

vim.keymap.set("n", "<leader>gf", vim.lsp.buf.format, {})
