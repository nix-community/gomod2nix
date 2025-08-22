final: prev:
let
  # The current default sdk for macOS fails to compile go projects, so we use a newer one for now.
  # This has no effect on other platforms.
  callPackage = final.callPackage;
in
{
  inherit (callPackage ./builder { }) buildGoApplication mkGoEnv mkVendorEnv;
  gomod2nix = callPackage ./default.nix { };
}
