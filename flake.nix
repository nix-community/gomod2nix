{
  description = "Convert go.mod/go.sum to Nix packages";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/master";
  inputs.flake-utils.url = "github:numtide/flake-utils";

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    {
      overlays.default = import ./overlay.nix;

      templates = {
        app = {
          path = ./templates/app;
          description = "Gomod2nix packaged application";
        };
        container = {
          path = ./templates/container;
          description = "Gomod2nix packaged container";
        };
        default = self.templates.app;
      };
    }
    // (flake-utils.lib.eachSystem
      [
        "aarch64-linux"
        "aarch64-darwin"
        "x86_64-darwin"
        "x86_64-linux"
        "riscv64-linux"
      ]
      (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};

          callPackage = pkgs.callPackage;

          inherit
            (callPackage ./builder {
              inherit gomod2nix;
            })
            mkGoEnv
            buildGoApplication
            ;
          gomod2nix = callPackage ./default.nix {
            inherit mkGoEnv buildGoApplication;
          };
        in
        {
          packages.default = gomod2nix;
          legacyPackages = {
            # we cannot put them in packages because they are builder functions
            inherit mkGoEnv buildGoApplication;
            # just have this here for convenience
            inherit gomod2nix;
          };
          devShells.default = callPackage ./shell.nix {
            inherit mkGoEnv gomod2nix;
          };
        }
      )
    );
}
