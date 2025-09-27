require('grug-far').setup({
	-- options, see Configuration section below
	-- there are no required options atm
});

vim.keymap.set({ 'n', 'x' }, '<leader>si', function()
  require('grug-far').open({ visualSelectionUsage = 'operate-within-range' })
end, { desc = 'grug-far: Search within range' })
