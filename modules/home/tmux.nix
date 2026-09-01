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

      # winch daemon + sidebar TUI, from its own flake (thoughts/
      # winch-architecture.md records the design history).
      inherit (inputs.winch.packages.${pkgs.stdenv.hostPlatform.system}) winch;

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
          winch
        ];

        # Re-register winch's notification bundle on every switch.
        #
        # macOS delivers a notification on behalf of an APP, and
        # UNUserNotificationCenter only talks to apps LaunchServices knows
        # about — which it learns by scanning /Applications and
        # ~/Applications, never the nix store. So the bundle has to be
        # registered explicitly, and because every rebuild moves its store
        # path, a registration made once goes stale the next time you switch:
        # notifications then fail SILENTLY, with no error anywhere.
        #
        # Idempotent and cheap, so it just runs. stdout is dropped because
        # the command's success message is written for a human typing it;
        # a failure still surfaces.
        home.activation = lib.mkIf pkgs.stdenv.hostPlatform.isDarwin {
          winchNotifyInstall = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
            run ${winch}/bin/winch notify-install > /dev/null || \
              echo "winch: notify-install failed; desktop notifications may not work"
          '';
        };

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
                # treat the winch sidebar like a vim split: navigator keys are
                # sent INTO it (the TUI maps C-l to enter, C-j/C-k to j/k)
                # instead of select-pane escaping out of a zoomed billboard
                set -g @vim_navigator_pattern '(\S+/)?g?\.?(view|l?n?vim?x?|fzf|winch)(diff)?(-wrapped)?'
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

                # agent state counts from winch (!blocked ✓done ✻working);
                # empty when quiet — a plain option reference, zero-cost render
                set -g status-right "#{E:@winch_agents} "
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
              # tmux server" — but winch keeps a persistent control-mode
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

            # Closing a session's last window destroys the session, and tmux's
            # default then throws the client all the way out to a bare shell
            # even with other sessions still running. `off` carries it to the
            # most recently used one instead. Nothing to do with winch —
            # verified identical with the sidebar entirely uninvolved.
            set -g detach-on-destroy off

            # kitty supports synchronized output (DECSET 2026) but tmux does
            # not auto-detect it: without the sync feature EVERY redraw —
            # switch-client, zoom, swap-pane, sidebar paints — goes out
            # unwrapped, and kitty renders whatever half-frame has arrived
            # when its frame timer fires (random flicker on transitions).
            set -as terminal-features 'xterm-kitty:sync'

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

            # winch notifications go through the OS, not the terminal.
            #
            # The terminal route is winch's default and the portable one — an
            # OSC to the client's tty, which follows you over ssh. It cannot
            # work here, because kitty never registers with macOS
            # notifications: it does not appear in System Settings ->
            # Notifications at all, so there is no permission to grant and
            # every dialect fails silently. Not the ad-hoc nix signature
            # (terminal-notifier has the same one and registers fine), not the
            # process tree, not the dialect — each ruled out by comparison.
            # osascript from this same tmux tree lands every time.
            # A Linux box should leave this unset.
            set -g @winch-notify-via system

            # Notify on turn-end too, not just when an agent is blocked.
            # winch defaults to blocked-only because done fires once per turn
            # per agent, which is a lot if you are not waiting on them.
            set -g @winch-notify all

            # winch suppresses a notification for the agent in the window you
            # are already on — but "you can see it" is only true if you are
            # LOOKING. tmux tracks whether the terminal has OS focus
            # (CLIENT_FOCUSED, cleared on the terminal's focus-out report),
            # and only asks the terminal to report focus at all when this is
            # on. Without it every client reads as focused forever and the
            # rule stays as coarse as it was. nvim's FocusGained/FocusLost
            # want this too.
            set -g focus-events on

            # vim-tmux-navigator's is_vim shells out to `ps -o state=`, and
            # macOS 26.5 hides the process state field behind an entitlement
            # — the blank field breaks its regex, so C-hjkl silently degraded
            # to raw select-pane everywhere: escaping winch billboards,
            # ignoring nvim splits. Rebind over the plugin with tmux's own
            # format regex on pane_current_command — no ps, no subprocess
            # per keypress.
            bind -n C-h if -F '#{m/r:^(view|l?n?vim?x?|fzf|winch)(diff)?(-wrapped)?$,#{pane_current_command}}' 'send-keys C-h' 'select-pane -L'
            bind -n C-j if -F '#{m/r:^(view|l?n?vim?x?|fzf|winch)(diff)?(-wrapped)?$,#{pane_current_command}}' 'send-keys C-j' 'select-pane -D'
            bind -n C-k if -F '#{m/r:^(view|l?n?vim?x?|fzf|winch)(diff)?(-wrapped)?$,#{pane_current_command}}' 'send-keys C-k' 'select-pane -U'
            bind -n C-l if -F '#{m/r:^(view|l?n?vim?x?|fzf|winch)(diff)?(-wrapped)?$,#{pane_current_command}}' 'send-keys C-l' 'select-pane -R'
            bind -n 'C-\' if -F '#{m/r:^(view|l?n?vim?x?|fzf|winch)(diff)?(-wrapped)?$,#{pane_current_command}}' 'send-keys C-\\' 'select-pane -l'

            # window cycling: native normally; while the winch sidebar is
            # docked (@winch_docked set on the session) the switch routes
            # through the daemon so the sidebar arrives WITH the window —
            # never a frame of the target without it
            bind -n M-h if-shell -F "#{@winch_docked}" {run-shell -b '${winch}/bin/winch nav prev "#{client_name}"'} {previous-window}
            bind -n M-l if-shell -F "#{@winch_docked}" {run-shell -b '${winch}/bin/winch nav next "#{client_name}"'} {next-window}

            # winch sidebar: M-s docks the list as a real pane on the left
            # (herdr-style; main area stays your live panes), M-s again
            # undocks and restores the exact layout. client passed explicitly
            # — implicit targeting picks the wrong client whenever a second
            # one is attached (the winch control client always is).
            # (M-s previously toggled the native session tree — that stays
            # reachable on prefix+s)
            bind -n M-s run-shell -b '${winch}/bin/winch toggle "#{client_name}"'

            # M-a: the agent switcher — like M-s but pinned on the
            # top-attention agent; rapid taps cycle through agents.
            # (aerospace used to eat alt-a for `layout accordion`; that
            # toggle lives on alt-t now.)
            bind -n M-a run-shell -b '${winch}/bin/winch agents "#{client_name}"'
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
