{
  stdenv,
  go,
  lib,
  makeWrapper,
  installShellFiles,
  buildGoApplication,
  mkGoEnv,
}:

buildGoApplication {
  pname = "gomod2nix";
  version = "dev";

  modules = ./gomod2nix.toml;

  src = lib.cleanSourceWith {
    filter =
      name: type:
      builtins.foldl' (v: s: v && !lib.hasSuffix s name) true [
        "tests"
        "builder"
        "templates"
      ];
    src = lib.cleanSource ./.;
  };

  inherit go;

  allowGoReference = true;

  subPackages = [ "." ];

  nativeBuildInputs = [
    makeWrapper
    installShellFiles
  ];

  passthru = {
    inherit buildGoApplication mkGoEnv;
  };

  postInstall =
    lib.optionalString (stdenv.buildPlatform == stdenv.targetPlatform) ''
      installShellCompletion --cmd gomod2nix \
        --bash <($out/bin/gomod2nix completion bash) \
        --fish <($out/bin/gomod2nix completion fish) \
        --zsh <($out/bin/gomod2nix completion zsh)
    ''
    + ''
      wrapProgram $out/bin/gomod2nix --prefix PATH : ${lib.makeBinPath [ go ]}
    '';

  meta = {
    description = "Convert applications using Go modules -> Nix";
    homepage = "https://github.com/nix-community/gomod2nix";
    license = lib.licenses.mit;
    maintainers = [ lib.maintainers.adisbladis ];
  };
}
