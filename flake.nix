{
  description = "Convert go.mod/go.sum to Nix packages";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  inputs.utils.url = "github:numtide/flake-utils";

  outputs = { self, nixpkgs, utils }:
    {
      overlay = final: prev: {
        buildGoApplication = final.callPackage ./builder { };
        gomod2nix = final.callPackage ./default.nix { };
      };
    } //
    (utils.lib.eachDefaultSystem
      (system:
        let
          pkgs = import nixpkgs {
            inherit system;
            overlays = [
              self.overlay
            ];
          };
        in
        {
          defaultPackage = pkgs.callPackage ./default.nix { };
          devShell = with pkgs; mkShell {
            buildInputs = [
              gomod2nix
            ];
          };
        })
    );
}
