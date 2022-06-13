{ runCommand, mkGoEnv }:

let
  env = mkGoEnv {
    pwd = ./.;
  };
in
runCommand "mkgoenv-assert" { } ''
  if ! test -f ${env}/bin/stringer; then
    echo "stringer command not found in env!"
    exit 1
  fi

  ln -s ${env} $out
''
