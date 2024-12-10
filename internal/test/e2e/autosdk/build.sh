#!/usr/bin/env bash

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

usage() {
    local progname
    progname="$( basename "$0" )"

    cat <<-EOF
    Usage: $progname [OPTIONS]

    Builds the SDK end-to-end testing docker image.

    OPTIONS:
        -t --tag            Docker tag to use ["sample-app"]
        -h --help           Show this help message
EOF
}

parse_opts() {
    # Make sure getopts starts at the beginning
    OPTIND=1

    local deliminator option
    local arg=
    local tag=()

    # Translate --gnu-long-options to -g (short options)
    for arg
    do
        deliminator=""
        case "$arg" in
            --tag)
                args="${args}-d "
                ;;
            --help)
                args="${args}-h "
                ;;
            *)
                [[ "${arg:0:1}" == "-" ]] || deliminator='"'
                args="${args}${deliminator}${arg}${deliminator} "
                ;;
        esac
    done

    # Reset the positional parameters to start parsing short options
    eval set -- "$args"

    while getopts ":t:h" option
    do
        case "$option" in
            t)
                tag+=("$OPTARG")
                ;;
            h)
                usage
                exit 0
                ;;
            *)
                echo "Invalid option: -${option}" >&2
                usage
                exit 1
                ;;
        esac
    done

    # Default values
    if [ ${#tag[@]} -eq 0 ]; then
        readonly TAG=("sample-app")
    else
		readonly TAG=("${tag[@]}")
    fi

    return 0
}

build() {
    local root_dir="$1"
    local local_dir="$2"
    local dockerfile="${local_dir}/Dockerfile"
    local tag_arg tag

    if [ ! -f "$dockerfile" ]; then
        echo "Dockerfile does not exist: $dockerfile"
        return 1
    fi

    if [ ! -d "$root_dir" ]; then
        echo "Project root directory does not exist: $root_dir"
        return 1
    fi

    for tag in "${TAG[@]}"; do
        tag_arg+=("-t" "$tag")
    done

    (cd "$root_dir" && docker build "${tag_arg[@]}" -f "$dockerfile" .)
    return 0
}

main() {
    local root_dir script_dir

    # Check dependencies
    hash git 2>/dev/null\
        || { echo >&2 "Required git client not found"; exit 1; }
    hash docker 2>/dev/null\
        || { echo >&2 "Required docker client not found"; exit 1; }

    parse_opts "$@"

    root_dir=$( git rev-parse --show-toplevel )
    script_dir=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
    build "$root_dir" "$script_dir"
}

main "$@"
