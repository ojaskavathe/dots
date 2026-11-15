{ inputs, ... }:
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

          historyLimit = 100000;

          plugins = with pkgs; [
            {
              plugin = tmuxPlugins.vim-tmux-navigator;
              extraConfig = ''
                set -g @vim_navigator_pattern '(\S+/)?g?\.?(view|l?n?vim?x?|fzf)(diff)?(-wrapped)?'
              '';
            }
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
              # continuum silently disables autosave when it sees "another
              # tmux server" — the rigs spawn side servers, so the heuristic
              # is permanently true here and snapshots stopped (2026-08-11,
              # discovered after a tmux crash left only a 9-day-old save).
              # The guard exists to stop two full servers rotating the same
              # save dir; accepted trade-off — saves beat no saves.
              plugin = tmuxPlugins.continuum.overrideAttrs (old: {
                postInstall = (old.postInstall or "") + ''
                  sed -i 's/if ! another_tmux_server_running; then/if true; then/' \
                    $out/share/tmux-plugins/continuum/continuum.tmux
                '';
              });
              extraConfig = ''
                # restore last save on start (& save every 15 min)
                set -g @continuum-restore 'on'
                set -g @continuum-boot 'off' # https://github.com/tmux-plugins/tmux-continuum/issues/73
              '';
            }
          ];

          extraConfig = ''
            set -g set-clipboard on

            # hm-session-vars.sh only loads once per environment; drop its guards
            # so new panes pick up the current session vars instead of the ones
            # this server inherited when it started.
            set-environment -gu __HM_SESS_VARS_SOURCED
            set-environment -gu __HM_ZSH_SESS_VARS_SOURCED

            # Closing a session's last window destroys the session, and tmux's
            # default then throws the client all the way out to a bare shell
            # even with other sessions still running. `off` carries it to the
            # most recently used one instead. Nothing to do with any plugin —
            # verified identical with the sidebar entirely uninvolved.
            set -g detach-on-destroy off

            # kitty supports synchronized output (DECSET 2026) but tmux does
            # not auto-detect it: without the sync feature EVERY redraw —
            # switch-client, zoom, swap-pane, sidebar paints — goes out
            # unwrapped, and kitty renders whatever half-frame has arrived
            # when its frame timer fires (random flicker on transitions).
            set -as terminal-features 'xterm-kitty:sync'

            # Pass OSC 8 hyperlinks through to kitty. Without this tmux strips
            # them, and kitty falls back to detecting URLs in plain text. That
            # breaks on wrapped links: tmux redraws every row with its own
            # cursor move, so kitty sees each row as a separate line and only
            # the first row is clickable. With OSC 8, every cell carries the
            # link, so shift-click works on any row.
            set -as terminal-features 'xterm-kitty:hyperlinks'

            # update status bar every second
            set -g status-interval 15
            set -g status-position top

            # prefix-: must REPLACE the status line, not draw over it.
            #
            # `fill=` is the whole trick. status_prompt_redraw copies the real
            # bar into the prompt screen and format_draw blanks only what the
            # fill covers — `bg=` does not do it. tmux's own default is
            # `bg=yellow,fg=black,fill=yellow`, and that fill is why a stock
            # tmux looks like it replaces the bar; catppuccin sets bg=default
            # and no fill, so the old status text stayed visible underneath.
            #
            # No align= at all, which means left. align does DOUBLE duty here:
            # status_prompt_area reads it to place the prompt's area, and
            # message-format embeds the same style so format_draw reads it again
            # to place the text inside that area. align=centre therefore centred
            # the ":" in a full-width area rather than centring anything useful.
            #
            # Both options, because they split the job: message-command-style
            # paints prefix-: (PROMPT_COMMAND), message-style paints every other
            # message.
            set -g message-style "fg=#94e2d5,bg=#181825,fill=#181825"
            set -g message-command-style "fg=#94e2d5,bg=#181825,fill=#181825"

            # nvim's FocusGained/FocusLost need the terminal's focus reports.
            set -g focus-events on

            # vim-tmux-navigator's is_vim shells out to `ps -o state=`, and
            # macOS 26.5 hides the process state field behind an entitlement
            # — the blank field breaks its regex, so C-hjkl silently degraded
            # to raw select-pane everywhere, ignoring nvim splits. Rebind over the plugin with tmux's own
            # format regex on pane_current_command — no ps, no subprocess
            # per keypress.
            bind -n C-h if -F '#{m/r:^(view|l?n?vim?x?|fzf)(diff)?(-wrapped)?$,#{pane_current_command}}' 'send-keys C-h' 'select-pane -L'
            bind -n C-j if -F '#{m/r:^(view|l?n?vim?x?|fzf)(diff)?(-wrapped)?$,#{pane_current_command}}' 'send-keys C-j' 'select-pane -D'
            bind -n C-k if -F '#{m/r:^(view|l?n?vim?x?|fzf)(diff)?(-wrapped)?$,#{pane_current_command}}' 'send-keys C-k' 'select-pane -U'
            bind -n C-l if -F '#{m/r:^(view|l?n?vim?x?|fzf)(diff)?(-wrapped)?$,#{pane_current_command}}' 'send-keys C-l' 'select-pane -R'
            bind -n 'C-\' if -F '#{m/r:^(view|l?n?vim?x?|fzf)(diff)?(-wrapped)?$,#{pane_current_command}}' 'send-keys C-\\' 'select-pane -l'

            bind -n M-h previous-window
            bind -n M-l next-window

            # equalize tmux panes and neovim splits
            bind -n M-e run-shell -b '${tmuxEqualizeNvim}/bin/tmux-equalize-nvim'
            bind -n M-g send-keys C-l \; run-shell -b -d 0.05 -C 'clear-history -t "#{pane_id}"'

            # vi mode
            set-window-option -g mode-keys vi
            # keybinds
            bind-key -T copy-mode-vi v send-keys -X begin-selection
            bind-key -T copy-mode-vi C-v send-keys -X rectangle-toggle
            bind-key -T copy-mode-vi y send-keys -X copy-selection

            # search: prefix / drops into copy-mode with an incremental search
            # up the history. Inside copy-mode / and ? are incremental too, and
            # / searches up since copy-mode starts at the bottom.
            bind / copy-mode \; command-prompt -i -T search -p "(search up)" { send-keys -X search-backward-incremental "%%" }
            bind-key -T copy-mode-vi / command-prompt -i -T search -p "(search up)" { send-keys -X search-backward-incremental "%%" }
            bind-key -T copy-mode-vi ? command-prompt -i -T search -p "(search down)" { send-keys -X search-forward-incremental "%%" }

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
          '';
        };
      };
    };
}
