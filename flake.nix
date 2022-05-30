{
  description = "Convert go.mod/go.sum to Nix packages";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

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
      defaultTemplate = self.templates.app;

    } //
    (utils.lib.eachDefaultSystem
      (system:
        let
          pkgs = import nixpkgs {
            inherit system;
            overlays = [
              self.overlays.default
            ];
          };
        in
        {
          packages.default = pkgs.callPackage ./default.nix { };
          devShells.default = import ./shell.nix { inherit pkgs; };
        })
    );
}
