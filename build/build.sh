#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

# Variable groups: repository paths and release artifacts.
BUILD_SCRIPT_DIRECTORY="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPOSITORY_ROOT="$(cd "${BUILD_SCRIPT_DIRECTORY}/.." && pwd)"
BUILD_CACHE_DIRECTORY="${BUILD_SCRIPT_DIRECTORY}/.cache"
DISTRIBUTION_DIRECTORY="${BUILD_SCRIPT_DIRECTORY}/dists"
APPLICATION_NAME="file-folder-renamer"

# Variable groups: compiler and packaging versions.
GO_TOOLCHAIN_VERSION="go1.26.8"
FYNE_TOOL_VERSION="v1.7.2"
LLVM_VERSION=21
MACOS_MINIMUM_VERSION=12.0
OSXCROSS_REVISION=27d21e4977c9751d01199c7a226a6faf494c3dd9

# Variable groups: container identity and mount paths.
BUILDER_USER_NAME="eightaugusto"
CONTAINER_HOME_DIRECTORY="/home/${BUILDER_USER_NAME}"
CONTAINER_SOURCE_DIRECTORY="${CONTAINER_HOME_DIRECTORY}/source"
CONTAINER_OUTPUT_DIRECTORY="${CONTAINER_HOME_DIRECTORY}/out"
CONTAINER_SDK_DIRECTORY="${CONTAINER_HOME_DIRECTORY}/sdk"

# Variable groups: state for the requested build.
BUILD_ARCHITECTURES=()
DOCKER_HOST_ARCHITECTURE=""
BUILD_STAGE_DIRECTORY=""

build_usage() {
	cat <<'EOF'
Usage: build/build.sh <linux|windows|mac> [OPTIONS]

Commands:
  linux            Build Linux/amd64.
  windows          Build Windows/amd64.
  mac              Build macOS/amd64 and macOS/arm64.

Options:
  --arch ARCHITECTURE  Build a selected architecture for one platform.
                       Repeat for multiple macOS architectures.
  -h, --help        Show this help.
EOF
}

fail() {
	echo "Error: ${*}" >&2
	exit 1
}

require_command() {
	command -v "${1}" >/dev/null 2>&1 || fail "Required command '${1}' was not found."
}

