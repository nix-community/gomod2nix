{
  description = "Convert go.mod/go.sum to Nix packages";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  inputs.utils.url = "github:numtide/flake-utils";

  outputs = { self, nixpkgs, utils }: utils.lib.eachDefaultSystem (system:
    let
      pkgs = import nixpkgs { inherit system; };
    in
    {
      overlay = final: prev: {
        buildGoApplication = final.callPackage ./builder { };
        gomod2nix = final.callPackage ./default.nix { };
      };

      defaultPackage = pkgs.callPackage ./default.nix { };

    });

}
