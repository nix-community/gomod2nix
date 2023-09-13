{
  description = "A basic gomod2nix flake";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  inputs.flake-utils.url = "github:numtide/flake-utils";
  inputs.gomod2nix.url = "github:nix-community/gomod2nix";

  outputs = { self, nixpkgs, flake-utils, gomod2nix }:
    (flake-utils.lib.eachDefaultSystem
      (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          packages.default = pkgs.callPackage ./. {
             inherit (gomod2nix.legacyPackages.${system}) buildGoApplication;
          };
          devShells.default = pkgs.callPackage ./shell.nix {
            inherit (gomod2nix.legacyPackages.${system}) buildGoApplication mkGoEnv gomod2nix;
          };
        })
    );
}
