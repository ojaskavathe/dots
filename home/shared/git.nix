{
  pkgs,
  lib,
  config,
  ...
}:
{
  options = {
    git = {
      enable = lib.mkEnableOption "Enable Git";
    };
  };

  config = lib.mkIf config.git.enable {
    programs.git = {
      enable = true;
      settings = {
        user = {
          name = "Ojas Kavathe";
          email = "ojaskavathe@gmail.com";
        };
        pull.rebase = false;
        alias = {
          a = "add";
          c = "commit";
          ca = "commit --amend";
          can = "commit --amend --no-edit";
          cl = "clone";
          cm = "commit -m";
          co = "checkout";
          cp = "cherry-pick";
          cpx = "cherry-pick -x";
          d = "diff";
          f = "fetch";
          fo = "fetch origin";
          fu = "fetch upstream";
          lol = "log --graph --decorate --pretty=oneline --abbrev-commit";
          lola = "log --graph --decorate --pretty=oneline --abbrev-commit --all";
          pl = "pull";
          pr = "pull -r";
          ps = "push";
          psf = "push -f";
          rb = "rebase";
          rbi = "rebase -i";
          r = "remote";
          ra = "remote add";
          rr = "remote rm";
          rv = "remote -v";
          rs = "remote show";
          st = "status";
        };
      };
    };
  };
}
