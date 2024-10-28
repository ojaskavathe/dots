-- split movement
vim.keymap.set('', '<C-h>', '<C-w>h')
vim.keymap.set('', '<C-j>', '<C-w>j')
vim.keymap.set('', '<C-k>', '<C-w>k')
vim.keymap.set('', '<C-l>', '<C-w>l')

-- black hole register instead of cut
vim.keymap.set('', '<leader>d', '"_d')

-- yank to clipboard
vim.keymap.set({ 'n', 'v' }, '<leader>y', '"+y')

-- paste from clipboard
vim.keymap.set('n', '<leader>p', '"+p')
vim.keymap.set('n', '<leader>P', '"+P')

-- select all
vim.keymap.set('n', '<leader>sa', 'ggVG')
