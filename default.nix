{ buildGoApplication, go, nix, lib, makeWrapper, nix-prefetch-git }:

buildGoApplication {
  inherit go;
  pname = "gomod2nix";
  version = "0.1";
  src = lib.cleanSourceWith {
    filter = name: type: (! lib.hasSuffix "tests" name) && (! lib.hasSuffix "builder" name);
    src = lib.cleanSource ./.;
  };
  modules = ./gomod2nix.toml;

  allowGoReference = true;

  subPackages = [ "." ];

  nativeBuildInputs = [ makeWrapper ];

  postInstall = ''
    wrapProgram $out/bin/gomod2nix --prefix PATH : ${lib.makeBinPath [ go ]}
  '';
}
