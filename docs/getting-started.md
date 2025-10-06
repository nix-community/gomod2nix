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

To speed up development and avoid downloading dependencies again in the Nix store you can import them directly from the Go cache using:
``` bash
$ gomod2nix import
```
