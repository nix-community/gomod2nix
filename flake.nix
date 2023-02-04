{
  description = "Convert go.mod/go.sum to Nix packages";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/master";

  inputs.utils.url = "github:numtide/flake-utils";

  outputs = { self, nixpkgs, utils }:
    {
      overlays.default = import ./overlay.nix;

      templates = {
        app = {
          path = ./templates/app;
          description = "Gomod2nix packaged application";
        };
      };
      templates.default = self.templates.app;

    } //
    (utils.lib.eachDefaultSystem
      (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          callPackage = pkgs.darwin.apple_sdk_11_0.callPackage or pkgs.callPackage;
        in
        rec {
          packages.default = pkgs.callPackage ./. { inherit (lib) buildGoApplication mkGoEnv; };
          devShells.default = import ./shell.nix { inherit pkgs; gomod2nix = packages.default; inherit (lib) mkGoEnv; };
          lib = { inherit (callPackage ./builder { gomod2nix = packages.default; }) buildGoApplication mkGoEnv; };
        })
    );
}
