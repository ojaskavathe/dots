{
  pkgs,
  pkgs-stable,
  lib,
  config,
  ...
}:
let
  tmuxEqualizeNvim = pkgs.buildGoModule {
    pname = "tmux-equalize-nvim";
    version = "0.1.0";

    src = ./tmux-equalize-nvim;
    vendorHash = "sha256-/Bl4G5STa5lnNntZnMmt+BfES+N7ZYAwC9tzpuqUKcc=";

    ldflags = [
      "-X"
      "main.tmuxPath=${pkgs.tmux}/bin/tmux"
    ];
  };
in
{
  options = {
    tmux = {
      enable = lib.mkEnableOption "Enable Tmux";
    };
  };

  config = lib.mkIf config.tmux.enable {
    home.packages = [ tmuxEqualizeNvim ];

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

            # windows
            set -g @catppuccin_window_status_style "basic"
            set -g @catppuccin_window_text " #{b:pane_current_path}"
            set -g @catppuccin_window_current_text " #{b:pane_current_path}"
            set -gw window-status-separator ""

            # statusbar
            set -g @catppuccin_status_left_separator "█"
            set -g @catppuccin_status_right_separator "█"
            set -g status-left "" 

            # status bar updates every 15s by default**, change to 1s here 
            # (this step is optional - a lower latency might have negative battery/cpu usage impacts)
            set -g status-interval 1

            set -g status-right-length 100

            set -g status-right "#{E:@catppuccin_status_session}"
            set -ag status-right "#{E:@catppuccin_status_uptime}"
            set -ag status-right "#{E:@catppuccin_status_date_time}"
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
        set -g set-clipboard on

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

        # equally space tmux panes and neovim splits in the current window
        bind e run-shell -b '${tmuxEqualizeNvim}/bin/tmux-equalize-nvim'

        # restore clear with <prefix>C-l
        bind C-l send-keys 'C-l'
      '';
    };
  };
}
