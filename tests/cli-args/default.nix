{ runCommand, buildGoApplication }:

let
  drv = buildGoApplication {
    pname = "stringer";
    pwd = ./.;
  };
in
assert drv.version == "0.36.0";
runCommand "cli-args-stringer-assert" { } ''
  if ! test -f ${drv}/bin/stringer; then
    echo "stringer command not found in env!"
    exit 1
  fi

  ln -s ${drv} $out
''
