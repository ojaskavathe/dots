{ pkgs, ... }: {
  programs.kitty = {
    enable = true;
    font = {
      name = "JetBrainsMonoNL Nerd Font";
    };
    theme = "Catppuccin-Mocha";
  };
}
