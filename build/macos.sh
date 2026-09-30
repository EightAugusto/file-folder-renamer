#!/usr/bin/env bash
# Sourced by build.sh. Keep host SDK handling and native packaging here.
# Variable groups are declared locally for SDK selection, cache staging, and packaging.

sdk_signature() {
    # Variable group: SDK files used to identify the selected SDK.
    local SDK_DIRECTORY="$1" SDK_RELEASE="$2" SDK_SETTINGS_PATH="${1}/SDKSettings.json"
    local SDK_SETTINGS_HASH=missing
    [[ ! -f "${SDK_SETTINGS_PATH}" ]] || SDK_SETTINGS_HASH="$(shasum -a 256 "${SDK_SETTINGS_PATH}" | awk '{print $1}')"
    cache_key "${SDK_DIRECTORY}" "${SDK_RELEASE}" "$(stat -f '%m' "${SDK_DIRECTORY}")" \
        "$(shasum -a 256 "${SDK_DIRECTORY}/usr/lib/libSystem.tbd" | awk '{print $1}')" "${SDK_SETTINGS_HASH}"
}

mac_toolchain_cache_key() {
    cache_key "$1" "${GO_TOOLCHAIN_VERSION}" "${LLVM_VERSION}" "${MACOS_MINIMUM_VERSION}" "${OSXCROSS_REVISION}"
}

