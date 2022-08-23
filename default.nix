{ stdenv, buildGoApplication, go, nix, lib, makeWrapper, installShellFiles }:

buildGoApplication {
  inherit go;
  pname = "gomod2nix";
  version = "1.0.0";
  src = lib.cleanSourceWith {
    filter = name: type: builtins.foldl' (v: s: v && ! lib.hasSuffix s name) true [
      "tests"
      "builder"
      "templates"
    ];
    src = lib.cleanSource ./.;
  };
  modules = ./gomod2nix.toml;

  allowGoReference = true;

  subPackages = [ "." ];

  nativeBuildInputs = [ makeWrapper installShellFiles ];

  postInstall = lib.optionalString (stdenv.buildPlatform == stdenv.targetPlatform) ''
    $out/bin/gomod2nix completion bash > gomod2nix.bash
    $out/bin/gomod2nix completion fish > gomod2nix.fish
    $out/bin/gomod2nix completion zsh > _gomod2nix
    installShellCompletion gomod2nix.{bash,fish} _gomod2nix
  '' + ''
    wrapProgram $out/bin/gomod2nix --prefix PATH : ${lib.makeBinPath [ go ]}
  '';

  meta = {
    description = "Convert applications using Go modules -> Nix";
    homepage = "https://github.com/tweag/gomod2nix";
    license = lib.licenses.mit;
    maintainers = [ lib.maintainers.adisbladis ];
  };
}
