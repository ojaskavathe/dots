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

      # demux daemon + sidebar TUI, from its own flake (thoughts/
      # demux-architecture.md records the design history).
      inherit (inputs.demux.packages.${pkgs.stdenv.hostPlatform.system}) demuxd demux;

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
          demuxd
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
            {
              plugin = tmuxPlugins.vim-tmux-navigator;
              extraConfig = ''
                # treat the demux sidebar like a vim split: navigator keys are
                # sent INTO it (the TUI maps C-l to enter, C-j/C-k to j/k)
                # instead of select-pane escaping out of a zoomed billboard
                set -g @vim_navigator_pattern '(\S+/)?g?\.?(view|l?n?vim?x?|fzf|demuxd)(diff)?(-wrapped)?'
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

                # agent state counts from demuxd (!blocked ✓done ✻working);
                # empty when quiet — a plain option reference, zero-cost render
                set -g status-right "#{E:@demux_agents} "
                set -ag status-right "#{E:@catppuccin_status_session}"
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
              # tmux server" — but demuxd keeps a persistent control-mode
              # client and the rigs spawn side servers, so the heuristic is
              # permanently true here and snapshots stopped (2026-08-11,
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

            # kitty supports synchronized output (DECSET 2026) but tmux does
            # not auto-detect it: without the sync feature EVERY redraw —
            # switch-client, zoom, swap-pane, sidebar paints — goes out
            # unwrapped, and kitty renders whatever half-frame has arrived
            # when its frame timer fires (random flicker on transitions).
            set -as terminal-features 'xterm-kitty:sync'

            # update status bar every second
            set -g status-interval 15
            set -g status-position top

            # vim-tmux-navigator's is_vim shells out to `ps -o state=`, and
            # macOS 26.5 hides the process state field behind an entitlement
            # — the blank field breaks its regex, so C-hjkl silently degraded
            # to raw select-pane everywhere: escaping demux billboards,
            # ignoring nvim splits. Rebind over the plugin with tmux's own
            # format regex on pane_current_command — no ps, no subprocess
            # per keypress.
            bind -n C-h if -F '#{m/r:^(view|l?n?vim?x?|fzf|demuxd)(diff)?(-wrapped)?$,#{pane_current_command}}' 'send-keys C-h' 'select-pane -L'
            bind -n C-j if -F '#{m/r:^(view|l?n?vim?x?|fzf|demuxd)(diff)?(-wrapped)?$,#{pane_current_command}}' 'send-keys C-j' 'select-pane -D'
            bind -n C-k if -F '#{m/r:^(view|l?n?vim?x?|fzf|demuxd)(diff)?(-wrapped)?$,#{pane_current_command}}' 'send-keys C-k' 'select-pane -U'
            bind -n C-l if -F '#{m/r:^(view|l?n?vim?x?|fzf|demuxd)(diff)?(-wrapped)?$,#{pane_current_command}}' 'send-keys C-l' 'select-pane -R'
            bind -n 'C-\' if -F '#{m/r:^(view|l?n?vim?x?|fzf|demuxd)(diff)?(-wrapped)?$,#{pane_current_command}}' 'send-keys C-\\' 'select-pane -l'

            # window cycling: native normally; while the demux sidebar is
            # docked (@demux_docked set on the session) the switch routes
            # through the daemon so the sidebar arrives WITH the window —
            # never a frame of the target without it
            bind -n M-h if-shell -F "#{@demux_docked}" {run-shell -b '${demuxd}/bin/demuxd nav prev "#{client_name}"'} {previous-window}
            bind -n M-l if-shell -F "#{@demux_docked}" {run-shell -b '${demuxd}/bin/demuxd nav next "#{client_name}"'} {next-window}

            # demux sidebar: M-s docks the list as a real pane on the left
            # (herdr-style; main area stays your live panes), M-s again
            # undocks and restores the exact layout. client passed explicitly
            # — implicit targeting picks the wrong client whenever a second
            # one is attached (the demux control client always is).
            # (M-s previously toggled the native session tree — that stays
            # reachable on prefix+s)
            bind -n M-s run-shell -b '${demuxd}/bin/demuxd toggle "#{client_name}"'

            # M-g: the agent switcher — like M-s but pinned on the
            # top-attention agent; rapid taps cycle through agents.
            # (M-a kept too, but aerospace eats alt-a for its accordion
            # layout, so it only fires if that binding goes away.)
            bind -n M-g run-shell -b '${demuxd}/bin/demuxd agents "#{client_name}"'
            bind -n M-a run-shell -b '${demuxd}/bin/demuxd agents "#{client_name}"'
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
          '';
        };
      };
    };
}