stage_macos_sdk() {
    # Variable group: installed SDK candidates and fallback version.
    local INSTALLED_SDK_DIRECTORY="" SDK_CANDIDATE_DIRECTORY="" SDK_CANDIDATE_NAME=""
    local FALLBACK_SDK_VERSION="" FALLBACK_SDK_MAJOR=-1 FALLBACK_SDK_MINOR=-1
    local CANDIDATE_SDK_MAJOR=0 CANDIDATE_SDK_MINOR=0 SELECTED_SDK_VERSION=""
    # Variable group: cached SDK replacement transaction.
    local STAGED_SDK_DIRECTORY="" PREVIOUS_SDK_DIRECTORY="" STAGED_SDK_STAMP_PATH="" CACHED_SDK_SIGNATURE=""
    SDK_SYMLINK_PATH="$(xcrun --sdk macosx --show-sdk-path)"
    SDK_VERSION="$(xcrun --sdk macosx --show-sdk-version)"
    [[ -d "${SDK_SYMLINK_PATH}" ]] || fail "The selected macOS SDK does not exist: ${SDK_SYMLINK_PATH}"
    SDK_SOURCE_DIRECTORY="$(cd "${SDK_SYMLINK_PATH}" && pwd -P)"
    [[ -f "${SDK_SOURCE_DIRECTORY}/usr/lib/libSystem.tbd" ]] || fail "The selected macOS SDK has no libSystem.tbd: ${SDK_SOURCE_DIRECTORY}"
    if grep -Fq 'arm64e.x1-' "${SDK_SOURCE_DIRECTORY}/usr/lib/libSystem.tbd"; then
        SELECTED_SDK_VERSION="${SDK_VERSION}"
        INSTALLED_SDK_DIRECTORY="$(dirname "${SDK_SOURCE_DIRECTORY}")"
        for SDK_CANDIDATE_DIRECTORY in "${INSTALLED_SDK_DIRECTORY}"/MacOSX*.sdk; do
            [[ -d "${SDK_CANDIDATE_DIRECTORY}" && ! -L "${SDK_CANDIDATE_DIRECTORY}" ]] || continue
            SDK_CANDIDATE_NAME="$(basename "${SDK_CANDIDATE_DIRECTORY}")"
            [[ "${SDK_CANDIDATE_NAME}" =~ ^MacOSX([0-9]+)\.([0-9]+)\.sdk$ ]] || continue
            [[ -f "${SDK_CANDIDATE_DIRECTORY}/usr/lib/libSystem.tbd" ]] || continue
            if grep -Fq 'arm64e.x1-' "${SDK_CANDIDATE_DIRECTORY}/usr/lib/libSystem.tbd"; then continue; fi
            CANDIDATE_SDK_MAJOR=$((10#${BASH_REMATCH[1]}))
            CANDIDATE_SDK_MINOR=$((10#${BASH_REMATCH[2]}))
            if (( CANDIDATE_SDK_MAJOR > FALLBACK_SDK_MAJOR || (CANDIDATE_SDK_MAJOR == FALLBACK_SDK_MAJOR && CANDIDATE_SDK_MINOR > FALLBACK_SDK_MINOR) )); then
                FALLBACK_SDK_VERSION="${CANDIDATE_SDK_MAJOR}.${CANDIDATE_SDK_MINOR}"
                FALLBACK_SDK_MAJOR="${CANDIDATE_SDK_MAJOR}"
                FALLBACK_SDK_MINOR="${CANDIDATE_SDK_MINOR}"
            fi
        done
        [[ -n "${FALLBACK_SDK_VERSION}" ]] || fail "SDK ${SELECTED_SDK_VERSION} uses unsupported arm64e.x1 and no compatible SDK was found."
        SDK_SYMLINK_PATH="$(xcrun --sdk "macosx${FALLBACK_SDK_VERSION}" --show-sdk-path)"
        SDK_VERSION="$(xcrun --sdk "macosx${FALLBACK_SDK_VERSION}" --show-sdk-version)"
        SDK_SOURCE_DIRECTORY="$(cd "${SDK_SYMLINK_PATH}" && pwd -P)"
        echo "SDK ${SELECTED_SDK_VERSION} uses unsupported arm64e.x1; using SDK ${SDK_VERSION}."
    fi
    SDK_CACHE_DIRECTORY="${BUILD_CACHE_DIRECTORY}/macos-sdk/${SDK_VERSION}"
    SDK_CACHE_STAMP_PATH="${SDK_CACHE_DIRECTORY}.source"
    SDK_SOURCE_SIGNATURE="$(sdk_signature "${SDK_SOURCE_DIRECTORY}" "${SDK_VERSION}")"
    if [[ -f "${SDK_CACHE_STAMP_PATH}" ]]; then CACHED_SDK_SIGNATURE="$(<"${SDK_CACHE_STAMP_PATH}")"; fi
    if [[ "${CACHED_SDK_SIGNATURE}" == "${SDK_SOURCE_SIGNATURE}" && -d "${SDK_CACHE_DIRECTORY}" ]]; then return; fi

    echo "Staging macOS SDK ${SDK_VERSION} in the ignored Docker-readable build cache."
    mkdir -p "$(dirname "${SDK_CACHE_DIRECTORY}")"
    STAGED_SDK_DIRECTORY="$(mktemp -d "${BUILD_CACHE_DIRECTORY}/macos-sdk/.${SDK_VERSION}.new.XXXXXX")"
    rmdir "${STAGED_SDK_DIRECTORY}"
    PREVIOUS_SDK_DIRECTORY="$(mktemp -d "${BUILD_CACHE_DIRECTORY}/macos-sdk/.${SDK_VERSION}.previous.XXXXXX")"
    rmdir "${PREVIOUS_SDK_DIRECTORY}"
    STAGED_SDK_STAMP_PATH="${SDK_CACHE_STAMP_PATH}.new.$$"
    cp -R "${SDK_SOURCE_DIRECTORY}" "${STAGED_SDK_DIRECTORY}"
    printf '%s\n' "${SDK_SOURCE_SIGNATURE}" > "${STAGED_SDK_STAMP_PATH}"
    if [[ -e "${SDK_CACHE_DIRECTORY}" ]]; then mv "${SDK_CACHE_DIRECTORY}" "${PREVIOUS_SDK_DIRECTORY}"; fi
    if ! mv "${STAGED_SDK_DIRECTORY}" "${SDK_CACHE_DIRECTORY}"; then
        [[ ! -e "${PREVIOUS_SDK_DIRECTORY}" ]] || mv "${PREVIOUS_SDK_DIRECTORY}" "${SDK_CACHE_DIRECTORY}"
        fail "Could not stage the macOS SDK in ${SDK_CACHE_DIRECTORY}."
    fi
    mv "${STAGED_SDK_STAMP_PATH}" "${SDK_CACHE_STAMP_PATH}"
    rm -rf "${PREVIOUS_SDK_DIRECTORY}"
}

ensure_fyne_tool() {
    # Variable group: cached native Fyne packager.
    local FYNE_TOOL_DIRECTORY="${BUILD_CACHE_DIRECTORY}/tools/fyne-${FYNE_TOOL_VERSION}-${DOCKER_HOST_ARCHITECTURE}"
    FYNE_COMMAND_PATH="${FYNE_TOOL_DIRECTORY}/fyne"
    if [[ ! -x "${FYNE_COMMAND_PATH}" ]]; then
        BUILD_STAGE_DIRECTORY="$(new_stage_directory fyne-tool)"
        echo "Building project-local Fyne CLI ${FYNE_TOOL_VERSION}."
        run_build_container mac "${BUILD_STAGE_DIRECTORY}/export" '' '' fyne-tool '' "${DOCKER_HOST_ARCHITECTURE}"
        [[ -x "${BUILD_STAGE_DIRECTORY}/export/fyne" ]] || fail 'Fyne CLI was not exported.'
        mkdir -p "${FYNE_TOOL_DIRECTORY}"
        mv "${BUILD_STAGE_DIRECTORY}/export/fyne" "${FYNE_COMMAND_PATH}"
        rm -rf "${BUILD_STAGE_DIRECTORY}"
        BUILD_STAGE_DIRECTORY=""
    fi
    "${FYNE_COMMAND_PATH}" version
}

package_macos_arch() {
    # Variable group: packaging inputs and staging directories.
    local TARGET_ARCHITECTURE="$1" SOURCE_BINARY_PATH="$2" STAGED_DISTRIBUTION_DIRECTORY="$3" PACKAGE_STAGE_DIRECTORY="$4"
    local PACKAGE_SOURCE_DIRECTORY="${PACKAGE_STAGE_DIRECTORY}/package-source" PACKAGE_OUTPUT_DIRECTORY="${PACKAGE_STAGE_DIRECTORY}/package-output" DMG_CONTENTS_DIRECTORY="${PACKAGE_STAGE_DIRECTORY}/dmg-source"
    # Variable group: public filenames and Fyne's temporary bundle name.
    local BINARY_FILENAME="${APPLICATION_NAME}_${BUILD_VERSION}_darwin_${TARGET_ARCHITECTURE}"
    local DMG_FILENAME="${APPLICATION_DISPLAY_NAME}_${BUILD_VERSION}_darwin_${TARGET_ARCHITECTURE}.dmg"
    local SAFE_APPLICATION_NAME='Application'
    local GENERATED_BUNDLE_DIRECTORY="${PACKAGE_OUTPUT_DIRECTORY}/${SAFE_APPLICATION_NAME}.app" BUNDLE_PLIST_PATH="${PACKAGE_OUTPUT_DIRECTORY}/${SAFE_APPLICATION_NAME}.app/Contents/Info.plist"
    mkdir -p "${PACKAGE_SOURCE_DIRECTORY}/$(dirname "${APPLICATION_ICON_PATH}")" "${PACKAGE_OUTPUT_DIRECTORY}" "${DMG_CONTENTS_DIRECTORY}" "${STAGED_DISTRIBUTION_DIRECTORY}"
    cp "${REPOSITORY_ROOT}/FyneApp.toml" "${PACKAGE_SOURCE_DIRECTORY}/"
    cp "${REPOSITORY_ROOT}/${APPLICATION_ICON_PATH}" "${PACKAGE_SOURCE_DIRECTORY}/${APPLICATION_ICON_PATH}"
    printf 'package main\nfunc main() {}\n' > "${PACKAGE_SOURCE_DIRECTORY}/main.go"
    cp "${SOURCE_BINARY_PATH}" "${PACKAGE_SOURCE_DIRECTORY}/${APPLICATION_NAME}"
    (
        cd "${PACKAGE_OUTPUT_DIRECTORY}"
        GOOS=darwin GOARCH="${TARGET_ARCHITECTURE}" "${FYNE_COMMAND_PATH}" package --os darwin \
            --executable "${PACKAGE_SOURCE_DIRECTORY}/${APPLICATION_NAME}" --source-dir "${PACKAGE_SOURCE_DIRECTORY}" \
            --name "${SAFE_APPLICATION_NAME}" --app-version "${BUILD_VERSION}" --release
    )
    [[ -d "${GENERATED_BUNDLE_DIRECTORY}" ]] || fail "Fyne did not create ${SAFE_APPLICATION_NAME}.app."
    plutil -lint "${BUNDLE_PLIST_PATH}"
    plutil -replace CFBundleName -string "${APPLICATION_DISPLAY_NAME}" "${BUNDLE_PLIST_PATH}"
    plutil -replace LSApplicationCategoryType -string public.app-category.utilities "${BUNDLE_PLIST_PATH}"
    plutil -replace LSMinimumSystemVersion -string "${MACOS_MINIMUM_VERSION}" "${BUNDLE_PLIST_PATH}"
    plutil -lint "${BUNDLE_PLIST_PATH}"
    [[ "$(plutil -extract CFBundleName raw "${BUNDLE_PLIST_PATH}")" == "${APPLICATION_DISPLAY_NAME}" ]] || fail 'The bundle name is invalid.'
    [[ "$(plutil -extract CFBundleIdentifier raw "${BUNDLE_PLIST_PATH}")" == "${APPLICATION_IDENTIFIER}" ]] || fail 'The bundle identifier is invalid.'
    [[ "$(plutil -extract CFBundleExecutable raw "${BUNDLE_PLIST_PATH}")" == "${APPLICATION_NAME}" ]] || fail 'The bundle executable metadata is invalid.'
    [[ "$(plutil -extract CFBundleShortVersionString raw "${BUNDLE_PLIST_PATH}")" == "${BUILD_VERSION}" ]] || fail 'The bundle version is invalid.'
    [[ "$(plutil -extract LSMinimumSystemVersion raw "${BUNDLE_PLIST_PATH}")" == "${MACOS_MINIMUM_VERSION}" ]] || fail 'The bundle minimum macOS version is invalid.'
    [[ "$(plutil -extract LSApplicationCategoryType raw "${BUNDLE_PLIST_PATH}")" == public.app-category.utilities ]] || fail 'The bundle category is invalid.'
    [[ -x "${GENERATED_BUNDLE_DIRECTORY}/Contents/MacOS/${APPLICATION_NAME}" ]] || fail 'The bundle executable is missing.'
    cmp -s "${SOURCE_BINARY_PATH}" "${GENERATED_BUNDLE_DIRECTORY}/Contents/MacOS/${APPLICATION_NAME}" || fail 'The bundle does not contain the validated binary.'
    ditto "${GENERATED_BUNDLE_DIRECTORY}" "${DMG_CONTENTS_DIRECTORY}/${APPLICATION_DISPLAY_NAME}.app"
    ln -s /Applications "${DMG_CONTENTS_DIRECTORY}/Applications"
    hdiutil create -quiet -srcfolder "${DMG_CONTENTS_DIRECTORY}" -volname "${APPLICATION_DISPLAY_NAME}" \
        -fs HFS+ -format UDZO "${STAGED_DISTRIBUTION_DIRECTORY}/${DMG_FILENAME}"
    hdiutil verify -quiet "${STAGED_DISTRIBUTION_DIRECTORY}/${DMG_FILENAME}"
    [[ "$(hdiutil imageinfo -format "${STAGED_DISTRIBUTION_DIRECTORY}/${DMG_FILENAME}")" == UDZO ]] || fail 'The disk image is not compressed as UDZO.'
    rm -rf "${GENERATED_BUNDLE_DIRECTORY}" "${DMG_CONTENTS_DIRECTORY}/${APPLICATION_DISPLAY_NAME}.app"
    mv "${SOURCE_BINARY_PATH}" "${STAGED_DISTRIBUTION_DIRECTORY}/${BINARY_FILENAME}"
}

run_mac() (
    # Variable group: validated binaries and shared OSXCross cache.
    local TARGET_ARCHITECTURE="" BINARY_FILENAME="" SOURCE_BINARY_PATH="" EXPECTED_MACHO_ARCHITECTURE="" BINARY_MINIMUM_MACOS_VERSION="" BINARY_EXPORT_DIRECTORY="" OSXCROSS_CACHE_DIRECTORY=""
    parse_build_args mac 'amd64 arm64' "$@"
    [[ "$(uname -s)" == Darwin ]] || fail 'macOS builds and .app packaging require a Darwin host.'
    local REQUIRED_TOOL=""
    for REQUIRED_TOOL in xcrun plutil lipo otool ditto hdiutil shasum; do require_command "${REQUIRED_TOOL}"; done
    require_docker_buildx
    resolve_docker_host_arch
    build_toolchain_image mac
    BUILD_STAGE_DIRECTORY=""
    trap cleanup_stage EXIT
    stage_macos_sdk
    MACOS_TOOLCHAIN_CACHE_KEY="$(mac_toolchain_cache_key "${SDK_SOURCE_SIGNATURE}")"
    OSXCROSS_CACHE_DIRECTORY="${BUILD_CACHE_DIRECTORY}/osxcross/${MACOS_TOOLCHAIN_CACHE_KEY}"
    ensure_fyne_tool

    # A single container builds both requested architectures and shares its caches.
    BUILD_STAGE_DIRECTORY="$(new_stage_directory mac-binaries)"
    BINARY_EXPORT_DIRECTORY="${BUILD_STAGE_DIRECTORY}/export"
    echo "Building macOS architectures ${BUILD_ARCHITECTURES[*]} with SDK ${SDK_VERSION}."
    run_build_container mac "${BINARY_EXPORT_DIRECTORY}" "${SDK_CACHE_DIRECTORY}" "${OSXCROSS_CACHE_DIRECTORY}" mac "${BUILD_VERSION}" "${BUILD_ARCHITECTURES[@]}"
    for TARGET_ARCHITECTURE in "${BUILD_ARCHITECTURES[@]}"; do
        BINARY_FILENAME="${APPLICATION_NAME}_${BUILD_VERSION}_darwin_${TARGET_ARCHITECTURE}"
        SOURCE_BINARY_PATH="${BINARY_EXPORT_DIRECTORY}/${BINARY_FILENAME}"
        [[ -x "${SOURCE_BINARY_PATH}" ]] || fail "Container did not export ${BINARY_FILENAME}."
        EXPECTED_MACHO_ARCHITECTURE="${TARGET_ARCHITECTURE}"
        [[ "${TARGET_ARCHITECTURE}" != amd64 ]] || EXPECTED_MACHO_ARCHITECTURE=x86_64
        [[ "$(lipo -archs "${SOURCE_BINARY_PATH}")" == "${EXPECTED_MACHO_ARCHITECTURE}" ]] || fail "Unexpected architecture for ${BINARY_FILENAME}."
        BINARY_MINIMUM_MACOS_VERSION="$(otool -l "${SOURCE_BINARY_PATH}" | awk '$1 == "cmd" && $2 == "LC_BUILD_VERSION" { FOUND_BUILD_VERSION=1; next } FOUND_BUILD_VERSION && $1 == "minos" { print $2; exit }')"
        [[ "${BINARY_MINIMUM_MACOS_VERSION}" == "${MACOS_MINIMUM_VERSION}" ]] || fail "${BINARY_FILENAME} targets macOS '${BINARY_MINIMUM_MACOS_VERSION}', expected ${MACOS_MINIMUM_VERSION}."
    done
    for TARGET_ARCHITECTURE in "${BUILD_ARCHITECTURES[@]}"; do
        BINARY_FILENAME="${APPLICATION_NAME}_${BUILD_VERSION}_darwin_${TARGET_ARCHITECTURE}"
        local PACKAGE_STAGE_DIRECTORY="${BUILD_STAGE_DIRECTORY}/package-${TARGET_ARCHITECTURE}"
        mkdir -p "${PACKAGE_STAGE_DIRECTORY}"
        package_macos_arch "${TARGET_ARCHITECTURE}" "${BINARY_EXPORT_DIRECTORY}/${BINARY_FILENAME}" "${PACKAGE_STAGE_DIRECTORY}/distribution" "${PACKAGE_STAGE_DIRECTORY}"
        install_distribution "${PACKAGE_STAGE_DIRECTORY}/distribution" mac "${TARGET_ARCHITECTURE}"
        echo "macOS/${TARGET_ARCHITECTURE} distribution: ${DISTRIBUTION_DIRECTORY}"
    done
)
