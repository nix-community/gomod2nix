{ buildGoApplication, go, lib }:

buildGoApplication {
  inherit go;
  pname = "myapp";
  version = "0.1";
  src = ./.;
  modules = ./gomod2nix.toml;
  subPackages = [ "." ];
}
