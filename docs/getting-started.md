# Getting started with Gomod2nix

## Installation

### Using Niv

First initialize Niv:
``` bash
$ niv init --latest
$ niv add nix-community/gomod2nix
```

Create a `shell.nix` used for development:
``` nix
{ pkgs ? (
    let
      sources = import ./nix/sources.nix;
    in
    import sources.nixpkgs {
      overlays = [
        (import "${sources.gomod2nix}/overlay.nix")
      ];
    }
  )
}:

let
  goEnv = pkgs.mkGoEnv { pwd = ./.; };
in
pkgs.mkShell {
  packages = [
    goEnv
    pkgs.gomod2nix
    pkgs.niv
  ];
}
```

And a `default.nix` for building your package
``` nix
{ pkgs ? (
    let
      sources = import ./nix/sources.nix;
    in
    import sources.nixpkgs {
      overlays = [
        (import "${sources.gomod2nix}/overlay.nix")
      ];
    }
  )
}:

pkgs.buildGoApplication {
  pname = "myapp";
  version = "0.1";
  pwd = ./.;
  src = ./.;
  modules = ./gomod2nix.toml;
}
```

### Using Flakes

The quickest way to get started if using Nix Flakes is to use the Flake template:
``` bash
$ nix flake init -t github:nix-community/gomod2nix#app
```
It is also possible to use the container template to build container images:
```bash
$ nix flake init -t github:nix-community/gomod2nix#container
```

## Basic usage

After you have entered your development shell you can generate a `gomod2nix.toml` using:
``` bash
$ gomod2nix generate
```

To optimize build performance by pre-compiling dependencies in the build cache, use:
``` bash
$ gomod2nix generate --with-deps
```

This generates a list of imported packages that will be pre-compiled during the build, significantly reducing subsequent build times. This is particularly useful for projects with many dependencies.

To speed up development and avoid downloading dependencies again in the Nix store you can import them directly from the Go cache using:
``` bash
$ gomod2nix import
```

## Cross Compilation

With `gomodwnix` added to your `pkgs` via a `nixpkgs` overlay, cross compilation should "just work". To do that you'd have to use Nixpkgs' `pkgsCross.` and `callPackage` splicing. To do that, write a `.nix` file with `buildGoApplication` as an argument (so `callPackage` will splice it):

```nix
# example `package.nix`
{
  lib,
  buildGoApplication,
}:

buildGoApplication {
  pname = "myapp";
  version = "0.1";
  pwd = ./.;
  src = ./.;
  modules = ./gomod2nix.toml;
}
```

Then, if you'd use `pkgsCross.<cross-system>.callPackage ./package.nix { }`, with a target `<cross-system>` (e.g `aarch64-multiplatform`), `gomod2nix` will cross compile your project.
Note that `pkgsCross.<cross-system>.pkgsStatic.callPackage` should also work.
If you use a `flake.nix` for your project, simply define multiple `packages` attributes such as `myapp-linux-aarch64`/`myapp-linux-armv7` each defined with the appropriate `callPackage`.
