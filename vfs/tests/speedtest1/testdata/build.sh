#!/usr/bin/env bash
set -euo pipefail

cd -P -- "$(dirname -- "$0")"

ROOT=../../../../
BINARYEN="$ROOT/tools/binaryen-version_116/bin"
WASI_SDK="$ROOT/tools/wasi-sdk-21.0/bin"

"$WASI_SDK/clang" --target=wasm32-wasi -flto -g0 -O2 \
	-o speedtest1.wasm main.c \
	-I"$ROOT/sqlite3" \
	-msimd128 -mmutable-globals \
	-mbulk-memory -mreference-types \
	-mnontrapping-fptoint -msign-ext \
	-fno-stack-protector -fno-stack-clash-protection \
	-Wl,--stack-first \
	-Wl,--import-undefined \
	-D_HAVE_SQLITE_CONFIG_H

"$BINARYEN/wasm-opt" -g --strip --strip-producers -c -O3 \
	speedtest1.wasm -o speedtest1.tmp \
	--enable-simd --enable-mutable-globals --enable-multivalue \
	--enable-bulk-memory --enable-reference-types \
	--enable-nontrapping-float-to-int --enable-sign-ext
mv speedtest1.tmp speedtest1.wasm
bzip2 -9f speedtest1.wasm