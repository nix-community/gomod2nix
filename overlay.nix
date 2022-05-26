final: prev: {
  buildGoApplication = final.callPackage ./builder { };
  gomod2nix = final.callPackage ./default.nix { };
}
