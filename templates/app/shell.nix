{ pkgs ? import <nixpkgs> { } }:

pkgs.mkShell {
  packages = [
    pkgs.go
    pkgs.gomod2nix
  ];
}
