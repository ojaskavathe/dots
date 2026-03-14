-- Telescope configuration
-- Fuzzy finder and picker

return {
  {
    "telescope.nvim",
    for_cat = "general.telescope",
    cmd = "Telescope",
    keys = {
      { "<leader>ff", desc = "Find files" },
      { "<leader>fg", desc = "Find by grep", mode = { "n", "v" } },
      { "<leader>gs", desc = "Git status" },
      { "<leader>fb", desc = "Find buffers" },
      { "<leader>fs", desc = "Find symbols" },
    },
    after = function(_)
      require("telescope").setup({
        pickers = {
          find_files = {
            hidden = true,
            file_ignore_patterns = { 'node_modules', '.git/', '.venv' },
          },
          buffers = {
            show_all_buffers = true,
            sort_mru = true,
            mappings = {
              i = {
                ["<c-d>"] = "delete_buffer",
              },
            },
          },
        },
        extensions = {
          fzf = {
            fuzzy = true,
            override_generic_sorter = true,
            override_file_sorter = true,
            case_mode = "smart_case",
          },
          ["ui-select"] = {
            require("telescope.themes").get_dropdown({}),
          },
        },
      })

      -- Load extensions
      pcall(require('telescope').load_extension, 'fzf')
      pcall(require('telescope').load_extension, 'ui-select')

      -- Set up keymaps
      local builtin = require("telescope.builtin")
      vim.keymap.set("n", "<leader>ff", builtin.find_files, { desc = "Find files" })
      vim.keymap.set("n", "<leader>fg", builtin.live_grep, { desc = "Find by grep" })
      vim.keymap.set("v", "<leader>fg", builtin.grep_string, { desc = "Find by grep" })
      vim.keymap.set("n", "<leader>gs", builtin.git_status, { desc = "Git status" })
      vim.keymap.set("n", "<leader>fb", builtin.buffers, { desc = "Find buffers" })
      vim.keymap.set("n", "<leader>fs", builtin.symbols, { desc = "Find symbols" })
    end,
  },
}
