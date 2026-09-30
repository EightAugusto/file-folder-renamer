#!/usr/bin/env bash
set -euo pipefail

# Variable groups: build request from the host runner.
BUILD_MODE="${1:?build mode required}"
BUILD_VERSION="${2:-}"
shift 2

# Variable groups: artifact paths and Go package identity.
OUTPUT_DIRECTORY="${HOME}/out"
APPLICATION_NAME="${APPLICATION_NAME:?application name required}"
GO_MODULE_PATH="$(awk '$1 == "module" { print $2; exit }' go.mod)"
[[ -n "${GO_MODULE_PATH}" ]] || { echo 'Go module path is missing from go.mod' >&2; exit 1; }

build_osxcross() {
    # Variable group: active, staged, and previous OSXCross toolchains.
    local OSXCROSS_CACHE_DIRECTORY="${HOME}/.cache/osxcross"
    local ACTIVE_TOOLCHAIN_DIRECTORY="${OSXCROSS_CACHE_DIRECTORY}/toolchain"
    local STAGED_TOOLCHAIN_DIRECTORY=""
    local TOOLCHAIN_WORK_DIRECTORY=""
    local PREVIOUS_TOOLCHAIN_DIRECTORY=""
    [[ -n "${MACOS_TOOLCHAIN_CACHE_KEY:-}" && -n "${SDK_VERSION:-}" ]] || { echo 'macOS cache identity is missing' >&2; exit 1; }
    if [[ -x "${ACTIVE_TOOLCHAIN_DIRECTORY}/bin/o64-clang" && -x "${ACTIVE_TOOLCHAIN_DIRECTORY}/bin/oa64-clang" && \
          -f "${ACTIVE_TOOLCHAIN_DIRECTORY}/.cache-key" && "$(<"${ACTIVE_TOOLCHAIN_DIRECTORY}/.cache-key")" == "${MACOS_TOOLCHAIN_CACHE_KEY}" ]]; then
        echo "Reusing cached OSXCross toolchain for SDK ${SDK_VERSION}."
        export PATH="${ACTIVE_TOOLCHAIN_DIRECTORY}/bin:${PATH}"
        return
    fi

    echo "Building OSXCross toolchain for SDK ${SDK_VERSION}."
    TOOLCHAIN_WORK_DIRECTORY="$(mktemp -d "${OSXCROSS_CACHE_DIRECTORY}/work.XXXXXX")"
    STAGED_TOOLCHAIN_DIRECTORY="$(mktemp -d "${OSXCROSS_CACHE_DIRECTORY}/toolchain.new.XXXXXX")"
    rmdir "${STAGED_TOOLCHAIN_DIRECTORY}"
    cp -a "${HOME}/tools/osxcross-source" "${TOOLCHAIN_WORK_DIRECTORY}/source"
    mkdir -p "${TOOLCHAIN_WORK_DIRECTORY}/sdk/MacOSX${SDK_VERSION}.sdk"
    cp -R "${HOME}/sdk/." "${TOOLCHAIN_WORK_DIRECTORY}/sdk/MacOSX${SDK_VERSION}.sdk/"
    tar -C "${TOOLCHAIN_WORK_DIRECTORY}/sdk" -cJf "${TOOLCHAIN_WORK_DIRECTORY}/source/tarballs/MacOSX${SDK_VERSION}.sdk.tar.xz" "MacOSX${SDK_VERSION}.sdk"
    (
        cd "${TOOLCHAIN_WORK_DIRECTORY}/source"
        UNATTENDED=1 BUILD_FLAVOR=llvm ENABLE_REPLACEMENT_LIPO=0 \
            ENABLE_ARCHS='x86_64 arm64' OSX_VERSION_MIN="${MACOS_MINIMUM_VERSION}" \
            TARGET_DIR="${STAGED_TOOLCHAIN_DIRECTORY}" ./build.sh
    )
    printf '%s\n' "${MACOS_TOOLCHAIN_CACHE_KEY}" > "${STAGED_TOOLCHAIN_DIRECTORY}/.cache-key"
    [[ -x "${STAGED_TOOLCHAIN_DIRECTORY}/bin/o64-clang" && -x "${STAGED_TOOLCHAIN_DIRECTORY}/bin/oa64-clang" ]] || { echo 'OSXCross toolchain is incomplete' >&2; exit 1; }
    if [[ -e "${ACTIVE_TOOLCHAIN_DIRECTORY}" ]]; then
        PREVIOUS_TOOLCHAIN_DIRECTORY="$(mktemp -d "${OSXCROSS_CACHE_DIRECTORY}/toolchain.previous.XXXXXX")"
        rmdir "${PREVIOUS_TOOLCHAIN_DIRECTORY}"
        mv "${ACTIVE_TOOLCHAIN_DIRECTORY}" "${PREVIOUS_TOOLCHAIN_DIRECTORY}"
    fi
    if ! mv "${STAGED_TOOLCHAIN_DIRECTORY}" "${ACTIVE_TOOLCHAIN_DIRECTORY}"; then
        [[ -z "${PREVIOUS_TOOLCHAIN_DIRECTORY}" ]] || mv "${PREVIOUS_TOOLCHAIN_DIRECTORY}" "${ACTIVE_TOOLCHAIN_DIRECTORY}"
        exit 1
    fi
    rm -rf "${TOOLCHAIN_WORK_DIRECTORY}" "${PREVIOUS_TOOLCHAIN_DIRECTORY}"
    export PATH="${ACTIVE_TOOLCHAIN_DIRECTORY}/bin:${PATH}"
}

