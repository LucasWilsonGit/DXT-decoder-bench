#!/usr/bin/env bash
# Runs the decoder benchmark once per decoder, each in its own process, so
# no decoder's heap, GC or caches carry into another's run. It doesn't touch
# power states or the governor: set them first.
#
#   ./bench.sh [decoder...] [-- flags]
#
# With no decoders it runs all of them. Flags after -- go to every run, e.g.
#   ./bench.sh -- -cpu 4 -rounds 2000
#   ./bench.sh sonic gojson -- -copy
set -euo pipefail
cd "$(dirname "$0")"

decoders=()
while (($#)) && [[ $1 != -- ]]; do
	decoders+=("$1")
	shift
done
[[ ${1-} == -- ]] && shift
((${#decoders[@]})) || decoders=(gojson sonic jsonv2 jsonv2h)

mkdir -p bin
GOWORK=off go build -o bin/decoder-bench .

for d in "${decoders[@]}"; do
	bin/decoder-bench -decoder "$d" "$@"
done
