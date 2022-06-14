{ pkgs ? (
    let
      inherit (builtins) fetchTree fromJSON readFile;
    in
    import (fetchTree (fromJSON (readFile ./flake.lock)).nodes.nixpkgs.locked) {
      overlays = [
        (import ./overlay.nix)
      ];
    }
  )
}:

pkgs.mkShell {
  NIX_PATH = "nixpkgs=${builtins.toString pkgs.path}";
  buildInputs = [
    pkgs.nixpkgs-fmt
    pkgs.gomod2nix.go
    pkgs.gomod2nix
    pkgs.golangci-lint
  ];
}
