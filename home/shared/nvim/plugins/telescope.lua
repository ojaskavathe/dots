require("telescope").setup({
	pickers = {
		find_files = {
			hidden = true,
			file_ignore_patterns =  { 'node_modules', '.git', '.venv' },
		},
		buffers = {
			show_all_buffers = true,
			sort_mru = true,
			mappings = {
				i = {
					-- CTRL-D in buffer select
					["<c-d>"] = "delete_buffer",
				},
			},
		},
	},
	extensions = {
		fzf = {
			fuzzy = true,             -- false will only do exact matching
			override_generic_sorter = true, -- override the generic sorter
			override_file_sorter = true, -- override the file sorter
			case_mode = "smart_case", -- or "ignore_case" or "respect_case"
		},
		["ui-select"] = {
			require("telescope.themes").get_dropdown({}),
		},
	},
})

require("telescope").load_extension("fzf")
require("telescope").load_extension("ui-select")

local builtin = require("telescope.builtin")

vim.keymap.set("n", "<leader>ff", builtin.find_files, { desc = "[F]ind [F]iles" })
vim.keymap.set("n", "<leader>fg", builtin.live_grep, { desc = "[F]ind by [G]rep" })
vim.keymap.set("v", "<leader>fg", builtin.grep_string, { desc = "[F]ind by [G]rep" })
vim.keymap.set("n", "<leader>gs", builtin.git_status, { desc = "[G]it [S]tatus" })
vim.keymap.set("n", "<leader>fb", builtin.buffers, { desc = "[F]ind [B]uffers" })
vim.keymap.set("n", "<leader>fs", builtin.symbols, { desc = "[F]ind [S]ymbols" })
