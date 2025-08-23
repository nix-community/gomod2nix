{
  runCommand,
  mkGoEnv,
  which,
}:

let
  env = mkGoEnv {
    pwd = ./.;
  };
in
runCommand "mkgoenv-assert"
  {
    nativeBuildInputs = [ which ];
    buildInputs = [ env ]; # Trigger propagation
  }
  ''
    if ! test -f ${env}/bin/stringer; then
      echo "stringer command not found in env!"
      exit 1
    fi

    which go > /dev/null || echo "Go compiler not found in env!"

    ln -s ${env} $out
  ''