get_fyne_property() {
	# Variable group: requested TOML section and property.
	local SECTION_NAME="$1" PROPERTY_NAME="$2"
	# The release fields use plain double-quoted strings in FyneApp.toml.
	awk -v SECTION_NAME="${SECTION_NAME}" -v PROPERTY_NAME="${PROPERTY_NAME}" '
		/^[[:space:]]*\[/ {
			IN_SECTION = ($0 ~ "^[[:space:]]*\\[" SECTION_NAME "\\][[:space:]]*(#.*)?$")
			if (IN_SECTION) SECTION_COUNT++
			next
		}
		IN_SECTION && $0 ~ "^[[:space:]]*" PROPERTY_NAME "[[:space:]]*=" {
			PROPERTY_COUNT++
			if ($0 !~ /^[[:space:]]*[A-Za-z][A-Za-z0-9_]*[[:space:]]*=[[:space:]]*"[^"]*"[[:space:]]*(#.*)?$/) INVALID = 1
			split($0, PARTS, "\"")
			PROPERTY_VALUE = PARTS[2]
		}
		END { if (SECTION_COUNT != 1 || PROPERTY_COUNT != 1 || INVALID) exit 1; print PROPERTY_VALUE }
	' "${REPOSITORY_ROOT}/FyneApp.toml"
}

get_application_display_name() {
	local DISPLAY_NAME_VALUE; DISPLAY_NAME_VALUE="$(get_fyne_property Details Name)" || return 1
	[[ -n "${DISPLAY_NAME_VALUE}" && "${DISPLAY_NAME_VALUE}" != */* ]] || return 1
	printf '%s\n' "${DISPLAY_NAME_VALUE}"
}

get_application_identifier() {
	local APPLICATION_IDENTIFIER_VALUE; APPLICATION_IDENTIFIER_VALUE="$(get_fyne_property Details ID)" || return 1
	[[ -n "${APPLICATION_IDENTIFIER_VALUE}" ]] || return 1
	printf '%s\n' "${APPLICATION_IDENTIFIER_VALUE}"
}

get_application_icon_path() {
	local APPLICATION_ICON_PATH_VALUE; APPLICATION_ICON_PATH_VALUE="$(get_fyne_property Details Icon)" || return 1
	case "${APPLICATION_ICON_PATH_VALUE}" in
		''|/*|..|../*|*/../*|*/..) return 1 ;;
	esac
	[[ -f "${REPOSITORY_ROOT}/${APPLICATION_ICON_PATH_VALUE}" ]] || return 1
	printf '%s\n' "${APPLICATION_ICON_PATH_VALUE}"
}

get_release_version() {
	local RELEASE_VERSION_VALUE; RELEASE_VERSION_VALUE="$(get_fyne_property Details Version)" || return 1
	[[ "${RELEASE_VERSION_VALUE}" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] || return 1
	printf '%s\n' "${RELEASE_VERSION_VALUE}"
}

parse_build_args() {
	# Variable group: requested platform and supported architectures.
	local TARGET_PLATFORM="${1}"
	local SUPPORTED_ARCHITECTURES_TEXT="${2}"
	shift 2

	local SUPPORTED_ARCHITECTURES=(${SUPPORTED_ARCHITECTURES_TEXT})
	local REQUESTED_ARCHITECTURES=()
	# Variable group: parsed options and validation state.
	local ARCHITECTURE_VALUE=""
	local TARGET_ARCHITECTURE=""
	local CANDIDATE_ARCHITECTURE=""
	local ARCHITECTURE_SUPPORTED=0
	local ARCHITECTURE_ALREADY_REQUESTED=0

	while (( ${#} > 0 )); do
		case "${1}" in
			--arch)
				(( ${#} >= 2 )) || fail "--arch requires a value."
				[[ "${2}" != --* ]] || fail "--arch requires a value."
				REQUESTED_ARCHITECTURES+=("${2}")
				shift 2
				;;
			--arch=*)
				ARCHITECTURE_VALUE="${1#--arch=}"
				[[ -n "${ARCHITECTURE_VALUE}" ]] || fail "--arch requires a value."
				REQUESTED_ARCHITECTURES+=("${ARCHITECTURE_VALUE}")
				shift
				;;
			-h|--help)
				build_usage
				exit 0
				;;
			*)
				fail "Unknown argument '${1}'. Run with --help for usage."
				;;
		esac
	done

	if (( ${#REQUESTED_ARCHITECTURES[@]} == 0 )); then
		REQUESTED_ARCHITECTURES=("${SUPPORTED_ARCHITECTURES[@]}")
	fi

	BUILD_ARCHITECTURES=()
	for TARGET_ARCHITECTURE in "${REQUESTED_ARCHITECTURES[@]}"; do
		ARCHITECTURE_SUPPORTED=0
		for CANDIDATE_ARCHITECTURE in "${SUPPORTED_ARCHITECTURES[@]}"; do
			if [[ "${TARGET_ARCHITECTURE}" == "${CANDIDATE_ARCHITECTURE}" ]]; then
				ARCHITECTURE_SUPPORTED=1
				break
			fi
		done
		(( ARCHITECTURE_SUPPORTED == 1 )) || fail "Unsupported ${TARGET_PLATFORM} architecture '${TARGET_ARCHITECTURE}'. Supported: ${SUPPORTED_ARCHITECTURES_TEXT}."

		ARCHITECTURE_ALREADY_REQUESTED=0
		# Bash 3.2 treats an empty array expansion as unset when nounset is active.
		for CANDIDATE_ARCHITECTURE in ${BUILD_ARCHITECTURES[@]+"${BUILD_ARCHITECTURES[@]}"}; do
			if [[ "${TARGET_ARCHITECTURE}" == "${CANDIDATE_ARCHITECTURE}" ]]; then
				ARCHITECTURE_ALREADY_REQUESTED=1
				break
			fi
		done
		if (( ARCHITECTURE_ALREADY_REQUESTED == 0 )); then
			BUILD_ARCHITECTURES+=("${TARGET_ARCHITECTURE}")
		fi
	done

	APPLICATION_DISPLAY_NAME="$(get_application_display_name)" || fail 'FyneApp.toml needs one nonempty quoted [Details].Name without a slash.'
	APPLICATION_IDENTIFIER="$(get_application_identifier)" || fail 'FyneApp.toml needs one nonempty quoted [Details].ID.'
	APPLICATION_ICON_PATH="$(get_application_icon_path)" || fail 'FyneApp.toml needs one quoted [Details].Icon pointing to a repository file.'
	BUILD_VERSION="$(get_release_version)" || fail 'FyneApp.toml needs one quoted [Details].Version in MAJOR.MINOR.PATCH form.'
}

require_docker_buildx() {
    require_command docker
    docker buildx version >/dev/null 2>&1 || fail "Docker Buildx is required."
}

resolve_docker_host_arch() {
    case "$(uname -m)" in
        x86_64|amd64) DOCKER_HOST_ARCHITECTURE=amd64 ;;
        arm64|aarch64) DOCKER_HOST_ARCHITECTURE=arm64 ;;
        *) fail "Unsupported Docker host architecture: $(uname -m)" ;;
    esac
}

builder_image_name() {
    printf '%s-%s-builder:%s\n' "${APPLICATION_NAME}" "$1" "${GO_TOOLCHAIN_VERSION}"
}

build_toolchain_image() {
    # Variable group: Docker target and host identity.
    local TARGET_PLATFORM="$1"
    local CONTAINER_PLATFORM=linux/amd64
    local BUILDER_IMAGE_NAME="$(builder_image_name "${TARGET_PLATFORM}")"
    local HOST_USER_ID="$(id -u)"
    [[ "${TARGET_PLATFORM}" != mac ]] || CONTAINER_PLATFORM=linux/arm64
    (( HOST_USER_ID > 0 )) || fail 'The rootless builder requires a non-root host user ID.'
    echo "Preparing ${TARGET_PLATFORM} rootless toolchain image ${BUILDER_IMAGE_NAME} (${CONTAINER_PLATFORM})."
    docker buildx build --file "${BUILD_SCRIPT_DIRECTORY}/toolchains.Dockerfile" \
        --target "${TARGET_PLATFORM}" --platform "${CONTAINER_PLATFORM}" \
        --build-arg "GO_VERSION=${GO_TOOLCHAIN_VERSION#go}" \
        --build-arg "LLVM_VERSION=${LLVM_VERSION}" \
        --build-arg "MACOS_MINIMUM_VERSION=${MACOS_MINIMUM_VERSION}" \
        --build-arg "OSXCROSS_REVISION=${OSXCROSS_REVISION}" \
        --build-arg "USER_ID=${HOST_USER_ID}" --build-arg "USER_NAME=${BUILDER_USER_NAME}" \
        --tag "${BUILDER_IMAGE_NAME}" --load --pull "${REPOSITORY_ROOT}"
    [[ "$(docker image inspect --format '{{.Config.User}}' "${BUILDER_IMAGE_NAME}")" == "${BUILDER_USER_NAME}" ]] || \
        fail "${BUILDER_IMAGE_NAME} must run as the named user ${BUILDER_USER_NAME}."
}

cache_key() {
    if command -v shasum >/dev/null 2>&1; then
        printf '%s\0' "$@" | shasum -a 256 | awk '{print $1}'
    else
        printf '%s\0' "$@" | sha256sum | awk '{print $1}'
    fi
}

new_stage_directory() {
    mkdir -p "${BUILD_CACHE_DIRECTORY}/staging"
    mktemp -d "${BUILD_CACHE_DIRECTORY}/staging/${1}.XXXXXX"
}

cleanup_stage() {
    if [[ -n "${BUILD_STAGE_DIRECTORY:-}" && -d "${BUILD_STAGE_DIRECTORY}" ]]; then
        rm -rf "${BUILD_STAGE_DIRECTORY}"
    fi
}

# Restore both backup and install moves, including failures partway through backup.
rollback_distribution() {
    # Variable group: saved distribution and artifact being restored.
    local BACKUP_DIRECTORY="$1"
    shift
    local ARTIFACT_PATH=""
    for ARTIFACT_PATH in "$@"; do rm -rf "${ARTIFACT_PATH}"; done
    shopt -s nullglob
    for ARTIFACT_PATH in "${BACKUP_DIRECTORY}"/*; do mv "${ARTIFACT_PATH}" "${DISTRIBUTION_DIRECTORY}/" || return 1; done
    shopt -u nullglob
}

install_distribution() {
    # Variable group: staged release and replacement transaction.
    local STAGED_DISTRIBUTION_DIRECTORY="$1" TARGET_PLATFORM="$2" TARGET_ARCHITECTURE="$3"
    local TRANSACTION_DIRECTORY="" INCOMING_DIRECTORY="" BACKUP_DIRECTORY="" ARTIFACT_PATH="" ARTIFACT_FILENAME=""
    local -a EXPECTED_ARTIFACT_FILENAMES=() EXISTING_ARTIFACT_PATHS=() STAGED_ARTIFACT_PATHS=() INSTALLED_ARTIFACT_PATHS=()
    [[ -d "${STAGED_DISTRIBUTION_DIRECTORY}" ]] || fail "Distribution staging directory is missing: ${STAGED_DISTRIBUTION_DIRECTORY}"
    case "${TARGET_PLATFORM}" in
        linux) EXPECTED_ARTIFACT_FILENAMES=("${APPLICATION_NAME}_${BUILD_VERSION}_linux_${TARGET_ARCHITECTURE}") ;;
        windows) EXPECTED_ARTIFACT_FILENAMES=("${APPLICATION_NAME}_${BUILD_VERSION}_windows_${TARGET_ARCHITECTURE}.exe") ;;
        mac) EXPECTED_ARTIFACT_FILENAMES=("${APPLICATION_NAME}_${BUILD_VERSION}_darwin_${TARGET_ARCHITECTURE}" \
            "${APPLICATION_DISPLAY_NAME}_${BUILD_VERSION}_darwin_${TARGET_ARCHITECTURE}.dmg") ;;
        *) fail "Unsupported distribution platform '${TARGET_PLATFORM}'." ;;
    esac
    for ARTIFACT_FILENAME in "${EXPECTED_ARTIFACT_FILENAMES[@]}"; do
        [[ -e "${STAGED_DISTRIBUTION_DIRECTORY}/${ARTIFACT_FILENAME}" ]] || fail "Missing staged artifact: ${ARTIFACT_FILENAME}"
    done
    shopt -s nullglob
    STAGED_ARTIFACT_PATHS=("${STAGED_DISTRIBUTION_DIRECTORY}"/*)
    shopt -u nullglob
    (( ${#STAGED_ARTIFACT_PATHS[@]} == ${#EXPECTED_ARTIFACT_FILENAMES[@]} )) || fail "Unexpected files in staging: ${STAGED_DISTRIBUTION_DIRECTORY}"

    mkdir -p "${DISTRIBUTION_DIRECTORY}"
    TRANSACTION_DIRECTORY="$(mktemp -d "${DISTRIBUTION_DIRECTORY}/.transaction-${TARGET_PLATFORM}-${TARGET_ARCHITECTURE}.XXXXXX")"
    INCOMING_DIRECTORY="${TRANSACTION_DIRECTORY}/incoming"
    BACKUP_DIRECTORY="${TRANSACTION_DIRECTORY}/previous"
    mv "${STAGED_DISTRIBUTION_DIRECTORY}" "${INCOMING_DIRECTORY}"
    mkdir -p "${BACKUP_DIRECTORY}"
    shopt -s nullglob
    case "${TARGET_PLATFORM}" in
        linux) EXISTING_ARTIFACT_PATHS=("${DISTRIBUTION_DIRECTORY}/${APPLICATION_NAME}_"*_linux_"${TARGET_ARCHITECTURE}") ;;
        windows) EXISTING_ARTIFACT_PATHS=("${DISTRIBUTION_DIRECTORY}/${APPLICATION_NAME}_"*_windows_"${TARGET_ARCHITECTURE}.exe") ;;
        mac) EXISTING_ARTIFACT_PATHS=("${DISTRIBUTION_DIRECTORY}/${APPLICATION_NAME}_"*_darwin_"${TARGET_ARCHITECTURE}" \
            "${DISTRIBUTION_DIRECTORY}/${APPLICATION_DISPLAY_NAME}_"*_darwin_"${TARGET_ARCHITECTURE}.app" \
            "${DISTRIBUTION_DIRECTORY}/${APPLICATION_DISPLAY_NAME}_"*_darwin_"${TARGET_ARCHITECTURE}.dmg") ;;
    esac
    # Retire the shared license file left by earlier release builds.
    [[ ! -e "${DISTRIBUTION_DIRECTORY}/LICENSE" ]] || EXISTING_ARTIFACT_PATHS+=("${DISTRIBUTION_DIRECTORY}/LICENSE")
    shopt -u nullglob
    for ARTIFACT_PATH in ${EXISTING_ARTIFACT_PATHS[@]+"${EXISTING_ARTIFACT_PATHS[@]}"}; do
        if ! mv "${ARTIFACT_PATH}" "${BACKUP_DIRECTORY}/"; then
            rollback_distribution "${BACKUP_DIRECTORY}" || fail "Rollback failed for ${TARGET_PLATFORM}/${TARGET_ARCHITECTURE}: ${TRANSACTION_DIRECTORY}"
            rm -rf "${TRANSACTION_DIRECTORY}"
            fail "Could not back up ${TARGET_PLATFORM}/${TARGET_ARCHITECTURE} distribution."
        fi
    done
    for ARTIFACT_FILENAME in "${EXPECTED_ARTIFACT_FILENAMES[@]}"; do
        if ! mv "${INCOMING_DIRECTORY}/${ARTIFACT_FILENAME}" "${DISTRIBUTION_DIRECTORY}/${ARTIFACT_FILENAME}"; then
            rollback_distribution "${BACKUP_DIRECTORY}" ${INSTALLED_ARTIFACT_PATHS[@]+"${INSTALLED_ARTIFACT_PATHS[@]}"} || fail "Rollback failed for ${TARGET_PLATFORM}/${TARGET_ARCHITECTURE}: ${TRANSACTION_DIRECTORY}"
            rm -rf "${TRANSACTION_DIRECTORY}"
            fail "Could not install ${TARGET_PLATFORM}/${TARGET_ARCHITECTURE} distribution."
        fi
        INSTALLED_ARTIFACT_PATHS+=("${DISTRIBUTION_DIRECTORY}/${ARTIFACT_FILENAME}")
    done
    rm -rf "${TRANSACTION_DIRECTORY}"
}

run_build_container() {
    # Variable group: container request and bind mounts.
    local TARGET_PLATFORM="$1" OUTPUT_DIRECTORY="$2" SDK_DIRECTORY="$3" OSXCROSS_CACHE_DIRECTORY="$4" CONTAINER_BUILD_MODE="$5" RELEASE_VERSION="$6"
    shift 6
    local CONTAINER_PLATFORM=linux/amd64
    local -a CONTAINER_MOUNTS=()
    [[ "${TARGET_PLATFORM}" != mac ]] || CONTAINER_PLATFORM=linux/arm64
    mkdir -p "${BUILD_CACHE_DIRECTORY}/go-mod" "${BUILD_CACHE_DIRECTORY}/go-build" "${OUTPUT_DIRECTORY}"
    CONTAINER_MOUNTS+=(--mount "type=bind,source=${REPOSITORY_ROOT},target=${CONTAINER_SOURCE_DIRECTORY},readonly")
    CONTAINER_MOUNTS+=(--mount "type=bind,source=${OUTPUT_DIRECTORY},target=${CONTAINER_OUTPUT_DIRECTORY}")
    CONTAINER_MOUNTS+=(--mount "type=bind,source=${BUILD_CACHE_DIRECTORY}/go-mod,target=${CONTAINER_HOME_DIRECTORY}/go/pkg/mod")
    CONTAINER_MOUNTS+=(--mount "type=bind,source=${BUILD_CACHE_DIRECTORY}/go-build,target=${CONTAINER_HOME_DIRECTORY}/.cache/go-build")
    if [[ -n "${SDK_DIRECTORY}" ]]; then
        CONTAINER_MOUNTS+=(--mount "type=bind,source=${SDK_DIRECTORY},target=${CONTAINER_SDK_DIRECTORY},readonly")
    fi
    if [[ -n "${OSXCROSS_CACHE_DIRECTORY}" ]]; then
        mkdir -p "${OSXCROSS_CACHE_DIRECTORY}"
        CONTAINER_MOUNTS+=(--mount "type=bind,source=${OSXCROSS_CACHE_DIRECTORY},target=${CONTAINER_HOME_DIRECTORY}/.cache/osxcross")
    fi
    docker run --rm --platform "${CONTAINER_PLATFORM}" \
        --env "APPLICATION_NAME=${APPLICATION_NAME}" \
        --env "FYNE_TOOL_VERSION=${FYNE_TOOL_VERSION}" \
        --env "MACOS_MINIMUM_VERSION=${MACOS_MINIMUM_VERSION}" \
        --env "SDK_VERSION=${SDK_VERSION:-}" --env "MACOS_TOOLCHAIN_CACHE_KEY=${MACOS_TOOLCHAIN_CACHE_KEY:-}" \
        "${CONTAINER_MOUNTS[@]}" --workdir "${CONTAINER_SOURCE_DIRECTORY}" \
        "$(builder_image_name "${TARGET_PLATFORM}")" \
        /bin/bash "${CONTAINER_SOURCE_DIRECTORY}/build/container.sh" "${CONTAINER_BUILD_MODE}" "${RELEASE_VERSION}" "$@"
}

run_binary_platform() (
    # Variable group: platform selection and output artifact.
    local TARGET_PLATFORM="$1" SUPPORTED_ARCHITECTURES_TEXT="$2" TARGET_ARCHITECTURE="" ARTIFACT_FILENAME=""
    shift 2
    parse_build_args "${TARGET_PLATFORM}" "${SUPPORTED_ARCHITECTURES_TEXT}" "$@"
    require_docker_buildx
    build_toolchain_image "${TARGET_PLATFORM}"
    BUILD_STAGE_DIRECTORY=""
    trap cleanup_stage EXIT
    for TARGET_ARCHITECTURE in "${BUILD_ARCHITECTURES[@]}"; do
        BUILD_STAGE_DIRECTORY="$(new_stage_directory "${TARGET_PLATFORM}-${TARGET_ARCHITECTURE}")"
        mkdir -p "${BUILD_STAGE_DIRECTORY}/distribution"
        echo "Building ${TARGET_PLATFORM}/${TARGET_ARCHITECTURE} in a rootless container."
        run_build_container "${TARGET_PLATFORM}" "${BUILD_STAGE_DIRECTORY}/export" '' '' "${TARGET_PLATFORM}" "${BUILD_VERSION}" "${TARGET_ARCHITECTURE}"
        ARTIFACT_FILENAME="${APPLICATION_NAME}_${BUILD_VERSION}_${TARGET_PLATFORM}_${TARGET_ARCHITECTURE}"
        [[ "${TARGET_PLATFORM}" != windows ]] || ARTIFACT_FILENAME+=.exe
        [[ -x "${BUILD_STAGE_DIRECTORY}/export/${ARTIFACT_FILENAME}" ]] || fail "Container did not export ${ARTIFACT_FILENAME}."
        mv "${BUILD_STAGE_DIRECTORY}/export/${ARTIFACT_FILENAME}" "${BUILD_STAGE_DIRECTORY}/distribution/${ARTIFACT_FILENAME}"
        install_distribution "${BUILD_STAGE_DIRECTORY}/distribution" "${TARGET_PLATFORM}" "${TARGET_ARCHITECTURE}"
        echo "${TARGET_PLATFORM}/${TARGET_ARCHITECTURE} distribution: ${DISTRIBUTION_DIRECTORY}"
        rm -rf "${BUILD_STAGE_DIRECTORY}"
        BUILD_STAGE_DIRECTORY=""
    done
)

run_linux() { run_binary_platform linux amd64 "$@"; }
run_windows() { run_binary_platform windows amd64 "$@"; }

source "${BUILD_SCRIPT_DIRECTORY}/macos.sh"

main() {
    local BUILD_COMMAND="${1:-}"
    case "${BUILD_COMMAND}" in
        -h|--help) build_usage; return ;;
        '') fail 'A build command is required. Run with --help for usage.' ;;
    esac
    shift
    case "${BUILD_COMMAND}" in
        linux) run_linux "$@" ;;
        windows) run_windows "$@" ;;
        mac) run_mac "$@" ;;
        *) fail "Unknown build command '${BUILD_COMMAND}'. Run with --help for usage." ;;
    esac
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then main "$@"; fi
