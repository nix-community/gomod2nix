{ pkgs ? import <nixpkgs> {
    overlays = [
      (import ./overlay.nix)
    ];
  }
}:
let
  pythonEnv = pkgs.python3.withPackages (_: [ ]);

in
pkgs.mkShell {
  buildInputs = [
    pkgs.nix-prefetch-git
    pkgs.nixpkgs-fmt
    pkgs.go
    pkgs.gomod2nix
    pythonEnv
  ];
}
