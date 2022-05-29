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
  inherit (builtins) split elemAt filter typeOf;

  nixVersion = builtins.substring 0 3 builtins.nixVersion;
  isNix24Plus = lib.versionAtLeast nixVersion "2.4";

  parseGoMod = import ./parser.nix;
  parseVersion = import ./parse-version.nix;

  removeExpr = refs: ''remove-references-to ${lib.concatMapStrings (ref: " -t ${ref}") refs}'';

  fetchGoModule = (
    lib.makeOverridable (
      { hash
      , goPackagePath
      , version
      , private ? false
      , go ? pkgs.go
      }:
      if private then
        fetchGoModulePrivate
          {
            inherit goPackagePath version;
          } else
        stdenvNoCC.mkDerivation {
          name = "${baseNameOf goPackagePath}_${version}";
          builder = ./fetch.sh;
          inherit goPackagePath version;
          nativeBuildInputs = [ go jq ];
          outputHashMode = "recursive";
          outputHashAlgo = null;
          outputHash = hash;
          SSL_CERT_FILE = "${cacert}/etc/ssl/certs/ca-bundle.crt";
          impureEnvVars = lib.fetchers.proxyImpureEnvVars;
        }
    )
  );

  # A "best effort" attempt at generalising fetching private repositories
  # It's very likely that more advanced use cases needs to be done manually
  # and that we'll need to have some UX for that.
  #
  # This version works for popular forges such as Github and Gitlab.
  fetchGoModulePrivate =
    { goPackagePath
    , version
    }:
    let
      parsedVersion = parseVersion version;

      segments = filter (s: typeOf s != "list") (split "/" goPackagePath);
      seg = elemAt segments;
      domain = seg 0;

      url = "git@${domain}:${seg 1}/${seg 2}.git";
      sourceRoot = lib.concatStringsSep "/" (lib.drop 3 segments);

      src = builtins.fetchGit
        {
          inherit url;
        } // lib.optionalAttrs isNix24Plus {
        allRefs = true;
      } // lib.optionalAttrs (parsedVersion.rev != "") {
        # Nix has a bug handling short revs so this won't work.
        inherit (parsedVersion) rev;
      } // lib.optionalAttrs (parsedVersion.version != "v0.0.0") {
        ref = "refs/tags/${parsedVersion.version}";
      };

    in
    if sourceRoot != "" then
      stdenvNoCC.mkDerivation
        {
          name = "${baseNameOf goPackagePath}_${version}-wrapper";
          inherit src;
          dontConfigure = true;
          dontBuild = true;
          dontFixup = true;
          installPhase = ''
            cd "${sourceRoot}"
            cp -a . $out
          '';
        } else src;

  buildGoApplication =
    { modules
    , go ? pkgs.go
    , src
    , pwd ? null
    , CGO_ENABLED ? "0"
    , nativeBuildInputs ? [ ]
    , allowGoReference ? false
    , meta ? { }
    , passthru ? { }
    , srcOverrides ? self: super: { }
    , ...
    }@attrs:
    let
      modulesStruct = builtins.fromTOML (builtins.readFile modules);

      goMod = parseGoMod (builtins.readFile "${builtins.toString pwd}/go.mod");
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

      vendorEnv = runCommand "vendor-env"
        {
          nativeBuildInputs = [ go ];
          json = builtins.toJSON modulesStruct;

          sources = builtins.toJSON (builtins.removeAttrs
            ((lib.makeExtensible (self: (
              lib.mapAttrs
                (goPackagePath: meta: fetchGoModule {
                  goPackagePath = meta.replaced or goPackagePath;
                  inherit (meta) version hash;
                  inherit go;
                })
                modulesStruct.mod
            ))).extend srcOverrides) [ "extend" "__unfix__" ]);

          passAsFile = [ "json" "sources" ];
        }
        (
          ''
            mkdir vendor

            export GOCACHE=$TMPDIR/go-cache
            export GOPATH="$TMPDIR/go"

            go run ${./symlink.go}
            ${lib.concatStringsSep "\n" localReplaceCommands}

            mv vendor $out
          ''
        );

      removeReferences = [ ] ++ lib.optional (!allowGoReference) go;

      package = stdenv.mkDerivation (attrs // {
        nativeBuildInputs = [ removeReferencesTo go ] ++ nativeBuildInputs;

        inherit (go) GOOS GOARCH;
        inherit CGO_ENABLED;

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
buildGoApplication
