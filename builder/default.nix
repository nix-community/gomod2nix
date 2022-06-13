{ stdenv
, stdenvNoCC
, runCommand
, buildEnv
, lib
, fetchgit
, removeReferencesTo
, jq
, cacert
, pkgs
}:
let

  parseGoMod = import ./parser.nix;

  removeExpr = refs: ''remove-references-to ${lib.concatMapStrings (ref: " -t ${ref}") refs}'';

  # Internal only build-time attributes
  internal =
    let
      mkInternalPkg = name: src: pkgs.runCommand "gomod2nix-${name}"
        {
          nativeBuildInputs = [ pkgs.go ];
        } ''
        export HOME=$(mktemp -d)
        cp ${src} src.go
        go build -o $out src.go
      '';
    in
    {

      # Create a symlink tree of vendored sources
      symlink = mkInternalPkg "symlink" ./symlink/symlink.go;

      # Install development dependencies from tools.go
      install = mkInternalPkg "symlink" ./install/install.go;

    };

  fetchGoModule =
    { hash
    , goPackagePath
    , version
    , go ? pkgs.go
    }:
    stdenvNoCC.mkDerivation {
      name = "${baseNameOf goPackagePath}_${version}";
      builder = ./fetch.sh;
      inherit goPackagePath version;
      nativeBuildInputs = [ go jq ];
      outputHashMode = "recursive";
      outputHashAlgo = null;
      outputHash = hash;
      SSL_CERT_FILE = "${cacert}/etc/ssl/certs/ca-bundle.crt";
      impureEnvVars = lib.fetchers.proxyImpureEnvVars ++ [ "GOPROXY" ];
    };

  mkVendorEnv = { go, modulesStruct, localReplaceCommands ? [ ] }: runCommand "vendor-env"
    {
      nativeBuildInputs = [ go ];
      json = builtins.toJSON modulesStruct;

      sources = builtins.toJSON (
        lib.mapAttrs
          (goPackagePath: meta: fetchGoModule {
            goPackagePath = meta.replaced or goPackagePath;
            inherit (meta) version hash;
            inherit go;
          })
          modulesStruct.mod
      );

      passAsFile = [ "json" "sources" ];
    }
    (
      ''
        mkdir vendor

        export GOCACHE=$TMPDIR/go-cache
        export GOPATH="$TMPDIR/go"

        ${internal.symlink}
        ${lib.concatStringsSep "\n" localReplaceCommands}

        mv vendor $out
      ''
    );

  # Select Go attribute based on version specified in go.mod
  selectGo = attrs: goMod: attrs.go or (if goMod == null then pkgs.go else
  (
    let
      goVersion = goMod.go;
      goAttr = "go_" + (lib.replaceStrings [ "." ] [ "_" ] goVersion);
    in
    (
      if builtins.hasAttr goAttr pkgs then pkgs.${goAttr}
      else builtins.trace "go.mod specified Go version ${goVersion} but doesn't exist. Falling back to ${pkgs.go.version}." pkgs.go
    )
  ));


  mkGoEnv =
    { pwd
    }@attrs:
    let
      goMod = parseGoMod (builtins.readFile "${builtins.toString pwd}/go.mod");
      modulesStruct = builtins.fromTOML (builtins.readFile "${builtins.toString pwd}/gomod2nix.toml");

      go = selectGo attrs goMod;

      vendorEnv = mkVendorEnv {
        inherit go modulesStruct;
      };

    in
    stdenv.mkDerivation {
      name = "${builtins.baseNameOf goMod.module}-env";

      dontUnpack = true;
      dontConfigure = true;
      dontInstall = true;

      propagatedNativeBuildInputs = [ go ];

      GO_NO_VENDOR_CHECKS = "1";

      GO111MODULE = "on";
      GOFLAGS = "-mod=vendor";

      preferLocalBuild = true;

      buildPhase = ''
        mkdir $out

        export GOCACHE=$TMPDIR/go-cache
        export GOPATH="$out"
        export GOSUMDB=off
        export GOPROXY=off

      '' + lib.optionalString (lib.pathExists (pwd + "/tools.go")) ''
        mkdir source
        cp ${pwd + "/go.mod"} source/go.mod
        cp ${pwd + "/go.sum"} source/go.sum
        cp ${pwd + "/tools.go"} source/tools.go
        cd source
        ln -s ${vendorEnv} vendor

        ${internal.install}
      '';
    };

  buildGoApplication =
    { modules
    , src
    , pwd ? null
    , nativeBuildInputs ? [ ]
    , allowGoReference ? false
    , meta ? { }
    , passthru ? { }
    , ...
    }@attrs:
    let
      modulesStruct = builtins.fromTOML (builtins.readFile modules);

      goMod =
        if pwd != null
        then parseGoMod (builtins.readFile "${builtins.toString pwd}/go.mod")
        else null;
      localReplaceCommands =
        let
          localReplaceAttrs = lib.filterAttrs (n: v: lib.hasAttr "path" v) goMod.replace;
          commands = (
            lib.mapAttrsToList
              (name: value: (
                ''
                  mkdir -p $(dirname vendor/${name})
                  ln -s ${pwd + "/${value.path}"} vendor/${name}
                ''
              ))
              localReplaceAttrs);
        in
        if pwd != null then commands else [ ];

      go = selectGo attrs goMod;

      removeReferences = [ ] ++ lib.optional (!allowGoReference) go;

      vendorEnv = mkVendorEnv {
        inherit go modulesStruct localReplaceCommands;
      };

      package = stdenv.mkDerivation (attrs // {
        nativeBuildInputs = [ removeReferencesTo go ] ++ nativeBuildInputs;

        inherit (go) GOOS GOARCH;

        GO_NO_VENDOR_CHECKS = "1";

        GO111MODULE = "on";
        GOFLAGS = "-mod=vendor";

        configurePhase = attrs.configurePhase or ''
          runHook preConfigure

          export GOCACHE=$TMPDIR/go-cache
          export GOPATH="$TMPDIR/go"
          export GOSUMDB=off
          export GOPROXY=off
          cd "$modRoot"
          if [ -n "${vendorEnv}" ]; then
              rm -rf vendor
              ln -s ${vendorEnv} vendor
          fi

          runHook postConfigure
        '';

        buildPhase = attrs.buildPhase or ''
          runHook preBuild

          buildGoDir() {
            local d; local cmd;
            cmd="$1"
            d="$2"
            . $TMPDIR/buildFlagsArray
            echo "$d" | grep -q "\(/_\|examples\|Godeps\|testdata\)" && return 0
            [ -n "$excludedPackages" ] && echo "$d" | grep -q "$excludedPackages" && return 0
            local OUT
            if ! OUT="$(go $cmd $buildFlags "''${buildFlagsArray[@]}" -v -p $NIX_BUILD_CORES $d 2>&1)"; then
              if echo "$OUT" | grep -qE 'imports .*?: no Go files in'; then
                echo "$OUT" >&2
                return 1
              fi
              if ! echo "$OUT" | grep -qE '(no( buildable| non-test)?|build constraints exclude all) Go (source )?files'; then
                echo "$OUT" >&2
                return 1
              fi
            fi
            if [ -n "$OUT" ]; then
              echo "$OUT" >&2
            fi
            return 0
          }

          getGoDirs() {
            local type;
            type="$1"
            if [ -n "$subPackages" ]; then
              echo "$subPackages" | sed "s,\(^\| \),\1./,g"
            else
              find . -type f -name \*$type.go -exec dirname {} \; | grep -v "/vendor/" | sort --unique
            fi
          }

          if (( "''${NIX_DEBUG:-0}" >= 1 )); then
            buildFlagsArray+=(-x)
          fi

          if [ ''${#buildFlagsArray[@]} -ne 0 ]; then
            declare -p buildFlagsArray > $TMPDIR/buildFlagsArray
          else
            touch $TMPDIR/buildFlagsArray
          fi
          if [ -z "$enableParallelBuilding" ]; then
              export NIX_BUILD_CORES=1
          fi
          for pkg in $(getGoDirs ""); do
            echo "Building subPackage $pkg"
            buildGoDir install "$pkg"
          done
        '' + lib.optionalString (stdenv.hostPlatform != stdenv.buildPlatform) ''
          # normalize cross-compiled builds w.r.t. native builds
          (
            dir=$GOPATH/bin/${go.GOOS}_${go.GOARCH}
            if [[ -n "$(shopt -s nullglob; echo $dir/*)" ]]; then
              mv $dir/* $dir/..
            fi
            if [[ -d $dir ]]; then
              rmdir $dir
            fi
          )
        '' + ''
          runHook postBuild
        '';

        doCheck = attrs.doCheck or true;
        checkPhase = attrs.checkPhase or ''
          runHook preCheck

          for pkg in $(getGoDirs test); do
            buildGoDir test "$pkg"
          done

          runHook postCheck
        '';

        installPhase = attrs.installPhase or ''
          runHook preInstall

          mkdir -p $out
          dir="$GOPATH/bin"
          [ -e "$dir" ] && cp -r $dir $out

          runHook postInstall
        '';

        preFixup = (attrs.preFixup or "") + ''
          find $out/{bin,libexec,lib} -type f 2>/dev/null | xargs -r ${removeExpr removeReferences} || true
        '';

        strictDeps = true;

        disallowedReferences = lib.optional (!allowGoReference) go;

        passthru = passthru // { inherit go vendorEnv; };

        meta = { platforms = go.meta.platforms or lib.platforms.all; } // meta;
      });

    in
    package;

in
{
  inherit buildGoApplication mkGoEnv;
}
