{ runCommand, buildGoApplication }:

# `go test` reuses the GOCACHE restored from `cachePackages` only if it builds
# with the same flags as the build that populated it (-trimpath). The check hook
# drops -trimpath by default, which makes every dependency a cache miss, so the
# whole tree is compiled again. `keepTrimpathInCheck = true` keeps the flag.
#
# We run the check with `-x` and look for `compile ... -p github.com/spf13/...`
# in its output: that line is only printed when the package is really compiled,
# not when it is served from the cache.
let
  mk =
    keepTrimpathInCheck:
    buildGoApplication {
      pname = "keep-trimpath-in-check";
      version = "0.1";
      src = ./.;
      pwd = ./.;
      doCheck = true;
      inherit keepTrimpathInCheck;

      preCheck = ''
        export GOFLAGS="$GOFLAGS -x"
        exec 2> >(tee "$TMPDIR/check.stderr" >&2)
      '';

      postCheck = ''
        sleep 1 # let tee flush
        recompiled=$(grep -cE -- '-p github\.com/spf13/' "$TMPDIR/check.stderr" || true)
        echo "dependencies compiled during the check (keepTrimpathInCheck=${toString keepTrimpathInCheck}): $recompiled"
        ${
          if keepTrimpathInCheck then
            ''
              if [ "$recompiled" -ne 0 ]; then
                echo "expected the restored GOCACHE to be reused, but dependencies were recompiled"
                exit 1
              fi
            ''
          else
            ''
              if [ "$recompiled" -eq 0 ]; then
                echo "control failed: without keepTrimpathInCheck the dependencies should be recompiled"
                exit 1
              fi
            ''
        }
      '';
    };

  kept = mk true;
  dropped = mk false;
in
runCommand "keep-trimpath-in-check" { } ''
  test -f ${kept}/bin/keep-trimpath-in-check
  test -f ${dropped}/bin/keep-trimpath-in-check
  touch $out
''
