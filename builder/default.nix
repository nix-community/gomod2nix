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
      buildPackages
      stdenv
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
      mod ? modulesStruct.mod,
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
      ) mod;
    in
    runCommand "vendor-env"
      {
        nativeBuildInputs = [ go ];
        json = toJSON (filterAttrs (n: _: n != defaultPackage) mod);

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
      ]
      ++ nativeBuildInputs;

      inherit buildInputs;

      inherit (go) GOOS GOARCH;
      inherit CGO_ENABLED;
      GOWORK = "off";

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
              # `cachePackages` is the GLOBAL workspace union of production deps.
              # In workspace mode each module only vendors its OWN transitive
              # deps, so importing the full list breaks the build: a single
              # generated cache.go that imports an un-vendored package fails at
              # import resolution *before compiling anything*, leaving the cache
              # empty (the old `go build cache.go || true` swallowed exactly that
              # error). Instead, filter to packages actually present in THIS
              # module's vendor tree and compile them directly by import path —
              # resilient, because Go caches each package it manages to compile
              # independently of the others.
              : > cache-pkgs.txt
              for pkg in ${lib.escapeShellArgs cachePackages}; do
                if [ -d "vendor/$pkg" ]; then
                  printf '%s\n' "$pkg" >> cache-pkgs.txt
                fi
              done
              echo "Building $(wc -l < cache-pkgs.txt) of ${toString (builtins.length cachePackages)} cache packages present in this module's vendor..."

              # Compile each package INDEPENDENTLY (one `go build` per import
              # path), tolerating failures. A single `go build pkgA pkgB ...`
              # aborts during package *loading* if any one path has an
              # unresolvable import (e.g. a local test-only helper that imports a
              # non-vendored test dependency) — leaving the cache empty. Building
              # them one at a time means a bad package only drops itself; every
              # other package's compiled output still lands in $GOCACHE. Run in
              # parallel ($NIX_BUILD_CORES at a time) since Go's cache is
              # concurrency-safe and shared deps are compiled once and reused.
              xargs -r -P "''${NIX_BUILD_CORES:-1}" -I {} \
                sh -c 'go build -mod=vendor "$1" 2>/dev/null || true' _ {} \
                < cache-pkgs.txt

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
      # Pre-built GOCACHE to restore instead of computing a per-build one.
      # Lets a whole workspace share ONE cache of compiled third-party deps:
      # since the Go build cache is content-addressed (keyed by source + flags +
      # toolchain), an object compiled once is valid for every binary built with
      # the same flags. The caller is responsible for building it with matching
      # tags/ldflags/CGO/go version (see mkWorkspaceCacheEnv).
      externalCacheEnv ? null,
      workspaceModule ? null,

      ...
    }@attrs:
    let
      modulesStruct = if modules == null then { } else fromTOML (readFile modules);

      goModPath = "${toString pwd}/go.mod";

      goMod = if pwd != null && pathExists goModPath then parseGoMod (readFile goModPath) else null;

      go = selectGo attrs goMod;

      defaultPackage = modulesStruct.goPackagePath or "";

      # When workspaceModule is set, filter mod to only that module's transitive deps
      allMod = modulesStruct.mod or { };

      wsModuleInfo =
        if workspaceModule != null then
          if
            hasAttr "workspaceModules" modulesStruct
            && hasAttr workspaceModule modulesStruct.workspaceModules
          then
            modulesStruct.workspaceModules.${workspaceModule}
          else
            throw "buildGoApplication: workspaceModule '${workspaceModule}' not found in gomod2nix.toml workspaceModules. Available: ${builtins.toString (builtins.attrNames (modulesStruct.workspaceModules or { }))}"
        else
          null;

      effectiveMod =
        if wsModuleInfo != null then
          let
            allowedSet = builtins.listToAttrs (
              map (name: {
                inherit name;
                value = true;
              }) wsModuleInfo.deps
            );
          in
          filterAttrs (n: _: hasAttr n allowedSet) allMod
        else
          allMod;

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
            mod = effectiveMod;
          }
        else
          null;

      # Filter source to only dependency files for cache derivation
      # Use fetched source when building from goPackagePath
      # When pwd is set but doesn't contain go.mod (goMod == null), use src instead
      depFilesSrc =
        if defaultPackage != "" then
          vendorEnv.passthru.sources.${defaultPackage}
        else if goMod != null then
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
        if externalCacheEnv != null then
          externalCacheEnv
        else if (!disableGoCache && modulesStruct != { } && depFilesPath != null) then
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
      // removeAttrs attrs [
        "workspaceModule"
        "externalCacheEnv"
      ]
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
        GOWORK = "off";

        # Pass allowGoReference to hook for GOFLAGS configuration
        allowGoReference = if allowGoReference then "1" else "";

        goVendorDir = if vendorEnv != null then vendorEnv else "";
        goCacheDir = if cacheEnv != null then cacheEnv else "";
        inherit tags ldflags;
        modRoot =
          if attrs ? modRoot then
            attrs.modRoot
          else if attrs ? sourceRoot then
            ""
          else if wsModuleInfo != null then
            wsModuleInfo.dir
          else
            "";

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

  # Build ONE GOCACHE for an entire go.work workspace: compile every cached
  # third-party dependency a single time, against a vendor tree holding the
  # union of all modules' deps. The resulting cache is fed to every app via
  # `buildGoApplication { externalCacheEnv = ...; }`. Because the Go build cache
  # is content-addressed (keyed by source + flags + toolchain), the objects are
  # identical to what each app would have produced on its own — so a single
  # workspace cache replaces N per-app caches that each recompiled the same
  # ~4000 objects.
  #
  # Consistency requirement: tags/ldflags/CGO_ENABLED and the Go toolchain MUST
  # match what buildGoApplication uses for the apps, or the cache keys diverge
  # and nothing is reused. The Go version is derived from `referenceGoMod` with
  # the same `selectGo` logic buildGoApplication applies per app.
  mkWorkspaceCacheEnv =
    {
      modules,
      referenceGoMod,
      tags ? [ ],
      ldflags ? [ ],
      CGO_ENABLED ? null,
      allowGoReference ? false,
    }:
    let
      modulesStruct = fromTOML (readFile modules);
      refGoMod = parseGoMod (readFile referenceGoMod);
      go = selectGo { } refGoMod;

      # Vendor the union of all third-party modules. goMod = null skips local
      # replace symlinks (the workspace's own packages are never cached — their
      # source changes every build), pwd is therefore unused.
      vendorEnv = mkVendorEnv {
        inherit go modulesStruct;
        mod = modulesStruct.mod;
        goMod = null;
        pwd = null;
      };

      # Minimal synthetic module. With GO_NO_VENDOR_CHECKS=1 (set by the config
      # hook) Go resolves any package physically present in vendor/, so this
      # go.mod needs only the module + go directives — no require/go.sum.
      depFilesPath = runCommand "workspace-cache-dep-files" { } ''
        mkdir -p "$out"
        {
          echo "module inity-workspace-cache"
          echo "go ${refGoMod.go}"
        } > "$out/go.mod"
        touch "$out/go.sum"
      '';
    in
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
      CGO_ENABLED = if CGO_ENABLED != null then CGO_ENABLED else go.CGO_ENABLED;
      goMod = { replace = { }; };
    };

in
{
  inherit
    buildGoApplication
    mkGoEnv
    mkVendorEnv
    mkGoCacheEnv
    mkWorkspaceCacheEnv
    hooks
    ;
}
