#!/usr/bin/env bash

set -euo pipefail

if (($# < 1 || $# > 2)); then
	printf 'usage: %s <midi-file> [bank:0..81 (default: 81)]\n' "${0##*/}" >&2
	exit 2
fi

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
format=s16c2r48000
bank=${2:-81}

"$repo_root/bin/middec" -sp=true "$1" "$bank" 6 |
	"$repo_root/scripts/sox/reverb.bash" -if "$format" -room small -level 65 |
	"$repo_root/bin/pcmtui" -if "$format" -vu -spectro
