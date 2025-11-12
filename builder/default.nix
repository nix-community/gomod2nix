{
  buildEnv,
  buildPackages,
  cacert,
  fetchgit,
  git,
  gnutar,
  gomod2nix,
  jq,
  lib,
  makeSetupHook,
  pkgsBuildBuild,
  rsync,
  runCommand,
  runtimeShell,
  stdenv,
  stdenvNoCC,
  writeScript,
  zstd,
}:
let

  hooks = import ./hooks/default.nix {
    inherit
      lib
      makeSetupHook
      rsync
      stdenv
      gnutar
      zstd
      ;
  };

  inherit (hooks)
    goConfigHook
    goBuildHook
    goCheckHook
    goInstallHook
    ;

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

      # Generate dummy import file for cache warming
      cachegen = mkInternalPkg "cachegen" ./cachegen/cachegen.go;
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

  mkGoCacheEnv =
    {
      go,
      modulesStruct,
      goMod,
      vendorEnv,
      depFilesPath,
      # Build environment parameters (should match buildGoApplication)
      nativeBuildInputs ? [ ],
      buildInputs ? [ ],
      CGO_ENABLED ? go.CGO_ENABLED,
      tags ? [ ],
      ldflags ? [ ],
      allowGoReference ? false,
    }:
    let
      # Check if cachePackages is defined in modulesStruct
      cachePackages = modulesStruct.cachePackages or [ ];
      hasCachePackages = cachePackages != [ ];
    in
    stdenv.mkDerivation {
      name = "go-cache-env";

      dontUnpack = true;

      nativeBuildInputs = [
        rsync
        go
        goConfigHook
        gnutar
        zstd
        # goConfigHook
      ]
      ++ nativeBuildInputs;

      inherit buildInputs;

      inherit (go) GOOS GOARCH;
      inherit CGO_ENABLED;

      # Pass allowGoReference to hook for GOFLAGS configuration
      allowGoReference = if allowGoReference then "1" else "";

      # Pass tags and ldflags (used by hooks)
      inherit tags ldflags;

      goVendorDir = vendorEnv;

      # Change the working directory in prePatch so GoConfigHook sets up
      # vendor/ at the right location
      prePatch = ''
        # Create a working directory (Go ignores go.mod in /build)
        mkdir -p source
        cd source

        # Copy go.mod and go.sum from filtered source
        cp ${depFilesPath}/go.mod ./go.mod
        cp ${depFilesPath}/go.sum ./go.sum 2>/dev/null || touch go.sum
      '';

      configurePhase = ''
        # Set up GOCACHE directory (will compress to $out later)
        mkdir -p "$GOCACHE"
      '';

      buildPhase = ''
        runHook preBuild

        ${
          if hasCachePackages then
            ''
              echo "Building ${toString (builtins.length cachePackages)} packages to populate cache..."

              # Generate cache.go that imports all packages
              printf '%s\n' ${lib.escapeShellArgs cachePackages} | ${internal.cachegen} > cache.go

              cat cache.go

              # Build cache.go - Go will build all dependencies using its scheduler
              go build -v -mod=vendor cache.go || true

              echo "Cache population complete"
            ''
          else
            ''
              echo "No cache packages defined, skipping cache population"
            ''
        }

        runHook postBuild
      '';

      installPhase = ''
        runHook preInstall

        echo "Compressing Go build cache..."
        mkdir -p "$out"
        tar -cf - -C "$GOCACHE" . | zstd -T$NIX_BUILD_CORES -o "$out/cache.tar.zst"

        echo "Cache compressed to $out/cache.tar.zst"

        runHook postInstall
      '';
    };

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
      allowGoReference ? false,
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
      removeAttrs attrs [
        "pwd"
        "allowGoReference"
      ]
      // {
        name = "${baseNameOf goMod.module}-env";

        dontUnpack = true;
        dontConfigure = true;
        dontInstall = true;

        CGO_ENABLED = attrs.CGO_ENABLED or go.CGO_ENABLED;

        # Pass allowGoReference to hook for GOFLAGS configuration
        allowGoReference = if allowGoReference then "1" else "";

        nativeBuildInputs = [
          rsync
          goConfigHook
        ];

        propagatedBuildInputs = [ go ];

        # Pass vendor directory to the setup hook
        goVendorDir = vendorEnv;

        preferLocalBuild = true;

        buildPhase = ''
          mkdir $out

          export GOPATH="$out"

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
      disableGoCache ? false,

      ...
    }@attrs:
    let
      modulesStruct = if modules == null then { } else fromTOML (readFile modules);

      goModPath = "${toString pwd}/go.mod";

      goMod = if pwd != null && pathExists goModPath then parseGoMod (readFile goModPath) else null;

      go = selectGo attrs goMod;

      defaultPackage = modulesStruct.goPackagePath or "";

      vendorEnv =
        if modulesStruct != { } then
          mkVendorEnv {
            inherit
              defaultPackage
              go
              goMod
              modulesStruct
              pwd
              ;
          }
        else
          null;

      # Filter source to only dependency files for cache derivation
      # Use fetched source when building from goPackagePath
      depFilesSrc =
        if defaultPackage != "" then
          vendorEnv.passthru.sources.${defaultPackage}
        else if pwd != null then
          pwd
        else
          src;

      depFilesPath =
        if (!disableGoCache && modulesStruct != { } && depFilesSrc != null) then
          lib.cleanSourceWith {
            src = depFilesSrc;
            filter =
              path: type:
              let
                baseName = baseNameOf path;
              in
              baseName == "go.mod" || baseName == "go.sum" || baseName == "gomod2nix.toml";
            name = "go-dep-files";
          }
        else
          null;

      cacheEnv =
        if (!disableGoCache && modulesStruct != { } && depFilesPath != null) then
          mkGoCacheEnv {
            inherit
              go
              modulesStruct
              vendorEnv
              depFilesPath
              tags
              ldflags
              allowGoReference
              ;
            CGO_ENABLED = attrs.CGO_ENABLED or go.CGO_ENABLED;
            goMod = if goMod != null then goMod else { replace = { }; };
          }
        else
          null;

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
          go
          goConfigHook
          goBuildHook
          goCheckHook
          goInstallHook
        ]
        ++ nativeBuildInputs;

        inherit (go) GOOS GOARCH;

        CGO_ENABLED = attrs.CGO_ENABLED or go.CGO_ENABLED;

        # Pass allowGoReference to hook for GOFLAGS configuration
        allowGoReference = if allowGoReference then "1" else "";

        goVendorDir = if vendorEnv != null then vendorEnv else "";
        goCacheDir = if cacheEnv != null then cacheEnv else "";
        inherit tags ldflags;
        modRoot = attrs.modRoot or "";

        doCheck = attrs.doCheck or true;

        strictDeps = true;

        disallowedReferences = optional (!allowGoReference) go;

        passthru = {
          inherit go vendorEnv hooks;
          goCacheEnv = cacheEnv;
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
  inherit
    buildGoApplication
    mkGoEnv
    mkVendorEnv
    mkGoCacheEnv
    hooks
    ;
}
