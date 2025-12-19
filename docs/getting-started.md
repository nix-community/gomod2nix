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

## Working with private repositories

Gomod2nix supports private Go module repositories (e.g., private GitHub repos or self-hosted Git servers). This works by using `builtins.fetchGit` with pre-computed hashes, which allows pure Nix evaluation without requiring network access during build time.

### Generating hashes for private repos

Use the `--private` flag to specify prefixes for private repositories. Gomod2nix will clone these repos via SSH and compute their NAR hashes:

``` bash
$ gomod2nix generate --private=github.com/myorg,git.company.com
```

You can specify multiple prefixes separated by commas. Any module path starting with these prefixes will be treated as private.

**Requirements:**
- SSH access to the private repositories (via ssh-agent or SSH keys)
- Git must be available in your PATH

The generated `gomod2nix.toml` will include additional fields for private repos:

```toml
[mod."github.com/myorg/private-lib"]
  version = "v1.2.3"
  hash = "sha256-..."
  gitHash = "sha256-..."
  gitRev = "abc123..."
```

### Building with private repos

In your Nix configuration, specify which module prefixes are private using `privateRepoPrefixes`:

``` nix
pkgs.buildGoApplication {
  pname = "myapp";
  version = "0.1";
  pwd = ./.;
  src = ./.;
  modules = ./gomod2nix.toml;
  privateRepoPrefixes = [
    "github.com/myorg"
    "git.company.com"
  ];
}
```

At evaluation time, Nix will use `builtins.fetchGit` to fetch these dependencies via SSH, using the pre-computed `gitHash` and `gitRev` from `gomod2nix.toml`.

### Alternative: Using the sources parameter

For more control, you can manually provide sources for specific modules using the `sources` parameter:

``` nix
pkgs.buildGoApplication {
  pname = "myapp";
  version = "0.1";
  pwd = ./.;
  src = ./.;
  modules = ./gomod2nix.toml;
  sources = {
    "github.com/myorg/private-lib@v1.2.3" = fetchGit {
      url = "git@github.com:myorg/private-lib.git";
      rev = "...";
    };
  };
}
```

The key format is `"<module-path>@<version>"`.
