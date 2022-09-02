{ stdenv
, callPackage
, lib
, makeWrapper
, installShellFiles
, fetchFromGitHub
, buildGoApplication
, mkGoEnv
, which
}:

buildGoApplication {
  pname = "gomod2nix";
  version = "dev";

  pwd = ./.;

  modules = ./gomod2nix.toml;

  src = lib.cleanSourceWith {
    filter = name: type: builtins.foldl' (v: s: v && ! lib.hasSuffix s name) true [
      "tests"
      "builder"
      "templates"
    ];
    src = lib.cleanSource ./.;
  };

  allowGoReference = true;

  subPackages = [ "." ];

  nativeBuildInputs = [ makeWrapper installShellFiles which ];

  passthru = {
    inherit buildGoApplication mkGoEnv;
  };

  postInstall = lib.optionalString (stdenv.buildPlatform == stdenv.targetPlatform) ''
    installShellCompletion --cmd gomod2nix \
      --bash <($out/bin/gomod2nix completion bash) \
      --fish <($out/bin/gomod2nix completion fish) \
      --zsh <($out/bin/gomod2nix completion zsh)
  '' + ''
    wrapProgram $out/bin/gomod2nix --suffix PATH : $(dirname $(which go))
  '';

  meta = {
    description = "Convert applications using Go modules -> Nix";
    homepage = "https://github.com/nix-community/gomod2nix";
    license = lib.licenses.mit;
    maintainers = [ lib.maintainers.adisbladis ];
  };
}
