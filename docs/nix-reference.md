# Gomod2nix Nix API

## Public functions

### buildGoApplication

Arguments:

- **modules** Path to `gomod2nix.toml` (\_default: `pwd + "/gomod2nix.toml"`).
- **src** Path to sources (\_default: `pwd`).
- **pwd** Path to working directory (\_default: `null`).
- **go** The Go compiler to use (can be omitted).
- **subPackages** Only build these specific sub packages.
- **allowGoReference** Allow references to the Go compiler in the output closure (\_default: `false`).
- **tags** A list of tags to pass the Go compiler during the build (\_default: `[ ]`).
- **ldflags** A list of `ldflags` to pass the Go compiler during the build (\_default: `[ ]`).
- **nativeBuildInputs** A list of packages to include in the build derivation (\_default: `[ ]`).

All other arguments are passed verbatim to `stdenv.mkDerivation`.

### mkGoEnv

Arguments:

- **pwd** Path to working directory.
- **modules** Path to `gomod2nix.toml` (\_default: `pwd + "/gomod2nix.toml"`).
- **toolsGo** Path to `tools.go` (\_default: `pwd + "/tools.go"`).

All other arguments are passed verbatim to `stdenv.mkDerivation`.
