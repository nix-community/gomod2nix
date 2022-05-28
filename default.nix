{ buildGoApplication, go_1_18, nix, lib, makeWrapper, nix-prefetch-git }:

let
  go = go_1_18;
in
buildGoApplication {
  inherit go;
  pname = "gomod2nix";
  version = "0.1";
  src = lib.cleanSourceWith {
    filter = name: type: ! lib.hasSuffix "tests" name;
    src = lib.cleanSource ./.;
  };
  modules = ./gomod2nix.toml;

  allowGoReference = true;

  subPackages = [ "." ];

  nativeBuildInputs = [ makeWrapper ];

  postInstall = ''
    wrapProgram $out/bin/gomod2nix --prefix PATH : ${lib.makeBinPath [ go ]}
    rm -f $out/bin/builder
  '';
}
