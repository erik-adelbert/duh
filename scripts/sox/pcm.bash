#!/usr/bin/env bash

# pcm_format <associative-array-name> <format>
# Example: declare -A ref; pcm_format ref s16c2r44100
# Sets tag, bit_depth, channel_count, sample_rate, sample_size, and frame_size.
# Requires Bash 4.3 or newer for nameref support.
pcm_format() {
	if (($# != 2)); then
		printf 'usage: pcm_format <associative-array-name> <format>\n' >&2
		return 2
	fi

	local decl
	decl=$(declare -p -- "$1" 2>/dev/null) || {
		printf 'pcm_format: %s is not declared\n' "$1" >&2
		return 2
	}
	if [[ ! $decl =~ ^declare\ -[^[:space:]]*A ]]; then
		printf 'pcm_format: %s must be an associative array\n' "$1" >&2
		return 2
	fi

	local -n result=$1
	local format=$2 tag bit_depth channel_count sample_rate sample_size
	if [[ ! $format =~ ^([asfSuUmμ])([0-9]+)c([0-9]+)r([0-9]+)$ ]]; then
		printf 'pcm_format: invalid format: %s\n' "$format" >&2
		return 1
	fi

	tag=${BASH_REMATCH[1]}
	bit_depth=$((10#${BASH_REMATCH[2]}))
	channel_count=$((10#${BASH_REMATCH[3]}))
	sample_rate=$((10#${BASH_REMATCH[4]}))

	case $tag in
		m) tag=μ ;;
		U)
			if ((bit_depth == 8)); then
				tag=u
			fi
			;;
	esac

	if ((channel_count <= 0 || sample_rate <= 0)); then
		printf 'pcm_format: rate and channel count must be positive\n' >&2
		return 1
	fi

	case $tag in
		a|μ)
			if ((bit_depth != 8)); then
				printf 'pcm_format: A-law and μ-law must be 8 bits\n' >&2
				return 1
			fi
			;;
		f)
			if ((bit_depth != 32 && bit_depth != 64)); then
				printf 'pcm_format: float must be 32 or 64 bits\n' >&2
				return 1
			fi
			;;
		*)
			if ((bit_depth <= 0 || bit_depth > 32)); then
				printf 'pcm_format: unsupported bit depth\n' >&2
				return 1
			fi
			;;
	esac

	sample_size=$(((bit_depth + 7) / 8))
	result[tag]=$tag
	result[bit_depth]=$bit_depth
	result[channel_count]=$channel_count
	result[sample_rate]=$sample_rate
    #shellcheck disable=SC2034 # result is fine
	result[sample_size]=$sample_size
}

