{ pkgs ? import <nixpkgs> { } }:

pkgs.mkShell {
  buildInputs = [
    pkgs.nix-prefetch-git
    pkgs.go
  ];
}
