{
  pkgs ? (
    let
      inherit (builtins) fetchTree fromJSON readFile;
    in
    import (fetchTree (fromJSON (readFile ./flake.lock)).nodes.nixpkgs.locked) {
      overlays = [
        (import ./overlay.nix)
      ];
    }
  ),
  gomod2nix ? pkgs.gomod2nix,
  mkGoEnv ? pkgs.mkGoEnv,
}:

pkgs.mkShell {
  NIX_PATH = "nixpkgs=${builtins.toString pkgs.path}";
  nativeBuildInputs = [
    pkgs.nixfmt-tree
    pkgs.golangci-lint
    gomod2nix
    (mkGoEnv { pwd = ./.; })
  ];
}
