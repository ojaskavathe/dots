{ ... }:
{
  flake.modules.homeManager.tmux =
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

      demux = pkgs.writeShellApplication {
        name = "demux";
        runtimeInputs = [ pkgs.tmux ];
        text = builtins.replaceStrings [ "@frameconf@" ] [ "${./demux/frame.conf}" ] (
          builtins.readFile ./demux/sidebar.sh
        );
      };

      # events that change what the demux sidebar shows
      demuxRefreshHooks = [
        "session-created"
        "session-closed"
        "session-renamed"
        "client-attached"
        "client-detached"
        "window-linked"
        "window-unlinked"
        "window-renamed"
      ];
      # events that move the client elsewhere: the sidebar follows
      demuxFollowHooks = [
        "client-session-changed"
        "session-window-changed"
      ];
    in
    {
      options = {
        tmux = {
          enable = lib.mkEnableOption "Enable Tmux";
        };
      };

      config = lib.mkIf config.tmux.enable {
        home.packages = [
          tmuxEqualizeNvim
          demux
        ];

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

            # Cycle windows; with the demux sidebar open, route through demux
            # so the sidebar moves in the same batch (no reflow jitter)
            bind -n M-h if-shell -F '#{@demux_open}' 'run-shell -b "${demux}/bin/demux nav prev \"#{window_id}\""' 'previous-window'
            bind -n M-l if-shell -F '#{@demux_open}' 'run-shell -b "${demux}/bin/demux nav next \"#{window_id}\""' 'next-window'

            # demux sidebar (real pane): M-s opens / focuses / closes.
            # client is passed explicitly — implicit switch-client picks the
            # wrong client whenever a second one is attached
            bind -n M-s run-shell -b '${demux}/bin/demux focus "#{pane_id}" "#{client_name}"'
            bind -n M-e run-shell -b '${tmuxEqualizeNvim}/bin/tmux-equalize-nvim'
            bind -n M-g send-keys C-l \; run-shell -b -d 0.05 -C 'clear-history -t "#{pane_id}"'

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

            # directional splits (vim hjkl = side the new pane opens on)
            bind h split-window -hb -c "#{pane_current_path}"
            bind j split-window -v  -c "#{pane_current_path}"
            bind k split-window -vb -c "#{pane_current_path}"
            bind l split-window -h  -c "#{pane_current_path}"

            # kill pane without the y/n confirmation
            bind x kill-pane

            # equally space tmux panes and neovim splits in the current window
            bind e run-shell -b '${tmuxEqualizeNvim}/bin/tmux-equalize-nvim'

            # restore clear with <prefix>C-l
            bind C-l send-keys 'C-l'

            # demux hooks: repaint on changes; follow the client on switches
            ${lib.concatMapStrings (h: ''
              set-hook -g ${h} 'run-shell -b "${demux}/bin/demux refresh"'
            '') demuxRefreshHooks}
            ${lib.concatMapStrings (h: ''
              set-hook -g ${h} 'run-shell -b "${demux}/bin/demux follow"'
            '') demuxFollowHooks}
          '';
        };
      };
    };
}
