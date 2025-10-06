-- local lspconfig = require("lspconfig")
local lspconfig = vim.lsp

local capabilities = require("cmp_nvim_lsp").default_capabilities()
capabilities.textDocument.completion.completionItem.snippetSupport = true

lspconfig.enable('nil_ls')
lspconfig.enable('lua_ls')

lspconfig.enable('ts_ls')
lspconfig.enable('prismals')

lspconfig.config('css_ls', {
  settings = {
    css = {
      validate = true,
      lint = {
	unknownAtRules = "ignore",
      },
    },
  },
  capabilities = capabilities,
})
lspconfig.enable('css_ls')
lspconfig.enable('tailwindcss')
lspconfig.enable('html')

lspconfig.config('clangd', {
  cmd = {
    "clangd",
    "--offset-encoding=utf-16",
  },
})
lspconfig.enable('clangd')
lspconfig.enable('cmake')

-- lspconfig.rust_analyzer.setup{
--   settings = {
--     ['rust-analyzer'] = {
--       checkOnSave =  {
-- 	command = "clippy",
--       }
--     }
--   }
-- }

lspconfig.config('elixirls', {
  cmd = { "elixir-ls" }
})

-- python
lspconfig.config('pyright', {
  settings = {
    pyright = {
      disableOrganizeImports = true, -- using Ruff
    },
    python = {
      analysis = {
	ignore = { "*" }, -- using Ruff
	typeCheckingMode = "off", -- using mypy
      },
    },
  },
})
lspconfig.enable('ruff')

lspconfig.enable('vala_ls')

-- Global mappings.
-- See `:help vim.diagnostic.*` for documentation on any of the below functions
vim.keymap.set("n", "<leader>E", vim.diagnostic.open_float)
vim.keymap.set("n", "[d", vim.diagnostic.goto_prev)
vim.keymap.set("n", "]d", vim.diagnostic.goto_next)
vim.keymap.set("n", "<leader>q", vim.diagnostic.setloclist)

-- Use LspAttach autocommand to only map the following keys
-- after the language server attaches to the current buffer
vim.api.nvim_create_autocmd("LspAttach", {
  group = vim.api.nvim_create_augroup("UserLspConfig", {}),
  callback = function(ev)
    -- Enable completion triggered by <c-x><c-o>
    vim.bo[ev.buf].omnifunc = "v:lua.vim.lsp.omnifunc"

    -- Buffer local mappings.
    -- See `:help vim.lsp.*` for documentation on any of the below functions
    local opts = { buffer = ev.buf }
    vim.keymap.set("n", "gD", vim.lsp.buf.declaration, opts)
    vim.keymap.set("n", "gd", vim.lsp.buf.definition, opts)
    vim.keymap.set("n", "K", vim.lsp.buf.hover, opts)
    vim.keymap.set("n", "gi", vim.lsp.buf.implementation, opts)
    vim.keymap.set("n", "<leader>wa", vim.lsp.buf.add_workspace_folder, opts)
    vim.keymap.set("n", "<leader>wr", vim.lsp.buf.remove_workspace_folder, opts)
    vim.keymap.set("n", "<leader>wl", function()
    	print(vim.inspect(vim.lsp.buf.list_workspace_folders()))
    end, opts)
    vim.keymap.set("n", "<leader>D", vim.lsp.buf.type_definition, opts)
    vim.keymap.set("n", "<leader>rn", vim.lsp.buf.rename, opts)
    vim.keymap.set({ "n", "v" }, "<leader>ca", vim.lsp.buf.code_action, opts)
    vim.keymap.set("n", "gr", vim.lsp.buf.references, opts)
    vim.keymap.set("n", "<leader>f", function()
    	vim.lsp.buf.format({ async = true })
    end, opts)
  end,
})
