-- Main plugin lazy-loading specifications
-- Uses lze for lazy-loading all optional plugins

-- Load all plugin specs
require("lze").load({
	-- Import modular plugin configurations
	{ import = "config.plugins.completion" },
	{ import = "config.plugins.telescope" },
	{ import = "config.plugins.treesitter" },

	-- ==========================================================================
	-- NAVIGATION
	-- ==========================================================================

	-- ==========================================================================
	-- UI PLUGINS
	-- ==========================================================================

	{
		"lualine.nvim",
		for_cat = "general.always",
		event = "DeferredUIEnter",
		after = function(_)
			require("lualine").setup({
				options = {
					icons_enabled = true,
					theme = "auto",
					component_separators = "|",
					section_separators = "",
					globalstatus = true, -- Single statusline at bottom for all splits
				},
			})
		end,
	},

	{
		"trouble.nvim",
		for_cat = "general.extra",
		cmd = "Trouble",
		keys = {
			{ "<leader>xx", "<cmd>Trouble diagnostics toggle<CR>", desc = "Toggle diagnostics" },
			{ "<leader>xq", "<cmd>Trouble qflist toggle<CR>", desc = "Toggle quickfix" },
			{ "<leader>xl", "<cmd>Trouble loclist toggle<CR>", desc = "Toggle loclist" },
			{ "gR", "<cmd>Trouble lsp_references<CR>", desc = "LSP references" },
		},
		after = function(_)
			require("trouble").setup({})
		end,
	},

	{
		"outline.nvim",
		for_cat = "general.extra",
		cmd = "Outline",
		keys = {
			{ "<leader>o", "<cmd>Outline<CR>", desc = "Toggle outline" },
		},
		after = function(_)
			require("outline").setup({})
		end,
	},

	{
		"indent-blankline.nvim",
		for_cat = "general.extra",
		event = "DeferredUIEnter",
		after = function(_)
			require("ibl").setup()
		end,
	},

	{
		"which-key.nvim",
		for_cat = "general.extra",
		event = "DeferredUIEnter",
		keys = {
			{
				"<leader>?",
				function()
					require("which-key").show({ global = false })
				end,
				desc = "Buffer Local Keymaps (which-key)",
			},
		},
		after = function(_)
			require("which-key").setup()
		end,
	},

	-- ==========================================================================
	-- GIT PLUGINS
	-- ==========================================================================

	{
		"gitsigns.nvim",
		for_cat = "general.always",
		event = "DeferredUIEnter",
		after = function(_)
			require("gitsigns").setup({
				current_line_blame = true,
				current_line_blame_opts = {
					delay = 300,
				},
				signs = {
					add = { text = "+" },
					change = { text = "~" },
					delete = { text = "_" },
					topdelete = { text = "‾" },
					changedelete = { text = "~" },
				},
				on_attach = function(bufnr)
					local gs = package.loaded.gitsigns
					local function map(mode, l, r, opts)
						opts = opts or {}
						opts.buffer = bufnr
						vim.keymap.set(mode, l, r, opts)
					end

					-- Navigation
					map("n", "]c", function()
						if vim.wo.diff then
							return "]c"
						end
						vim.schedule(function()
							gs.next_hunk()
						end)
						return "<Ignore>"
					end, { expr = true, desc = "Next hunk" })

					map("n", "[c", function()
						if vim.wo.diff then
							return "[c"
						end
						vim.schedule(function()
							gs.prev_hunk()
						end)
						return "<Ignore>"
					end, { expr = true, desc = "Previous hunk" })

					-- Actions
					map("n", "<leader>gs", gs.stage_hunk, { desc = "Stage hunk" })
					map("n", "<leader>gr", gs.reset_hunk, { desc = "Reset hunk" })
					map("n", "<leader>gp", gs.preview_hunk, { desc = "Preview hunk" })
					map("n", "<leader>gb", function()
						gs.blame_line({ full = true })
					end, { desc = "Blame line" })
				end,
			})
		end,
	},

	{
		"vim-fugitive",
		for_cat = "general.always",
		cmd = { "Git", "G", "Gdiffsplit", "Gvdiffsplit", "Gedit", "Gsplit" },
	},

	-- ==========================================================================
	-- FILE TREE
	-- ==========================================================================

	{
		"nui.nvim",
		for_cat = "general.extra",
		dep_of = { "neo-tree.nvim" },
	},

	{
		"neo-tree.nvim",
		for_cat = "general.extra",
		cmd = "Neotree",
		keys = {
			{ "<leader>t", "<cmd>Neotree toggle<CR>", desc = "Toggle file tree" },
		},
		after = function(_)
			require("neo-tree").setup({
				close_if_last_window = true,
				filesystem = {
					follow_current_file = { enabled = true },
					hijack_netrw_behavior = "disabled", -- oil handles this
					filtered_items = {
						visible = true, -- show dimmed hidden files
						hide_dotfiles = false,
						hide_gitignored = false,
						hide_by_name = { ".git" },
					},
				},
				window = {
					width = 35,
					mappings = {
						["<space>"] = "none", -- don't conflict with leader
					},
				},
			})
		end,
	},

	-- ==========================================================================
	-- NAVIGATION & SEARCH
	-- ==========================================================================

	{
		"oil.nvim",
		for_cat = "general.extra",
		cmd = "Oil",
		event = "VimEnter", -- Load early so it handles `nvim .` (netrw is disabled)
		keys = {
			{ "-", "<cmd>Oil<CR>", desc = "Open parent directory" },
		},
		after = function(_)
			require("oil").setup({
				default_file_explorer = true,
				view_options = {
					show_hidden = true,
				},
			})
		end,
	},

	{
		"grug-far.nvim",
		for_cat = "general.extra",
		cmd = "GrugFar",
		keys = {
			{ "<leader>si", desc = "Search in range" },
		},
		after = function(_)
			require("grug-far").setup({})
			vim.keymap.set({ "n", "x" }, "<leader>si", function()
				require("grug-far").open({ visualSelectionUsage = "operate-within-range" })
			end, { desc = "Search within range" })
		end,
	},

	-- ==========================================================================
	-- MARKDOWN
	-- ==========================================================================

	{
		"render-markdown.nvim",
		for_cat = "general.extra",
		ft = { "markdown" },
		keys = {
			{ "<leader>mr", "<cmd>RenderMarkdown toggle<CR>", desc = "Toggle markdown render" },
		},
		after = function(_)
			require("render-markdown").setup({})
		end,
	},

	-- ==========================================================================
	-- UNDO
	-- ==========================================================================

	{
		"undotree",
		for_cat = "lsp",
		cmd = "UndotreeToggle",
		keys = {
			{ "<leader>u", "<cmd>UndotreeToggle<CR>", desc = "Toggle undotree" },
		},
	},

	-- ==========================================================================
	-- AI COMPLETIONS
	-- ==========================================================================

	{
		"supermaven-nvim",
		for_cat = "general.extra",
		event = "InsertEnter",
		after = function(_)
			require("supermaven-nvim").setup({})
		end,
	},

	-- ==========================================================================
	-- EDITING ENHANCEMENTS
	-- ==========================================================================

	{
		"nvim-surround",
		for_cat = "general.always",
		event = "DeferredUIEnter",
		after = function(_)
			require("nvim-surround").setup()
		end,
	},

	{
		"comment.nvim",
		for_cat = "general.extra",
		keys = {
			{ "gc", mode = { "n", "v" }, desc = "Toggle comment" },
			{ "gb", mode = { "n", "v" }, desc = "Toggle block comment" },
		},
		after = function(_)
			require("ts_context_commentstring").setup({ enable_autocmd = false })
			require("Comment").setup({
				pre_hook = require("ts_context_commentstring.integrations.comment_nvim").create_pre_hook(),
			})
		end,
	},

	{
		"nvim-autopairs",
		for_cat = "general.extra",
		event = "InsertEnter",
		after = function(_)
			require("nvim-autopairs").setup()
		end,
	},

	{
		"nvim-treesitter-textsubjects",
		for_cat = "general.extra",
		event = "DeferredUIEnter",
		after = function(_)
			require("nvim-treesitter.configs").setup({
				textsubjects = {
					enable = true,
					prev_selection = ",",
					keymaps = {
						["."] = "textsubjects-smart",
						[";"] = "textsubjects-container-outer",
						["i;"] = { "textsubjects-container-inner", desc = "Select inside containers" },
					},
				},
			})
		end,
	},
})
