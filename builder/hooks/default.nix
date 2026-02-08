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
      # From some reason using these packages directly breaks cross compilation
      # with gomod2nix - meaning e.g the hook tries to use an rsync binary from
      # the host platform and not the build platform. In Nixpkgs this exact
      # issue does not exist. Using `pkgs.__splicedPackages.callPackage` in
      # `flake.nix` doesn't help, but it might be related.
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
