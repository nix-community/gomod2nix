{ stdenv
, stdenvNoCC
, runCommand
, buildEnv
, lib
, fetchgit
, jq
, cacert
, pkgs
, pkgsBuildBuild
}:
let

  inherit (builtins) substring toJSON hasAttr trace split readFile elemAt;
  inherit (lib)
    concatStringsSep replaceStrings removePrefix optionalString pathExists
    optional concatMapStrings fetchers filterAttrs mapAttrs mapAttrsToList
    warnIf optionalAttrs platforms
    ;

  parseGoMod = import ./parser.nix;

  removeExpr = refs: ''remove-references-to ${concatMapStrings (ref: " -t ${ref}") refs}'';

  # Internal only build-time attributes
  internal =
    let
      mkInternalPkg = name: src: pkgsBuildBuild.runCommand "gomod2nix-${name}"
        {
          inherit (pkgsBuildBuild.go) GOOS GOARCH;
          nativeBuildInputs = [ pkgsBuildBuild.go ];
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
      nativeBuildInputs = [
        go
        jq
        cacert
      ];
      outputHashMode = "recursive";
      outputHashAlgo = null;
      outputHash = hash;
      impureEnvVars = fetchers.proxyImpureEnvVars ++ [ "GOPROXY" ];
    };

  mkVendorEnv =
    { go
    , modulesStruct
    , localReplaceCommands ? [ ]
    , defaultPackage ? ""
    , goMod
    , pwd
    }:
    let
      localReplaceCommands =
        let
          localReplaceAttrs = filterAttrs (n: v: hasAttr "path" v) goMod.replace;
          commands = (
            mapAttrsToList
              (name: value: (
                ''
                  mkdir -p $(dirname vendor/${name})
                  ln -s ${pwd + "/${value.path}"} vendor/${name}
                ''
              ))
              localReplaceAttrs);
        in
        if goMod != null then commands else [ ];

      sources = mapAttrs
        (goPackagePath: meta: fetchGoModule {
          goPackagePath = meta.replaced or goPackagePath;
          inherit (meta) version hash;
          inherit go;
        })
        modulesStruct.mod;
    in
    runCommand "vendor-env"
      {
        nativeBuildInputs = [ go ];
        json = toJSON (filterAttrs (n: _: n != defaultPackage) modulesStruct.mod);

        sources = toJSON (filterAttrs (n: _: n != defaultPackage) sources);

        passthru = {
          inherit sources;
        };

        passAsFile = [ "json" "sources" ];
      }
      (
        ''
          mkdir vendor

          export GOCACHE=$TMPDIR/go-cache
          export GOPATH="$TMPDIR/go"

          ${internal.symlink}
          ${concatStringsSep "\n" localReplaceCommands}

          mv vendor $out
        ''
      );

  # Select Go attribute based on version specified in go.mod
  selectGo = attrs: goMod: attrs.go or (if goMod == null then pkgs.go else
  (
    let
      goVersion = goMod.go;
      goAttr = "go_" + (replaceStrings [ "." ] [ "_" ] goVersion);
    in
    (
      if hasAttr goAttr pkgs then pkgs.${goAttr}
      else trace "go.mod specified Go version ${goVersion} but doesn't exist. Falling back to ${pkgs.go.version}." pkgs.go
    )
  ));

  # Strip the rubbish that Go adds to versions, and fall back to a version based on the date if it's a placeholder value
  stripVersion = version:
    let
      parts = elemAt (split "(\\+|-)" (removePrefix "v" version));
      v = parts 0;
      d = parts 2;
    in
    if v != "0.0.0" then v else "unstable-" + (concatStringsSep "-" [
      (substring 0 4 d)
      (substring 4 2 d)
      (substring 6 2 d)
    ]);

  mkGoEnv =
    { pwd
    }@attrs:
    let
      goMod = parseGoMod (readFile "${toString pwd}/go.mod");
      modulesStruct = fromTOML (readFile "${toString pwd}/gomod2nix.toml");

      go = selectGo attrs goMod;

      vendorEnv = mkVendorEnv {
        inherit go modulesStruct pwd goMod;
      };

    in
    stdenv.mkDerivation (removeAttrs attrs [ "pwd" ] // {
      name = "${baseNameOf goMod.module}-env";

      dontUnpack = true;
      dontConfigure = true;
      dontInstall = true;

      propagatedBuildInputs = [ go ];

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

      '' + optionalString (pathExists (pwd + "/tools.go")) ''
        mkdir source
        cp ${pwd + "/go.mod"} source/go.mod
        cp ${pwd + "/go.sum"} source/go.sum
        cp ${pwd + "/tools.go"} source/tools.go
        cd source
        cp -r ${vendorEnv} vendor

        ${internal.install}
      '';
    });

  buildGoApplication =
    { modules ? pwd + "/gomod2nix.toml"
    , src ? pwd
    , pwd ? null
    , nativeBuildInputs ? [ ]
    , allowGoReference ? false
    , meta ? { }
    , passthru ? { }
    , tags ? [ ]

      # needed for buildFlags{,Array} warning
    , buildFlags ? ""
    , buildFlagsArray ? ""

    , ...
    }@attrs:
    let
      modulesStruct = fromTOML (readFile modules);

      goModPath = "${toString pwd}/go.mod";

      goMod =
        if pwd != null && pathExists goModPath
        then parseGoMod (readFile goModPath)
        else null;

      go = selectGo attrs goMod;

      defaultPackage = modulesStruct.goPackagePath or "";

      vendorEnv = mkVendorEnv {
        inherit go modulesStruct defaultPackage goMod pwd;
      };

    in
    warnIf (buildFlags != "" || buildFlagsArray != "")
      "Use the `ldflags` and/or `tags` attributes instead of `buildFlags`/`buildFlagsArray`"
      stdenv.mkDerivation
      (optionalAttrs (defaultPackage != "")
        {
          pname = attrs.pname or baseNameOf defaultPackage;
          version = stripVersion (modulesStruct.mod.${defaultPackage}).version;
          src = vendorEnv.passthru.sources.${defaultPackage};
        } // optionalAttrs (hasAttr "subPackages" modulesStruct) {
        subPackages = modulesStruct.subPackages;
      } // attrs // {
        nativeBuildInputs = [ go ] ++ nativeBuildInputs;

        inherit (go) GOOS GOARCH;

        GO_NO_VENDOR_CHECKS = "1";

        GO111MODULE = "on";
        GOFLAGS = [ "-mod=vendor" ] ++ lib.optionals (!allowGoReference) [ "-trimpath" ];

        configurePhase = attrs.configurePhase or ''
          runHook preConfigure

          export GOCACHE=$TMPDIR/go-cache
          export GOPATH="$TMPDIR/go"
          export GOSUMDB=off
          export GOPROXY=off
          cd "$modRoot"
          if [ -n "${vendorEnv}" ]; then
              rm -rf vendor
              cp -r ${vendorEnv} vendor
          fi

          runHook postConfigure
        '';

        buildPhase = attrs.buildPhase or ''
          runHook preBuild

          exclude='\(/_\|examples\|Godeps\|testdata'
          if [[ -n "$excludedPackages" ]]; then
            IFS=' ' read -r -a excludedArr <<<$excludedPackages
            printf -v excludedAlternates '%s\\|' "''${excludedArr[@]}"
            excludedAlternates=''${excludedAlternates%\\|} # drop final \| added by printf
            exclude+='\|'"$excludedAlternates"
          fi
          exclude+='\)'

          buildGoDir() {
            local cmd="$1" dir="$2"

            . $TMPDIR/buildFlagsArray

            declare -a flags
            flags+=($buildFlags "''${buildFlagsArray[@]}")
            flags+=(''${tags:+-tags=${lib.concatStringsSep "," tags}})
            flags+=(''${ldflags:+-ldflags="$ldflags"})
            flags+=("-v" "-p" "$NIX_BUILD_CORES")

            if [ "$cmd" = "test" ]; then
              flags+=($checkFlags)
            fi

            local OUT
            if ! OUT="$(go $cmd "''${flags[@]}" $dir 2>&1)"; then
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
              find . -type f -name \*$type.go -exec dirname {} \; | grep -v "/vendor/" | sort --unique | grep -v "$exclude"
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
        '' + optionalString (stdenv.hostPlatform != stdenv.buildPlatform) ''
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

          # We do not set trimpath for tests, in case they reference test assets
          export GOFLAGS=''${GOFLAGS//-trimpath/}

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

        strictDeps = true;

        disallowedReferences = optional (!allowGoReference) go;

        passthru = { inherit go vendorEnv; } // passthru;

        meta = { platforms = go.meta.platforms or platforms.all; } // meta;
      });

in
{
  inherit buildGoApplication mkGoEnv;
}
