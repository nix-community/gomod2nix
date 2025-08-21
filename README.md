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

## Updating gomod2nix

```
[das@t:~/Downloads/gomod2nix]$ go get -u ./...
go: downloading github.com/spf13/cobra v1.9.1
go: downloading golang.org/x/tools v0.31.0
go: downloading golang.org/x/sys v0.31.0
go: downloading golang.org/x/mod v0.24.0
go: downloading github.com/spf13/pflag v1.0.6
go: downloading github.com/nix-community/go-nix v0.0.0-20250101154619-4bdde671e0a1
go: module golang.org/x/tools/go/vcs is deprecated: This module contains one deprecated package.
go: upgraded go 1.23 => 1.23.0
go: added toolchain go1.23.5
go: upgraded github.com/nix-community/go-nix v0.0.0-20220612195009-5f5614f7ca47 => v0.0.0-20250101154619-4bdde671e0a1
go: upgraded github.com/spf13/cobra v1.8.1 => v1.9.1
go: upgraded github.com/spf13/pflag v1.0.5 => v1.0.6
go: upgraded golang.org/x/mod v0.22.0 => v0.24.0
go: upgraded golang.org/x/sys v0.14.0 => v0.31.0

[das@t:~/Downloads/gomod2nix]$ go build -o gomod2nix main.go

[das@t:~/Downloads/gomod2nix]$ ./gomod2nix
INFO[0000] Parsing go.mod                                modPath=go.mod
INFO[0000] Downloading dependencies
INFO[0000] Done downloading dependencies
INFO[0000] Calculating NAR hash                          goPackagePath=golang.org/x/mod
INFO[0000] Calculating NAR hash                          goPackagePath=golang.org/x/sys
INFO[0000] Calculating NAR hash                          goPackagePath=github.com/spf13/pflag
INFO[0000] Calculating NAR hash                          goPackagePath=github.com/nix-community/go-nix
INFO[0000] Calculating NAR hash                          goPackagePath=github.com/spf13/cobra
INFO[0000] Done calculating NAR hash                     goPackagePath=github.com/spf13/pflag
INFO[0000] Done calculating NAR hash                     goPackagePath=github.com/spf13/cobra
INFO[0000] Done calculating NAR hash                     goPackagePath=golang.org/x/mod
INFO[0000] Done calculating NAR hash                     goPackagePath=github.com/nix-community/go-nix
INFO[0000] Done calculating NAR hash                     goPackagePath=golang.org/x/sys
INFO[0000] Wrote: gomod2nix.toml
```

Also need to update the template: [template](./templates/app/go.mod)

Then update the flake "nix flake update"

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

## See also

[bild-go-cache](https://github.com/numtide/build-go-cache)

[nixkgs Go](https://nixos.org/manual/nixpkgs/stable/#sec-language-go)
