#!/usr/bin/env bash
# shellcheck disable=SC2154,SC1091
# SC2154: Variables $stdenv, $out, $goPackagePath, $version are provided by Nix
# SC1091: Can't follow $stdenv/setup (Nix provides it at build time)

source "$stdenv/setup"

HOME=$(mktemp -d)
export HOME

# Setup authentication for private Go modules
# NETRC_CONTENT is set by nix when netrcFile parameter is provided
if [[ -n "${NETRC_CONTENT:-}" ]]; then
  echo "$NETRC_CONTENT" > "$HOME/.netrc"
  chmod 600 "$HOME/.netrc"
fi

# Call once first outside of subshell for better error reporting
go mod download "$goPackagePath@$version"

dir=$(go mod download --json "$goPackagePath@$version" | jq -r .Dir)

chmod -R +w "$dir"
find "$dir" -iname ".ds_store" -print0 | xargs -0 -r rm -rf

cp -r "$dir" "$out"
