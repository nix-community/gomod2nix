{
  buildEnv,
  buildPackages,
  cacert,
  fetchgit,
  git,
  gomod2nix,
  jq,
  lib,
  pkgsBuildBuild,
  rsync,
  runCommand,
  runtimeShell,
  stdenv,
  stdenvNoCC,
  writeScript,
}:
let

  inherit (builtins)
    elemAt
    hasAttr
    readFile
    split
    substring
    toJSON
    ;
  inherit (lib)
    concatStringsSep
    fetchers
    filterAttrs
    mapAttrs
    mapAttrsToList
    optional
    optionalAttrs
    optionalString
    pathExists
    removePrefix
    ;

  parseGoMod = import ./parser.nix;

  # Internal only build-time attributes
  internal =
    let
      mkInternalPkg =
        name: src:
        pkgsBuildBuild.runCommand "gomod2nix-${name}"
          {
            inherit (pkgsBuildBuild.go) GOOS GOARCH;
            nativeBuildInputs = [ pkgsBuildBuild.go ];
          }
          ''
            export HOME=$(mktemp -d)
            go build -o "$HOME/bin" ${src}
            mv "$HOME/bin" "$out"
          '';
    in
    {
      # Create a symlink tree of vendored sources
      symlink = mkInternalPkg "symlink" ./symlink/symlink.go;

      # Install development dependencies from tools.go
      install = mkInternalPkg "symlink" ./install/install.go;
    };

  fetchGoModule =
    {
      hash,
      goPackagePath,
      version,
      go,
    }:
    stdenvNoCC.mkDerivation {
      name = "${baseNameOf goPackagePath}_${version}";
      builder = ./fetch.sh;
      inherit goPackagePath version;
      nativeBuildInputs = [
        cacert
        git
        go
        jq
      ];
      outputHashMode = "recursive";
      outputHashAlgo = null;
      outputHash = hash;
      impureEnvVars = fetchers.proxyImpureEnvVars ++ [ "GOPROXY" ];
    };

  mkVendorEnv =
    {
      go,
      modulesStruct,
      localReplaceCommands ? [ ],
      defaultPackage ? "",
      goMod,
      pwd,
    }:
    let
      localReplaceCommands =
        let
          localReplaceAttrs = filterAttrs (n: v: hasAttr "path" v) goMod.replace;
          commands = (
            mapAttrsToList (name: value: (''
              mkdir -p $(dirname vendor/${name})
              ln -s ${pwd + "/${value.path}"} vendor/${name}
            '')) localReplaceAttrs
          );
        in
        if goMod != null then commands else [ ];

      sources = mapAttrs (
        goPackagePath: meta:
        fetchGoModule {
          goPackagePath = meta.replaced or goPackagePath;
          inherit (meta) version hash;
          inherit go;
        }
      ) modulesStruct.mod;
    in
    runCommand "vendor-env"
      {
        nativeBuildInputs = [ go ];
        json = toJSON (filterAttrs (n: _: n != defaultPackage) modulesStruct.mod);

        sources = toJSON (filterAttrs (n: _: n != defaultPackage) sources);

        passthru = {
          inherit sources;
        };

        passAsFile = [
          "json"
          "sources"
        ];
      }
      (''
        mkdir vendor

        export GOCACHE=$TMPDIR/go-cache
        export GOPATH="$TMPDIR/go"

        ${internal.symlink}
        ${concatStringsSep "\n" localReplaceCommands}

        mv vendor $out
      '');

  # Return a Go attribute and error out if the Go version is older than was specified in go.mod.
  selectGo =
    attrs: goMod:
    attrs.go or (
      if goMod == null then
        buildPackages.go
      else
        (
          let
            goVersion = goMod.go;
            goAttrs = lib.reverseList (
              builtins.filter (
                attr:
                lib.hasPrefix "go_" attr
                && (
                  let
                    try = builtins.tryEval buildPackages.${attr};
                  in
                  try.success && try.value ? version
                )
                && lib.versionAtLeast buildPackages.${attr}.version goVersion
              ) (lib.attrNames buildPackages)
            );
            goAttr = elemAt goAttrs 0;
          in
          (
            if goAttrs != [ ] then
              buildPackages.${goAttr}
            else
              throw "go.mod specified Go version ${goVersion}, but no compatible Go attribute could be found."
          )
        )
    );

  # Strip extra data that Go adds to versions, and fall back to a version based on the date if it's a placeholder value.
  # This is data that Nix can't handle in the version attribute.
  stripVersion =
    version:
    let
      parts = elemAt (split "(\\+|-)" (removePrefix "v" version));
      v = parts 0;
      d = parts 2;
    in
    if v != "0.0.0" then
      v
    else
      "unstable-"
      + (concatStringsSep "-" [
        (substring 0 4 d)
        (substring 4 2 d)
        (substring 6 2 d)
      ]);

  mkGoEnv =
    {
      pwd,
      toolsGo ? pwd + "/tools.go",
      modules ? pwd + "/gomod2nix.toml",
      ...
    }@attrs:
    let
      goMod = parseGoMod (readFile "${toString pwd}/go.mod");
      modulesStruct = fromTOML (readFile modules);

      go = selectGo attrs goMod;

      vendorEnv = mkVendorEnv {
        inherit
          go
          goMod
          modulesStruct
          pwd
          ;
      };

    in
    stdenv.mkDerivation (
      removeAttrs attrs [ "pwd" ]
      // {
        name = "${baseNameOf goMod.module}-env";

        dontUnpack = true;
        dontConfigure = true;
        dontInstall = true;

        CGO_ENABLED = attrs.CGO_ENABLED or go.CGO_ENABLED;

        nativeBuildInputs = [ rsync ];

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

        ''
        + optionalString (pathExists toolsGo) ''
          mkdir source
          cp ${pwd + "/go.mod"} source/go.mod
          cp ${pwd + "/go.sum"} source/go.sum
          cp ${toolsGo} source/tools.go
          cd source

          rsync -a -K --ignore-errors ${vendorEnv}/ vendor

          ${internal.install}
        '';
      }
    );

  buildGoApplication =
    {
      modules ? pwd + "/gomod2nix.toml",
      src ? pwd,
      pwd ? null,
      nativeBuildInputs ? [ ],
      allowGoReference ? false,
      meta ? { },
      passthru ? { },
      tags ? [ ],
      ldflags ? [ ],

      ...
    }@attrs:
    let
      modulesStruct = if modules == null then { } else fromTOML (readFile modules);

      goModPath = "${toString pwd}/go.mod";

      goMod = if pwd != null && pathExists goModPath then parseGoMod (readFile goModPath) else null;

      go = selectGo attrs goMod;

      defaultPackage = modulesStruct.goPackagePath or "";

      vendorEnv = mkVendorEnv {
        inherit
          defaultPackage
          go
          goMod
          modulesStruct
          pwd
          ;
      };

      pname = attrs.pname or baseNameOf defaultPackage;

    in
    stdenv.mkDerivation (
      optionalAttrs (defaultPackage != "") {
        inherit pname;
        version = stripVersion (modulesStruct.mod.${defaultPackage}).version;
        src = vendorEnv.passthru.sources.${defaultPackage};
      }
      // optionalAttrs (hasAttr "subPackages" modulesStruct) {
        subPackages = modulesStruct.subPackages;
      }
      // attrs
      // {
        nativeBuildInputs = [
          rsync
          go
        ]
        ++ nativeBuildInputs;

        inherit (go) GOOS GOARCH;

        GO_NO_VENDOR_CHECKS = "1";
        CGO_ENABLED = attrs.CGO_ENABLED or go.CGO_ENABLED;

        GO111MODULE = "on";
        GOFLAGS = [ "-mod=vendor" ] ++ lib.optionals (!allowGoReference) [ "-trimpath" ];

        configurePhase =
          attrs.configurePhase or ''
            runHook preConfigure

            export GOCACHE=$TMPDIR/go-cache
            export GOPATH="$TMPDIR/go"
            export GOSUMDB=off
            export GOPROXY=off
            cd "''${modRoot:-.}"

            ${optionalString (modulesStruct != { }) ''
              if [ -n "${vendorEnv}" ]; then
                rm -rf vendor
                rsync -a -K --ignore-errors ${vendorEnv}/ vendor
              fi
            ''}

            runHook postConfigure
          '';

        buildPhase =
          attrs.buildPhase or ''
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
                flags+=(-vet=off)
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
          ''
          + optionalString (stdenv.hostPlatform != stdenv.buildPlatform) ''
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
          ''
          + ''
            runHook postBuild
          '';

        doCheck = attrs.doCheck or true;
        checkPhase =
          attrs.checkPhase or ''
            runHook preCheck

            # We do not set trimpath for tests, in case they reference test assets
            export GOFLAGS=''${GOFLAGS//-trimpath/}

            for pkg in $(getGoDirs test); do
              buildGoDir test "$pkg"
            done

            runHook postCheck
          '';

        installPhase =
          attrs.installPhase or ''
            runHook preInstall

            mkdir -p $out
            dir="$GOPATH/bin"
            [ -e "$dir" ] && cp -r $dir $out

            runHook postInstall
          '';

        strictDeps = true;

        disallowedReferences = optional (!allowGoReference) go;

        passthru = {
          inherit go vendorEnv;
        }
        // optionalAttrs (hasAttr "goPackagePath" modulesStruct) {

          updateScript =
            let
              generatorArgs =
                if hasAttr "subPackages" modulesStruct then
                  concatStringsSep " " (
                    map (subPackage: modulesStruct.goPackagePath + "/" + subPackage) modulesStruct.subPackages
                  )
                else
                  modulesStruct.goPackagePath;

            in
            writeScript "${pname}-updater" ''
              #!${runtimeShell}
              ${optionalString (pwd != null) "cd ${toString pwd}"}
              exec ${gomod2nix}/bin/gomod2nix generate ${generatorArgs}
            '';

        }
        // passthru;

        inherit meta;
      }
    );

in
{
  inherit buildGoApplication mkGoEnv;
}
