# Shared by the server and client generation scripts.
CSILGEN_VERSION=0.3.3
export PATH="$PATH:$HOME/.cargo/bin"
if [[ "$(csilgen --version 2>/dev/null)" != "csilgen $CSILGEN_VERSION" ]]; then
    echo "Install csilgen $CSILGEN_VERSION and its matching WASM generators." >&2
    echo "From the csilgen repository, run ./tools.sh install-all csilgen/v$CSILGEN_VERSION" >&2
    exit 1
fi
