{
  pkgs,
  pkgs-stable,
  lib,
  config,
  ...
}:
{
  options = {
    tmux = {
      enable = lib.mkEnableOption "Enable Tmux";
    };
  };

  config = lib.mkIf config.tmux.enable {
    programs.tmux = {
      enable = true;

      baseIndex = 1;
      terminal = "tmux-256color";
      mouse = true;
      prefix = "C-Space";

      escapeTime = 10;

      historyLimit = 100000000;

      plugins = with pkgs; [
        tmuxPlugins.vim-tmux-navigator
        {
          plugin = tmuxPlugins.catppuccin;
          extraConfig = ''
            # colorscheme
            set -g @catppuccin_flavour "mocha"
            set -g @catppuccin_window_status_style "basic"

            # statusbar
            set -gw window-status-separator ""
            set -g @catppuccin_status_left_separator "█"
            set -g @catppuccin_status_right_separator "█"

            set -g status-left "" 

            set -g status-right-length 100

            set -g status-right "#{E:@catppuccin_status_session}"
            set -ag status-right "#{E:@catppuccin_status_uptime}"
            set -agF status-right "#{E:@catppuccin_status_date_time}"
          '';
        }
        {
          # unstable resurrect not building
          plugin = pkgs-stable.tmuxPlugins.resurrect;
          # https://github.com/tmux-plugins/tmux-resurrect/issues/247
          extraConfig = ''
            set -g @resurrect-capture-pane-contents 'on'
            set -g @resurrect-strategy-nvim 'session'
          '';
        }
        {
          plugin = tmuxPlugins.continuum;
          extraConfig = ''
            # restore last save on start (& save every 15 min)
            set -g @continuum-restore 'on'
            set -g @continuum-boot 'off' # https://github.com/tmux-plugins/tmux-continuum/issues/73
          '';
        }
      ];

      extraConfig = ''
        # update status bar every second
        set -g status-interval 15
        set -g status-position top

        # Cycle windows
        bind -n M-h previous-window
        bind -n M-l next-window

        # vi mode
        set-window-option -g mode-keys vi
        # keybinds
        bind-key -T copy-mode-vi v send-keys -X begin-selection
        bind-key -T copy-mode-vi C-v send-keys -X rectangle-toggle
        bind-key -T copy-mode-vi y send-keys -X copy-selection

        # disable 'release mouse to copy'
        unbind-key -T copy-mode-vi MouseDragEnd1Pane

        # open panes in current directory
        bind '"' split-window -h -c "#{pane_current_path}"
        bind % split-window -v -c "#{pane_current_path}"
        bind c new-window -c "#{pane_current_path}"

        # restore clear with <prefix>C-l
        bind C-l send-keys 'C-l'
      '';
    };
  };
}
