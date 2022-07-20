final: prev: {
  inherit (final.darwin.apple_sdk_11_0.callPackage ./builder { }) buildGoApplication mkGoEnv;
  gomod2nix = final.darwin.apple_sdk_11_0.callPackage ./default.nix { };
}
