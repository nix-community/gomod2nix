final: prev:
let
  callPackage = final.darwin.apple_sdk_11_0.callPackage;
in
{
  inherit (callPackage ./builder { }) buildGoApplication mkGoEnv;
  gomod2nix = callPackage (callPackage ./default.nix { }) { };
}