build_target() {
    # Variable group: target identity, compiler, and output artifact.
    local TARGET_PLATFORM="${1}"
    local TARGET_ARCHITECTURE="${2}"
    local ARTIFACT_FILENAME="${APPLICATION_NAME}_${BUILD_VERSION}_"
    local C_COMPILER=""
    local LINKER_FLAGS="-X ${GO_MODULE_PATH}/internal/version.Release=${BUILD_VERSION}"
    case "${TARGET_PLATFORM}" in
        linux)
            [[ "${TARGET_ARCHITECTURE}" == amd64 ]] || exit 1
            ARTIFACT_FILENAME+="linux_${TARGET_ARCHITECTURE}"
            C_COMPILER=gcc
            ;;
        windows)
            [[ "${TARGET_ARCHITECTURE}" == amd64 ]] || exit 1
            ARTIFACT_FILENAME+="windows_${TARGET_ARCHITECTURE}.exe"
            C_COMPILER=x86_64-w64-mingw32-gcc
            LINKER_FLAGS="-H=windowsgui ${LINKER_FLAGS}"
            ;;
        mac)
            ARTIFACT_FILENAME+="darwin_${TARGET_ARCHITECTURE}"
            case "${TARGET_ARCHITECTURE}" in
                amd64) C_COMPILER=o64-clang ;;
                arm64) C_COMPILER=oa64-clang ;;
                *) exit 1 ;;
            esac
            export CGO_CFLAGS="-mmacosx-version-min=${MACOS_MINIMUM_VERSION}"
            export CGO_LDFLAGS="-mmacosx-version-min=${MACOS_MINIMUM_VERSION}"
            ;;
        *) echo "Unsupported target: ${TARGET_PLATFORM}" >&2; exit 1 ;;
    esac
    local TARGET_GOOS="${TARGET_PLATFORM}"
    [[ "${TARGET_PLATFORM}" != mac ]] || TARGET_GOOS=darwin
    CGO_ENABLED=1 GOOS="${TARGET_GOOS}" GOARCH="${TARGET_ARCHITECTURE}" CC="${C_COMPILER}" \
        go build -buildvcs=false -mod=readonly -trimpath -ldflags "${LINKER_FLAGS}" \
        -o "${OUTPUT_DIRECTORY}/${ARTIFACT_FILENAME}" "./cmd/${APPLICATION_NAME}"
    case "${TARGET_PLATFORM}" in
        linux) file "${OUTPUT_DIRECTORY}/${ARTIFACT_FILENAME}" | grep 'ELF 64-bit.*x86-64' >/dev/null ;;
        windows)
            file "${OUTPUT_DIRECTORY}/${ARTIFACT_FILENAME}" | grep 'PE32+ executable.*x86-64' >/dev/null
            x86_64-w64-mingw32-objdump --private-headers "${OUTPUT_DIRECTORY}/${ARTIFACT_FILENAME}" | \
                grep -E 'Subsystem[[:space:]]+00000002.*Windows GUI' >/dev/null
            ;;
        mac) file "${OUTPUT_DIRECTORY}/${ARTIFACT_FILENAME}" | grep 'Mach-O 64-bit.*executable' >/dev/null ;;
    esac
}

case "${BUILD_MODE}" in
    fyne-tool)
        [[ -n "${FYNE_TOOL_VERSION:-}" && "${#}" -eq 1 ]] || exit 1
        CGO_ENABLED=0 GOOS=darwin GOARCH="${1}" go install "fyne.io/tools/cmd/fyne@${FYNE_TOOL_VERSION}"
        cp -p "${GOPATH}/bin/darwin_${1}/fyne" "${OUTPUT_DIRECTORY}/fyne"
        ;;
    linux|windows|mac)
        [[ -n "${BUILD_VERSION}" && "${#}" -gt 0 ]] || exit 1
        [[ "${BUILD_MODE}" != mac ]] || build_osxcross
        for TARGET_ARCHITECTURE in "$@"; do build_target "${BUILD_MODE}" "${TARGET_ARCHITECTURE}"; done
        ;;
    *) echo "Unknown container mode: ${BUILD_MODE}" >&2; exit 1 ;;
esac
