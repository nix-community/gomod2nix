{ pkgs ? import <nixpkgs> { } }:

let
  buildGoApplication = pkgs.buildGoApplication or pkgs.callPackage ./builder { };
  inherit (pkgs) lib;

in buildGoApplication {
  pname = "gomod2nix";
  version = "0.1";
  src = lib.cleanSource ./.;
  modules = ./gomod2nix.toml;

  nativeBuildInputs = [
    pkgs.makeWrapper
  ];

  postInstall = ''
    wrapProgram $out/bin/gomod2nix --prefix PATH : ${lib.makeBinPath [ pkgs.nix-prefetch-git ]}
    rm -f $out/bin/builder
  '';
}
