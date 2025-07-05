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
      };
      defaultTemplate = self.templates.app;

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

          # The current default sdk for macOS fails to compile go projects, so we use a newer one for now.
          # This has no effect on other platforms.

          inherit
            (pkgs.callPackage ./builder {
              inherit gomod2nix;
            })
            mkGoEnv
            buildGoApplication
            ;
          gomod2nix = pkgs.callPackage ./default.nix {
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
          devShells.default = pkgs.callPackage ./shell.nix {
            inherit mkGoEnv gomod2nix;
          };
        }
      )
    );
}
