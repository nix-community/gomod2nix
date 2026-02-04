# shellcheck shell=bash disable=SC2154

# Shared functions used by build and check hooks
buildGoDir() {
    local cmd="$1" dir="$2"

    . "$TMPDIR/buildFlagsArray"

    declare -a flags
    flags+=($buildFlags "${buildFlagsArray[@]}")
    flags+=(${tags:+-tags=${tags}})
    flags+=(${ldflags:+-ldflags="$ldflags"})
    flags+=("-v" "-p" "$NIX_BUILD_CORES")

    if [ "$cmd" = "test" ]; then
        flags+=(-vet=off)
        flags+=($checkFlags)
    fi

    local OUT
    if ! OUT="$(go "$cmd" "${flags[@]}" "$dir" 2>&1)"; then
        if echo "$OUT" | grep -qE 'imports .*?: no Go files in'; then
            echo "$OUT" >&2
            return 1
        fi
        if ! echo "$OUT" | grep -qE '(no( buildable| non-test)?|build constraints exclude all) Go (source )?files'; then
            echo "$OUT" >&2
            return 1
        fi
    fi
    if [ -n "$OUT" ]; then
        echo "$OUT" >&2
    fi
    return 0
}

getGoDirs() {
    local type
    type="$1"
    if [ -n "$subPackages" ]; then
        echo "$subPackages" | sed "s,\(^\| \),\1./,g"
    else
        find . -type f -name \*"$type".go -exec dirname {} \; | grep -v "/vendor/" | sort --unique | grep -v "$exclude"
    fi
}

goConfigHook() {
    echo "Executing goConfigHook"

    export GOCACHE=$TMPDIR/go-cache
    export GOPATH="$TMPDIR/go"
    export GOSUMDB=off
    export GOPROXY=off
    export GO_NO_VENDOR_CHECKS=1
    export GO111MODULE=on

    export GOFLAGS="${GOFLAGS:+$GOFLAGS } -mod=vendor"
    if [ "${allowGoReference:-}" != "1" ]; then
        export GOFLAGS="$GOFLAGS -trimpath"
    fi

    cd "${modRoot:-.}"

    # Set up vendor directory if goVendorDir is provided
    if [ -n "${goVendorDir-}" ]; then
        if [ -n "${goVendorDir}" ]; then
            echo "Setting up vendor directory from ${goVendorDir}"
            rm -rf vendor
            @rsync@ -a -K --ignore-errors "${goVendorDir}"/ vendor
        fi
    fi

    # Set up cache directory if goCacheDir is provided
    if [ -n "${goCacheDir-}" ]; then
        if [ -n "${goCacheDir}" ] && [ -f "${goCacheDir}/cache.tar.zst" ]; then
            echo "Restoring Go build cache from ${goCacheDir}/cache.tar.zst"
            mkdir -p "$GOCACHE"
            @zstd@ -d -c "${goCacheDir}/cache.tar.zst" | @tar@ -xf - -C "$GOCACHE"
            chmod -R +w "$GOCACHE"
        fi
    fi

    # Set up exclude pattern for getGoDirs
    exclude='\(/_\|examples\|Godeps\|testdata'
    if [[ -n "$excludedPackages" ]]; then
      IFS=' ' read -r -a excludedArr <<<"$excludedPackages"
      printf -v excludedAlternates '%s\\|' "${excludedArr[@]}"
      excludedAlternates=${excludedAlternates%\\|} # drop final \| added by printf
      exclude+='\|'"$excludedAlternates"
    fi
    exclude+='\)'
    export exclude

    echo "Finished goConfigHook"
}

postPatchHooks+=(goConfigHook)
