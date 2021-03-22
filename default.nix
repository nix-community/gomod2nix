{ buildGoApplication, lib, makeWrapper, go, nix-prefetch-git }:

buildGoApplication {
  pname = "gomod2nix";
  version = "0.1";
  src = lib.cleanSourceWith {
    filter = name: type: ! lib.hasSuffix "tests" name;
    src = lib.cleanSource ./.;
  };
  modules = ./gomod2nix.toml;

  subPackages = [ "." ];

  nativeBuildInputs = [ makeWrapper ];

  postInstall = ''
    wrapProgram $out/bin/gomod2nix --prefix PATH : ${lib.makeBinPath [ go nix-prefetch-git ]}
    rm -f $out/bin/builder
  '';
}
