#!/usr/bin/env bash

set -euo pipefail

#shellcheck source=/dev/null
source "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/pcm.bash"

usage() {
	printf 'usage: %s -if <pcm-format> [-room small|medium|large|hall|cathedral] [-level 0..100]\n' "${0##*/}" >&2
}

input_format=
room=medium
level=60
while (($#)); do
	case $1 in
		-if)
			if (($# < 2)); then
				usage
				exit 2
			fi
			input_format=$2
			shift 2
			;;
		-room)
			if (($# < 2)); then
				usage
				exit 2
			fi
			room=$2
			shift 2
			;;
		-level)
			if (($# < 2)); then
				usage
				exit 2
			fi
			level=$2
			shift 2
			;;
		-h|--help)
			usage
			exit 0
			;;
		*)
			printf 'unknown argument: %s\n' "$1" >&2
			usage
			exit 2
			;;
	esac
done

if [[ -z $input_format ]]; then
	usage
	exit 2
fi

if [[ ! $level =~ ^[0-9]+$ ]] || ((10#$level > 100)); then
	printf 'level must be an integer percent from 0 to 100\n' >&2
	exit 2
fi
level=$((10#$level))

declare -A pcm
pcm_format pcm "$input_format" || exit $?

case ${pcm[tag]} in
	s|S) encoding=signed-integer ;;
	u|U) encoding=unsigned-integer ;;
	f) encoding=floating-point ;;
	a) encoding=a-law ;;
	μ) encoding=mu-law ;;
	*)
		printf 'unsupported PCM encoding: %s\n' "${pcm[tag]}" >&2
		exit 2
		;;
esac

case $encoding in
	signed-integer|unsigned-integer)
		case ${pcm[bit_depth]} in
			8|16|24|32) ;;
			*)
				printf 'SoX raw PCM does not support %s-bit integers\n' "${pcm[bit_depth]}" >&2
				exit 2
				;;
		esac
		;;
esac

endian=()
if ((pcm[bit_depth] > 8)) && [[ ${pcm[tag]} == S || ${pcm[tag]} == U ]]; then
	endian=(-B)
elif ((pcm[bit_depth] > 8)); then
	endian=(-L)
fi

case $room in
	small) reverb=(30 70 30 35 5) ;;
	medium) reverb=(40 60 80 100 15) ;;
	large) reverb=(55 45 100 100 25) ;;
	hall) reverb=(65 35 100 100 30) ;;
	cathedral) reverb=(80 25 100 100 40) ;;
	*)
		printf 'unknown room: %s (choose small, medium, large, hall, or cathedral)\n' "$room" >&2
		exit 2
		;;
esac

# Map 0..100% linearly to SoX wet-gain -10..+10 dB; 60% gives +2 dB.
wet_tenths=$((2 * level - 100))
whole=$((wet_tenths / 10))
frac=$((wet_tenths % 10))
if ((frac < 0)); then
	frac=$((-frac))
fi
if ((frac == 0)); then
	wet_gain=$whole
elif ((wet_tenths < 0 && whole == 0)); then
	wet_gain="-0.$frac"
else
	wet_gain="$whole.$frac"
fi

raw_opts=(
	-t raw
	-r "${pcm[sample_rate]}"
	-b "${pcm[bit_depth]}"
	-c "${pcm[channel_count]}"
	-e "$encoding"
)

exec sox "${raw_opts[@]}" "${endian[@]}" - "${raw_opts[@]}" "${endian[@]}" - \
	reverb "${reverb[@]}" "$wet_gain"

