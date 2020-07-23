let
  pkgs = import <nixpkgs> {
    overlays = [
      (self: super: {
        buildGoApplication = super.callPackage ./builder { };
      })
    ];
  };

  inherit (pkgs) lib;

in pkgs.buildGoApplication {
  pname = "gomod2nix";
  version = "0.1";
  src = lib.cleanSource ./.;
  modules = ./gomod2nix.toml;
}
