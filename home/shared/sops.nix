{
  inputs,
  config,
  lib,
  ...
}:
let
  sopsFolder = (builtins.toString inputs.secrets);
  homeDirectory = config.home.homeDirectory;
  defaultSopsFile = "${sopsFolder}/hosts/common/secrets.yaml";
  fileSecrets =
    lib.attrsets.mergeAttrsList (
      lib.lists.map (name: {
        "${name}" = {
          sopsFile = defaultSopsFile;
        };
      }) [
        "aws_access_key_id"
        "aws_secret_access_key"
        "litellm_api_key"
        "litellm_endpoint"
      ]
    );
in
{
  imports = [
    inputs.sops-nix.homeManagerModules.sops
  ];

  home.sessionVariables = {
    SOPS_AGE_KEY_FILE = "${homeDirectory}/.config/sops/age/keys.txt";
  };

  sops = {
    age.keyFile = "${homeDirectory}/.config/sops/age/keys.txt";

    inherit defaultSopsFile;
    validateSopsFiles = false;

    secrets = fileSecrets;
  };
}
