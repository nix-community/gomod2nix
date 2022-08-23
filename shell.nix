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
  nativeBuildInputs = [
    pkgs.nixpkgs-fmt
    pkgs.golangci-lint
    pkgs.gomod2nix
    (pkgs.mkGoEnv { pwd = ./.; })
  ];
}
