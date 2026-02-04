{
  lib,
  makeSetupHook,
  buildPackages,
  stdenv,
}:
{
  goConfigHook = makeSetupHook {
    name = "goConfigHook";
    substitutions = {
      rsync = lib.getExe buildPackages.rsync;
      tar = lib.getExe buildPackages.gnutar;
      zstd = lib.getExe buildPackages.zstd;
    };
  } ./go-config-hook.sh;

  goBuildHook = makeSetupHook {
    name = "goBuildHook";
    substitutions = {
      hostPlatformConfig = stdenv.hostPlatform.config;
      buildPlatformConfig = stdenv.buildPlatform.config;
    };
  } ./go-build-hook.sh;

  goCheckHook = makeSetupHook {
    name = "goCheckHook";
  } ./go-check-hook.sh;

  goInstallHook = makeSetupHook {
    name = "goInstallHook";
  } ./go-install-hook.sh;
}
