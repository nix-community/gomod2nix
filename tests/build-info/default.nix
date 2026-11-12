{
  runCommand,
  buildGoApplication,
  buildGoModule,
}:

let
  drv = buildGoApplication {
    pname = "build-info";
    version = "0.1";
    pwd = ./.;
    src = ./.;
    subPackages = [ "." ];
  };

  reference = buildGoModule.override { inherit (drv) go; } {
    pname = "build-info-reference";
    version = "0.1";
    src = ./.;
    subPackages = [ "." ];
    vendorHash = "sha256-peg2oAJ0fp7dvrj4IeFOd4u1wO1azDHhOeoH3uQSkc4=";
  };
in
runCommand "build-info-assert" { nativeBuildInputs = [ drv.go ]; } ''
  deps() { go version -m "$1" | awk '$1 == "dep" || $1 == "=>"'; }
  diff -u <(deps ${reference}/bin/build-info) <(deps ${drv}/bin/build-info)

  ln -s ${drv} $out
''
