#!/usr/bin/env bash
# Builds astropng-core (the shared conversion pipeline, see
# third_party/astropng-core/VERSION for the pinned tag) and drops the static
# library where internal/batch's cgo wrapper expects it
# (third_party/astropng-core/lib/libastropng_core.a). Requires a Rust
# toolchain (cargo) on PATH. Safe to re-run; re-clones/rebuilds only if the
# pinned tag or cached checkout changed.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tag="$(cat "$repo_root/third_party/astropng-core/VERSION")"
cache_dir="${ASTROPNG_CORE_CACHE:-$HOME/.cache/astropng-core}/$tag"
lib_dir="$repo_root/third_party/astropng-core/lib"

if [[ ! -d "$cache_dir" ]]; then
    echo "Cloning astropng-core@$tag into $cache_dir"
    git clone --quiet --depth 1 --branch "$tag" \
        https://github.com/peterbuitho/astropng-core "$cache_dir"
fi

echo "Building astropng-core@$tag (release)"
(cd "$cache_dir" && cargo build --release --lib)

mkdir -p "$lib_dir"
cp "$cache_dir/target/release/libastropng_core.a" "$lib_dir/"
echo "Ready: $lib_dir/libastropng_core.a"
