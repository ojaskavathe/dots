-- Completion configuration using blink.cmp
-- Based on nixCats example implementation

local load_w_after = function(name)
  vim.cmd.packadd(name)
  vim.cmd.packadd(name .. '/after')
end

return {
  {
    "cmp-cmdline",
    for_cat = "general.blink",
    on_plugin = { "blink.cmp" },
    load = load_w_after,
  },
  {
    "blink.compat",
    for_cat = "general.blink",
    dep_of = { "cmp-cmdline" },
  },
  {
    "luasnip",
    for_cat = "general.blink",
    dep_of = { "blink.cmp" },
    after = function(_)
      local luasnip = require('luasnip')
      require('luasnip.loaders.from_vscode').lazy_load()
      luasnip.config.setup({})

      -- Keymap for changing snippet choices
      vim.keymap.set({ "i", "s" }, "<M-n>", function()
        if luasnip.choice_active() then
          luasnip.change_choice(1)
        end
      end, { desc = "Next snippet choice" })
    end,
  },
  {
    "blink.cmp",
    for_cat = "general.blink",
    event = "DeferredUIEnter",
    after = function(_)
      require("blink.cmp").setup({
        -- Use 'default' preset for familiar keybindings (C-y to accept)
        keymap = {
          preset = 'enter',
          ['<C-n>'] = { 'select_next', 'show', 'fallback' },
          ['<C-p>'] = { 'select_prev', 'fallback' },
        },

        -- Cmdline completion
        cmdline = {
          enabled = true,
          keymap = {
            preset = 'super-tab',
            ['<C-n>'] = { 'select_next', 'show', 'fallback' },
            ['<C-p>'] = { 'select_prev', 'fallback' },
          },
          completion = {
            menu = {
              auto_show = true,
            },
          },
          sources = function()
            local type = vim.fn.getcmdtype()
            -- Search forward and backward
            if type == '/' or type == '?' then return { 'buffer' } end
            -- Commands
            if type == ':' or type == '@' then return { 'cmdline', 'cmp_cmdline' } end
            return {}
          end,
        },

        -- Fuzzy matching
        fuzzy = {
          sorts = {
            'exact',
            'score',
            'sort_text',
          },
        },

        -- Signature help
        signature = {
          enabled = true,
          window = {
            show_documentation = true,
          },
        },

        -- Completion menu
        completion = {
          menu = {
            draw = {
              treesitter = { 'lsp' },
            },
          },
          documentation = {
            auto_show = true,
          },
        },

        -- Snippets configuration
        snippets = {
          preset = 'luasnip',
          active = function(filter)
            local snippet = require("luasnip")
            local blink = require("blink.cmp")
            if snippet.in_snippet() and not blink.is_visible() then
              return true
            else
              if not snippet.in_snippet() and vim.fn.mode() == "n" then
                snippet.unlink_current()
              end
              return false
            end
          end,
        },

        -- Sources configuration
        sources = {
          default = { 'lsp', 'path', 'snippets', 'buffer' },
          providers = {
            path = {
              score_offset = 50,
            },
            lsp = {
              score_offset = 40,
            },
            snippets = {
              score_offset = 40,
            },
            cmp_cmdline = {
              name = 'cmp_cmdline',
              module = 'blink.compat.source',
              score_offset = -100,
              opts = {
                cmp_name = 'cmdline',
              },
            },
          },
        },
      })
    end,
  },
}
