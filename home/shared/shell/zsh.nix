{
  pkgs,
  lib,
  config,
  ...
}:
{

  options = {
    zsh = {
      enable = lib.mkEnableOption "Enable ZSH";
    };
  };

  config = lib.mkIf config.zsh.enable {
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
      plugins = [
        {
          name = "pure-prompt";
          src = "${pkgs.pure-prompt}/share/zsh/site-functions";
        }
        {
          name = "zsh-autosuggestions";
          src = pkgs.zsh-autosuggestions;
          file = "share/zsh-autosuggestions/zsh-autosuggestions.zsh";
        }
      ];

      # https://nix-community.github.io/home-manager/options.xhtml#opt-programs.zsh.initContent
      initContent =
        let
          initExtraBeforeCompInit = lib.mkOrder 550 (builtins.readFile ./completions.zsh);
          initExtra = lib.mkOrder 1000 ''
            autoload -Uz promptinit; promptinit
            prompt pure

            setopt AUTO_PUSHD           # Push the current directory visited on the stack.
            setopt PUSHD_IGNORE_DUPS    # Do not store duplicates in the stack.
            setopt PUSHD_SILENT         # Do not print the directory stack after pushd or popd.

            ${builtins.readFile ./aliases.zsh}

            export LC_ALL=en_US.UTF-8
            export LANG=en_US.UTF-8

            bindkey -v                  # vi mode
          '';
        in
        lib.mkMerge [
          initExtraBeforeCompInit
          initExtra
        ];
    };
  };
}
