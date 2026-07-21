{
  pkgs,
  lib,
  config,
  ...
}:
let
  # Upstream publishes no git tags and the PyPI version (1.6.4) has drifted from
  # pyproject's (1.6.0), so pin a commit: it keeps the server and addon.py from
  # the same tree, which is what guarantees they speak the same socket protocol.
  src = pkgs.fetchFromGitHub {
    owner = "ahujasid";
    repo = "blender-mcp";
    rev = "6641189231caf3752302ae20591bc87fda85fc4e";
    hash = "sha256-4I5pLS4bf0PrPUolbrsGrZmnEhMFLlL0ELYIBBeHUns=";
  };

  blender-mcp = pkgs.python3Packages.buildPythonApplication {
    pname = "blender-mcp";
    version = "1.6.0-unstable-2026-07-14";
    pyproject = true;
    inherit src;

    build-system = with pkgs.python3Packages; [ setuptools ];
    dependencies = with pkgs.python3Packages; [
      mcp
      httpx
    ];

    # Telemetry is opt-out and defaults to uploading prompts and code snippets.
    makeWrapperArgs = [ "--set DISABLE_TELEMETRY true" ];

    doCheck = false; # upstream ships no tests
    pythonImportsCheck = [ "blender_mcp.server" ];

    meta = {
      description = "Blender integration through the Model Context Protocol";
      homepage = "https://github.com/ahujasid/blender-mcp";
      license = lib.licenses.mit;
      mainProgram = "blender-mcp";
    };
  };

  # Legacy bl_info addon (never migrated to the 4.2+ extensions platform), so it
  # belongs in scripts/addons and is keyed by filename: blender_mcp_addon.
  addonModule = "blender_mcp_addon";
  blenderVersion = lib.versions.majorMinor pkgs.blender.version;
  addonDir =
    if pkgs.stdenv.hostPlatform.isDarwin then
      "Library/Application Support/Blender/${blenderVersion}/scripts/addons"
    else
      "${config.xdg.configHome}/blender/${blenderVersion}/scripts/addons";
in
{
  options = {
    blender-mcp = {
      enable = lib.mkEnableOption "Blender MCP server and addon";
    };
  };

  config = lib.mkIf config.blender-mcp.enable {
    home.packages = [
      blender-mcp
      pkgs.blender
    ];

    home.file."${addonDir}/${addonModule}.py".source = "${src}/addon.py";

    claude.mcpServers.blender = {
      type = "stdio";
      command = lib.getExe blender-mcp;
    };
  };
}
