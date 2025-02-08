{
  lib,
  config,
  ...
}:
{
  options = {
    nvidia = {
      enable = lib.mkEnableOption "Enables NVIDIA drivers";
      optimus = lib.mkEnableOption "Enables NVIDIA Optimus";
    };
  };

  config = lib.mkIf config.nvidia.enable {
    services.xserver.videoDrivers = [ "nvidia" ];

    hardware.graphics.enable = true;

    hardware.nvidia = {
      modesetting.enable = true;
      powerManagement.enable = false;
      powerManagement.finegrained = false;

      open = false;

      # accessible via `nvidia-settings`.
      nvidiaSettings = true;

      prime = lib.mkIf config.nvidia.optimus {
        offload = {
          enable = true;
          enableOffloadCmd = true;
        };

        nvidiaBusId = "PCI:01:0:0";
        amdgpuBusId = "PCI:06:0:0";
      };

      # Optionally, you may need to select the appropriate driver version for your specific GPU.
      package = config.boot.kernelPackages.nvidiaPackages.production;
    };

    # environment.systemPackages = [ nvidia-offload ];
  };
}
