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
    initExtraBeforeCompInit = builtins.readFile ./completions.zsh;
    initExtra = ''
      autoload -Uz promptinit; promptinit
      prompt pure

      setopt AUTO_PUSHD           # Push the current directory visited on the stack.
      setopt PUSHD_IGNORE_DUPS    # Do not store duplicates in the stack.
      setopt PUSHD_SILENT         # Do not print the directory stack after pushd or popd.
    '' +
    builtins.readFile ./aliases.zsh;
  };
}
