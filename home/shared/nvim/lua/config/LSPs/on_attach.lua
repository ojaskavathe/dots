-- Shared LSP on_attach function
-- This function is called when an LSP attaches to a buffer

return function(_, bufnr)
	-- Helper function for setting LSP keymaps
	local nmap = function(keys, func, desc)
		if desc then
			desc = "LSP: " .. desc
		end
		vim.keymap.set("n", keys, func, { buffer = bufnr, desc = desc })
	end

	-- Enable completion
	vim.bo[bufnr].omnifunc = "v:lua.vim.lsp.omnifunc"

	-- Core LSP keymaps
	nmap("gD", vim.lsp.buf.declaration, "Goto declaration")
	nmap("gd", vim.lsp.buf.definition, "Goto definition")
	nmap("K", vim.lsp.buf.hover, "Hover documentation")
	-- gi is kept as built-in (go to last insert position)
	-- gI is mapped to LSP implementations via telescope below

	-- Workspace management
	nmap("<leader>wa", vim.lsp.buf.add_workspace_folder, "Add workspace folder")
	nmap("<leader>wr", vim.lsp.buf.remove_workspace_folder, "Remove workspace folder")
	nmap("<leader>wl", function()
		print(vim.inspect(vim.lsp.buf.list_workspace_folders()))
	end, "List workspace folders")

	-- Code actions
	nmap("<leader>D", vim.lsp.buf.type_definition, "Type definition")
	nmap("<leader>rn", vim.lsp.buf.rename, "Rename")
	vim.keymap.set({ "n", "v" }, "<leader>ca", vim.lsp.buf.code_action, { buffer = bufnr, desc = "LSP: Code action" })

	-- References (use telescope if available)
	if nixCats and nixCats("general.telescope") then
		nmap("gr", function()
			require("telescope.builtin").lsp_references()
		end, "Goto references")
		nmap("gI", function()
			require("telescope.builtin").lsp_implementations()
		end, "Goto implementations")
		nmap("<leader>ds", function()
			require("telescope.builtin").lsp_document_symbols()
		end, "Document symbols")
		nmap("<leader>ws", function()
			require("telescope.builtin").lsp_dynamic_workspace_symbols()
		end, "Workspace symbols")
	else
		nmap("gr", vim.lsp.buf.references, "Goto references")
	end

	-- Formatting (use conform if available, fallback to LSP)
	nmap("<leader>f", function()
		require("conform").format({ bufnr = bufnr, lsp_fallback = true })
	end, "Format buffer")
end
