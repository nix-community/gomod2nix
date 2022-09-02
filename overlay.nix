final: prev:
let
  # The newer Darwin SDK does not exist in current (nixos-22.05) stable
  # branches, so just fallback to the default callPackage.
  callPackage = final.darwin.apple_sdk_11_0.callPackage or final.callPackage;
in
{
  inherit (callPackage ./builder { }) buildGoApplication mkGoEnv;
  gomod2nix = callPackage ./default.nix { };
}
