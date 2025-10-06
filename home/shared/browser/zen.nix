{
  pkgs,
  lib,
  config,
  ...
}:
{
  options = {
    zen = {
      enable = lib.mkEnableOption "Enable Zen Browser";
    };
  };

  config = lib.mkIf config.zen.enable {
    home.packages = [
      (pkgs.writeShellApplication {
        name = "extract-zen-extension";
        runtimeInputs = with pkgs; [curl unzip jq gnused coreutils];
        text = builtins.readFile ./extract-zen-extension.sh;
      })
    ];

    programs.zen-browser = {
      enable = true;
      # policies = let
      #   mkExtensionSettings = builtins.mapAttrs (_: pluginId: {
      #     install_url = "https://addons.mozilla.org/firefox/downloads/latest/${pluginId}/latest.xpi";
      #     installation_mode = "force_installed";
      #   });
      # in {
      #   AutofillAddressEnabled = true;
      #   AutofillCreditCardEnabled = false;
      #   DisableAppUpdate = true;
      #   DisableFeedbackCommands = true;
      #   DisableFirefoxStudies = true;
      #   DisablePocket = true; # save webs for later reading
      #   DisableTelemetry = true;
      #   DontCheckDefaultBrowser = true;
      #   NoDefaultBookmarks = true;
      #   OfferToSaveLogins = false;
      #   EnableTrackingProtection = {
      #     Value = true;
      #     Locked = true;
      #     Cryptomining = true;
      #     Fingerprinting = true;
      #   };
      #   ExtensionSettings = mkExtensionSettings {
      #     "{85860b32-02a8-431a-b2b1-40fbd64c9c69}" = "github-file-icons";
      #     "{446900e4-71c2-419f-a6a7-df9c091e268b}" = "bitwarden-password-manager";
      #   };
      # };
      # nativeMessagingHosts = [pkgs.firefoxpwa];
      profiles."default" = {
        id = 0;
        isDefault = true;
        # containersForce = true;
        # containers = {
        #   Personal = {
        #     color = "purple";
        #     icon = "fingerprint";
        #     id = 1;
        #   };
        #   Work = {
        #     color = "blue";
        #     icon = "briefcase";
        #     id = 2;
        #   };
        # };
        # spacesForce = true;
        # spaces = let
        #   containers = config.programs.zen-browser.profiles."default".containers;
        # in {
        #   "personal" = {
        #     id = "c6de089c-410d-4206-961d-ab11f988d40a";
        #     container = containers."Personal".id;
        #     position = 1000;
        #   };
        #   "invideo" = {
        #     id = "cdd10fab-4fc5-494b-9041-325e5759195b";
        #     icon = "chrome://browser/skin/zen-icons/selectable/star-2.svg";
        #     container = containers."Work".id;
        #     position = 2000;
        #   };
        # };
      };
    };
  };
}
