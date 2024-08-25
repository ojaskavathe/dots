{ pkgs, ... }: {
  programs.zsh = {
    enable = true;
    dotDir = ".config/zsh";
    enableCompletion = true;
    history = {
      extended = true;
      share = true;
      expireDuplicatesFirst = true;
      ignoreDups = true;
      ignoreAllDups = true;
      ignoreSpace = true;
    };
    syntaxHighlighting.enable = true;
    plugins = with pkgs; [
      rec {
        name = "pure-prompt";
        src = "${pkgs.pure-prompt}/share/zsh/site-functions";
      }
    ];
    initExtra = ''
      autoload -Uz promptinit; promptinit
      prompt pure
    '';
  };
}
