final: prev: {
  inherit (final.callPackage ./builder { }) buildGoApplication mkGoEnv;
  gomod2nix = final.callPackage ./default.nix { };
}
