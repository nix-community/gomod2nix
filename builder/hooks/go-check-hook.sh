# shellcheck shell=bash disable=SC2154

goCheckHook() {
    echo "Executing goCheckHook"

    runHook preCheck

    # We do not set trimpath for tests, in case they reference test assets.
    # -trimpath is part of the build-cache key, so dropping it makes the restored
    # GOCACHE useless for the test build; keepTrimpathInCheck=1 opts out.
    if [ "${keepTrimpathInCheck:-}" != "1" ]; then
        export GOFLAGS=${GOFLAGS//-trimpath/}
    fi

    for pkg in $(getGoDirs test); do
      buildGoDir test "$pkg"
    done

    runHook postCheck

    echo "Finished goCheckHook"
}

if [ -z "${checkPhase-}" ]; then
    checkPhase=goCheckHook
fi
