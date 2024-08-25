{ pkgs, ... }: {

  programs.tmux = {
    enable = true;
 
    baseIndex = 1;
    terminal = "tmux-256color";
    mouse = true;
    prefix = "C-Space";

    plugins = with pkgs; [
      {
        plugin = tmuxPlugins.catppuccin;
        extraConfig = ''
          # colorscheme
          set -g @catppuccin_flavour "mocha" # latte, frappe, macchiato, mocha
          
          # statusbar
          set -g @catppuccin_date_time_text "%Y-%m-%d %H:%M"
          set -g @catppuccin_status_modules_right "application session date_time"
          set -g @catppuccin_status_left_separator "█"
          set -g @catppuccin_status_right_separator "█"
        '';
      }
      tmuxPlugins.vim-tmux-navigator
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
      bind-key -T copy-mode-vi y send-keys -X copy-selection-and-cancel
      
      # open panes in current directory
      bind '"' split-window -h -c "#{pane_current_path}"
      bind % split-window -v -c "#{pane_current_path}"
      bind c new-window -c "#{pane_current_path}"
      
      # restore clear with <prefix>C-l
      bind C-l send-keys 'C-l'
    '';
  };

}
