{ version
, src
, modules
}:
{ stdenv
, callPackage
, go
, lib
, makeWrapper
, installShellFiles
, fetchFromGitHub
, buildGoApplication ? (callPackage ./builder { }).buildGoApplication
, mkGoEnv ? (callPackage ./builder { }).buildGoApplication
}:

buildGoApplication {
  pname = "gomod2nix";
  inherit version modules src go;

  allowGoReference = true;

  subPackages = [ "." ];

  nativeBuildInputs = [ makeWrapper installShellFiles ];

  passthru = {
    inherit buildGoApplication mkGoEnv;
  };

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
