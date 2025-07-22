final: prev: {
  inherit (final.callPackage ./builder { }) buildGoApplication mkGoEnv mkVendorEnv;
  gomod2nix = final.callPackage ./default.nix { };
}
