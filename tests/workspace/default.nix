{
  runCommand,
  buildGoApplication,
}:

let
  drv = buildGoApplication {
    pname = "hello";
    version = "0.0.1";
    src = ./.;
    pwd = ./hello;
    modules = ./gomod2nix.toml;
    workspaceModule = "example.com/hello";
    doCheck = false;
  };
in
runCommand "workspace-hello-assert" { } ''
  if ! test -f ${drv}/bin/hello; then
    echo "hello binary not found!"
    exit 1
  fi

  result=$(${drv}/bin/hello)
  expected="Olleh"
  if [ "$result" != "$expected" ]; then
    echo "Expected '$expected' but got '$result'"
    exit 1
  fi

  ln -s ${drv} $out
''
