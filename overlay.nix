final: prev: {
  inherit (final.callPackage ./builder { })
    buildGoApplication
    mkGoEnv
    mkVendorEnv
    mkGoCacheEnv
    mkWorkspaceCacheEnv
    hooks
    ;
  gomod2nix = final.callPackage ./default.nix { };
}
