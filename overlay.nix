self: super: {
  buildGoApplication = self.callPackage ./builder { };
  gomod2nix = self.callPackage ./default.nix { };
}
