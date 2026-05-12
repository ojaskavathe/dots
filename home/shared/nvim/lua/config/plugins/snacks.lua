-- snacks.nvim: picker + gitbrowse
-- Replaces telescope.

return {
	{
		"snacks.nvim",
		for_cat = "general.snacks",
		event = "VimEnter", -- load early so LSP on_attach can reference Snacks.picker
		keys = {
			{ "<leader>ff", desc = "Find files" },
			{ "<leader>fg", desc = "Find by grep" },
			{ "<leader>fb", desc = "Find buffers" },
			{ "<leader>fs", desc = "Find symbols/icons" },
			{ "<leader>gs", desc = "Git status" },
			{ "<leader>go", desc = "Git: open in browser" },
		},
		after = function(_)
			-- Two-tone borderless picker: search panel + preview share the
			-- same window, separated only by a bg color difference.
			local function set_picker_hl()
				vim.api.nvim_set_hl(0, "SnacksPickerSearch", { bg = "#181825" }) -- mantle
				vim.api.nvim_set_hl(0, "SnacksPickerPreview", { bg = "#1e1e2e" }) -- base
			end
			set_picker_hl()
			vim.api.nvim_create_autocmd("ColorScheme", { callback = set_picker_hl })

			local search_wo = {
				winhighlight = "Normal:SnacksPickerSearch,NormalFloat:SnacksPickerSearch,FloatBorder:SnacksPickerSearch",
			}
			local preview_wo = {
				winhighlight = "Normal:SnacksPickerPreview,NormalFloat:SnacksPickerPreview,FloatBorder:SnacksPickerPreview",
			}

			require("snacks").setup({
				picker = {
					ui_select = true,
					sources = {
						files = {
							hidden = true,
							exclude = { "node_modules", ".git", ".venv" },
						},
						buffers = {
							sort_lastused = true,
						},
					},
					win = {
						input = {
							wo = search_wo,
							keys = {
								-- alt-h conflicts with aerospace window focus
								["<a-h>"] = false,
								["<a-.>"] = { "toggle_hidden", mode = { "n", "i" } },
								-- alt-i is a macOS dead key (option-i → ˆ)
								["<a-i>"] = false,
								["<a-,>"] = { "toggle_ignored", mode = { "n", "i" } },
							},
						},
						list = { wo = search_wo },
						preview = { wo = preview_wo },
					},
					layout = {
						layout = {
							box = "horizontal",
							width = 0.8,
							min_width = 120,
							height = 0.8,
							{
								box = "vertical",
								border = "none",
								{
									win = "input",
									height = 1,
									-- top-only "border" of spaces — gives the title a place to render
									-- without drawing an actual line. Inherits the input's bg via FloatBorder hl.
									border = { "", " ", "", "", "", "", "", "" },
									title = "{title} {live} {flags}",
									title_pos = "left",
								},
								{ win = "list", border = "none" },
							},
							{ win = "preview", border = "none", width = 0.5 },
						},
					},
				},
				gitbrowse = {
					enabled = true,
				},
			})

			vim.keymap.set("n", "<leader>ff", function()
				Snacks.picker.files()
			end, { desc = "Find files" })
			vim.keymap.set("n", "<leader>fg", function()
				Snacks.picker.grep()
			end, { desc = "Find by grep" })
			vim.keymap.set("x", "<leader>fg", function()
				Snacks.picker.grep_word()
			end, { desc = "Find by grep (selection)" })
			vim.keymap.set("n", "<leader>fb", function()
				Snacks.picker.buffers()
			end, { desc = "Find buffers" })
			vim.keymap.set("n", "<leader>fs", function()
				Snacks.picker.icons()
			end, { desc = "Find symbols/icons" })
			vim.keymap.set("n", "<leader>gs", function()
				Snacks.picker.git_status()
			end, { desc = "Git status" })
			vim.keymap.set({ "n", "x" }, "<leader>go", function()
				Snacks.gitbrowse.open()
			end, { desc = "Git: open in browser" })
		end,
	},
}
