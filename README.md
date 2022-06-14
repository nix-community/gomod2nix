# Gomod2nix
Convert applications using Go modules -> Nix

## Usage
From the Go project directory execute:
``` bash
$ gomod2nix
```

This will create `gomod2nix.toml` that's used like so
``` nix
let
  pkgs = import <nixpkgs> {
    overlays = [
      (self: super: {
        buildGoApplication = super.callPackage ./builder { };
      })
    ];
  };
in pkgs.buildGoApplication {
  pname = "gomod2nix-example";
  version = "0.1";
  src = ./.;
  modules = ./gomod2nix.toml;
}
```

For more in-depth usage check the [Getting Started](./docs/getting-started.md) docs.

## FAQ

### Why not continue work on vgo2nix?
Vgo2nix was built on top of the old Nixpkgs build abstraction `buildGoPackage`, this abstraction was built pre-modules and suffered from some fundamental design issues with modules, such as only allowing a single version of a Go package path inside the same build closure, something that Go itself allows for.

We need a better build abstraction that takes Go modules into account, while remaining [import from derivation](https://nixos.wiki/wiki/Import_From_Derivation)-free.

### Will this be included in Nixpkgs

Yes. Once the API is considered stable.

## Motivation

The [announcement blog post](https://www.tweag.io/blog/2021-03-04-gomod2nix/) contains comparisons with other Go build systems for Nix and additional notes on the design choices made.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE)
file for details.
