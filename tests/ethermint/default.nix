{ stdenv, buildGoApplication }:

buildGoApplication rec {
  pname = "ethermint";
  version = "0.0.1";
  src = ./.;
  modules = ./gomod2nix.toml;
  doCheck = false;
  subPackages = [ "./cmd/main.go" ];
}
