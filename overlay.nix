final: prev:
let
  # The current default sdk for macOS fails to compile go projects, so we use a newer one for now.
  # This has no effect on other platforms.
  callPackage = final.darwin.apple_sdk_11_0.callPackage or final.callPackage;
in
{
  inherit (callPackage ./builder { }) buildGoApplication mkGoEnv;
  gomod2nix = callPackage ./default.nix { };
}
