-- Configuration for diagnostics
-- Note: sign_define() is deprecated, using modern diagnostic signs instead

local config = {
  signs = {
    text = {
      [vim.diagnostic.severity.ERROR] = '',
      [vim.diagnostic.severity.WARN] = '',
      [vim.diagnostic.severity.HINT] = '',
      [vim.diagnostic.severity.INFO] = '',
    },
  },
  update_in_insert = false,
  underline = true,
  severity_sort = true,
  float = {
    focusable = true,
    style = 'minimal',
    border = 'single',
    source = 'always',
    header = 'Diagnostic',
    prefix = '',
  },
}

vim.diagnostic.config(config)

vim.keymap.set('n', '<leader>d', vim.diagnostic.open_float)
