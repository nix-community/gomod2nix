self: super: {
  buildGoApplication = self.callPackage ./builder { };
}
