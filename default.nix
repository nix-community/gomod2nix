{ lib }:
(import ./generic.nix {
  version = "1.2.0";
  src = lib.cleanSourceWith {
    filter = name: type: builtins.foldl' (v: s: v && ! lib.hasSuffix s name) true [
      "tests"
      "builder"
      "templates"
    ];
    src = lib.cleanSource ./.;
  };
  modules = ./gomod2nix.toml;
})
