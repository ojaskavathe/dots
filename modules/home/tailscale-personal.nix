# A second tailscaled on the personal tailnet, beside the Tailscale app on the work one. In userspace mode, as you: no
# root, no network interface, no routes, so the work connection can't tell it's there. It serves local web apps to the
# personal tailnet (`ts2 serve`) and offers a local proxy (SOCKS5 on 1055, HTTP on 1056) for whatever should reach
# personal machines. Only TCP goes through the proxy: UDP apps (Moonlight) need the app switched to personal. Logged in
# once by its link (`ts2 up --hostname=camille-personal`); its state folder keeps the login.
{ ... }:
{
  flake.modules.homeManager.tailscale-personal =
    {
      lib,
      config,
      pkgs,
      ...
    }:
    let
      dir = "${config.home.homeDirectory}/.local/state/tailscale-personal";
      socket = "${dir}/tailscaled.sock";
      # The CLI for this one; plain `tailscale` is the app's.
      ts2 = pkgs.writeShellScriptBin "ts2" ''exec ${pkgs.tailscale}/bin/tailscale --socket=${socket} "$@"'';
    in
    {
      options.tailscale-personal.enable = lib.mkEnableOption "a second, userspace tailscaled on the personal tailnet";

      config = lib.mkIf (config.tailscale-personal.enable && pkgs.stdenv.hostPlatform.isDarwin) {
        home.packages = [ ts2 ];

        home.activation.tailscalePersonalDir = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
          run mkdir -p ${dir}
          run chmod 700 ${dir}
        '';

        launchd.agents.tailscale-personal = {
          enable = true;
          config = {
            ProgramArguments = [
              "${pkgs.tailscale}/bin/tailscaled"
              "--tun=userspace-networking"
              "--statedir=${dir}"
              "--socket=${socket}"
              # Not the app's 41641.
              "--port=41642"
              "--socks5-server=127.0.0.1:1055"
              "--outbound-http-proxy-listen=127.0.0.1:1056"
            ];
            RunAtLoad = true;
            KeepAlive = true;
            ThrottleInterval = 30;
            # It logs a lot and nothing rotates it (a guide's grew to 210 MB): `ts2 status` says how it's doing.
            StandardOutPath = "/dev/null";
            StandardErrorPath = "/dev/null";
          };
        };
      };
    };
}
