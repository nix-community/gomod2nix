{ runCommand, buildGoApplication }:

let
  drv = buildGoApplication {
    pname = "build-info";
    version = "0.1";
    pwd = ./.;
    src = ./.;
    subPackages = [ "." ];
  };
in
runCommand "build-info-assert" { nativeBuildInputs = [ drv.go ]; } ''
  go version -m ${drv}/bin/build-info | tee build-info

  for dep in "github.com/samber/lo v1.47.0" "github.com/spf13/cobra v1.10.1" "github.com/spf13/pflag v1.0.9" "example.com/lib v0.0.0"; do
    if ! grep -qE "^\s+dep\s+$(echo $dep | sed 's/ /\\s+/')" build-info; then
      echo "dependency $dep missing from build info!"
      exit 1
    fi
  done

  ln -s ${drv} $out
''
