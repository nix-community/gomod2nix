{ pkgs ? import <nixpkgs> { } }:
let
  pythonEnv = pkgs.python3.withPackages (_: [ ]);

in
pkgs.mkShell {
  buildInputs = [
    pkgs.nix-prefetch-git
    pkgs.nixpkgs-fmt
    pkgs.go
    pythonEnv
  ];
}
