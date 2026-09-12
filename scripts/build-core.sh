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

# On Windows, Go's cgo links with the MinGW/GNU toolchain (already present on
# GitHub's windows-latest runners), but rustup's default target there is
# x86_64-pc-windows-msvc, which produces an incompatible astropng_core.lib
# (MSVC naming/ABI) instead of libastropng_core.a. Build the GNU target
# explicitly on Windows so the static lib matches what cgo expects.
target_dir="target"
target_flag=""
if [[ "${OS:-}" == "Windows_NT" || "${RUNNER_OS:-}" == "Windows" ]]; then
    rustup target add x86_64-pc-windows-gnu
    target_flag="--target x86_64-pc-windows-gnu"
    target_dir="target/x86_64-pc-windows-gnu"
fi

echo "Building astropng-core@$tag (release)"
# Word-splitting $target_flag is intentional (it's a controlled, fixed
# string); macOS's default bash (3.2) errors on referencing an empty array
# under `set -u`, so a plain string is used instead of a bash array.
(cd "$cache_dir" && cargo build --release --lib $target_flag)

mkdir -p "$lib_dir"
cp "$cache_dir/$target_dir/release/libastropng_core.a" "$lib_dir/"
echo "Ready: $lib_dir/libastropng_core.a"
