{ runCommand, buildGoApplication }:

let
  modules = builtins.fromTOML (builtins.readFile ./gomod2nix.toml);

  drv = buildGoApplication {
    pname = "private-via-git";
    version = "0.1";
    pwd = ./.;
    src = ./.;
  };
in
assert modules.mod."github.com/spf13/pflag".git.url == "https://github.com/spf13/pflag";
assert !(drv.vendorEnv.passthru.sources."github.com/spf13/pflag" ? drvPath);
runCommand "private-via-git-assert" { } ''
  test -x ${drv}/bin/private-via-git
  ln -s ${drv} $out
''
