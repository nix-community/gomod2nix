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

For more in-depth usage check the [Getting Started](./docs/getting-started.md) and the [Nix API reference](./docs/nix-reference.md) docs.

## Motivation

The [announcement blog post](https://www.tweag.io/blog/2021-03-04-gomod2nix/) contains comparisons with other Go build systems for Nix and additional notes on the design choices made.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE)
file for details.

## About the project
The developmentent of Trustix (which Gomod2nix is a part of) has been sponsored by [Tweag I/O](https://tweag.io/) and funded by the [NLNet foundation](https://nlnet.nl/project/Trustix) and the European Commission’s [Next Generation Internet programme](https://www.ngi.eu/funded_solution/trustix-nix/) through the NGI Zero PET (privacy and trust enhancing technologies) fund.

![NGI0 logo](./.assets/NGI0_tag.png)
![NLNet banner](./.assets/nlnet-banner.png)
![Tweag logo](./.assets/tweag.png)
