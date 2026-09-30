# Release builds

These instructions describe how to build the platform artifacts locally. Run commands from the repository root. For published downloads, see the [Releases page](https://github.com/EightAugusto/file-folder-renamer/releases).

## Build requirements

Docker Desktop or Docker Engine with Buildx is required for release builds. One multi-stage Dockerfile creates separate rootless Linux, Windows, and macOS toolchain images from the official `golang:1.26.8-bookworm` image. Linux adds Fyne's graphics dependencies, Windows adds MinGW, and macOS adds LLVM and the pinned OSXCross source. The images contain toolchains only; application compilation occurs in containers started from them. Native development continues to require Go 1.22 or newer.

Each platform image runs as the named non-root user `eightaugusto`. `USER_ID` defaults to `1000`; the build script overrides it with the invoking host user's UID so mounted caches and outputs remain writable. The Dockerfile installs each platform's apt dependencies with its index update in the same layer, then checks `go version` as the builder user. The official Go image verifies its downloaded Go toolchain; the release build pulls the current image for the pinned Go version.

The macOS image adds LLVM's signing key and repository before updating apt. The Go base image already has CA certificates, so apt uses normal TLS peer verification and verifies LLVM's signed repository metadata.

macOS builds must be started on a Mac with Xcode Command Line Tools installed and its license accepted. The workflow obtains the selected SDK with `xcrun`; if its `libSystem.tbd` uses the `arm64e.x1` target unsupported by LLVM 21, it selects the newest compatible installed SDK or fails clearly. It stages a Docker-readable copy beneath ignored `build/.cache/macos-sdk/` and mounts it read-only. The SDK copy is refreshed when its selected path, version, directory timestamp, SDK settings, or `libSystem.tbd` changes. Apple `plutil`, `lipo`, `otool`, `ditto`, and `hdiutil` perform native validation and packaging. The SDK is never committed, copied into an image, or placed in `build/dists`. Do not publish the SDK or SDK-derived OSXCross cache.

### Build commands

The root Makefile's `application.build` target runs the Linux, Windows, and macOS build commands in that order, stopping if one fails:

```sh
# Native tests and local application
make test.unit
make application.start

# Complete release matrix
make application.build

# One platform, all of its supported architectures
./build/build.sh linux
./build/build.sh windows
./build/build.sh mac

# Selected architectures for one platform
./build/build.sh mac --arch amd64 --arch arm64
```

On a non-Mac host, invoke the Linux or Windows command directly; the macOS build requires a Mac for SDK access and native packaging.

You can also invoke `build/build.sh` directly with `linux`, `windows`, or `mac`, plus repeatable `--arch` options. Omit `--arch` to build all architectures supported by that platform. Repeating an architecture is harmless; unsupported architectures and malformed arguments fail before an existing distribution is touched.

`FyneApp.toml` is the source for the application's display name, identifier, icon path, and release version. Set `[Details].Version` before a release, for example `Version = "1.0.0"`; it must have the form `MAJOR.MINOR.PATCH` without a leading `v`. The build uses these fields for artifact names, embedded executable versions, icon staging, and macOS bundle checks. The `file-folder-renamer` executable name remains stable separately from the display name; the Go import path comes from `go.mod`. Git tags are optional release labels; if you create one, use Go's standard `vMAJOR.MINOR.PATCH` form matching the metadata. The build does not derive its version from tags or environment variables.

Development builds continue to derive their About-version text from Go build information: the VCS revision is shortened and a modified worktree is marked dirty. If build information is unavailable they display `development`.

### Builder layout and caching

```text
build/
  build.sh
  container.sh
  macos.sh
  toolchains.Dockerfile
  .cache/
  dists/
    file-folder-renamer_<version>_<platform>_<architecture>[.exe]
    File & Folder Renamer_<version>_darwin_<architecture>.dmg
```

Each platform command prepares its own stage from `toolchains.Dockerfile`. Every container sees the repository read-only at `/home/eightaugusto/source` and exports results through `/home/eightaugusto/out`. Ignored `build/.cache/go-mod/` and `build/.cache/go-build/` are mounted at the builder user's Go cache paths. The macOS command additionally mounts the staged SDK read-only and an ignored OSXCross cache. Linux and Windows builder images use `linux/amd64`; macOS uses `linux/arm64` and compiles both Darwin architectures in one container run when both are requested. Compiler temporary files remain inside the disposable container.

The macOS image pins OSXCross to commit `27d21e4977c9751d01199c7a226a6faf494c3dd9`. Its cache key includes the SDK signature, Go version, LLVM version, macOS 12.0 deployment target, and OSXCross revision. The native Fyne CLI remains pinned to v1.7.2 under `build/.cache/tools/` and packages the validated binaries without recompiling them. Packaging uses the metadata and icon plus a small source stub; `plutil` sets the final display name and macOS Utilities category. Each `.app` is placed in a compressed UDZO `.dmg` with an Applications shortcut. The DMG is verified before the temporary standalone app copies are deleted.

Docker's layer cache preserves dependency installation. Go module and compilation caches, the SDK-derived OSXCross toolchain, the staged SDK, and the Fyne CLI persist under ignored `build/.cache/` to speed up repeat builds. Delete that directory to clear the local build caches; Docker's image layers are cached separately. Generated artifacts are staged separately and replace only the selected platform/architecture after compilation, validation, and packaging succeed. If backing up or installing an artifact fails, the distribution installer restores the prior files.

### Build outputs and verification limits

For version `1.0.0`, the primary outputs are:

| Target | Distribution |
| --- | --- |
| Linux x64 | `build/dists/file-folder-renamer_1.0.0_linux_amd64` |
| Windows x64 | `build/dists/file-folder-renamer_1.0.0_windows_amd64.exe` |
| macOS Intel | `build/dists/File & Folder Renamer_1.0.0_darwin_amd64.dmg` |
| macOS Apple Silicon | `build/dists/File & Folder Renamer_1.0.0_darwin_arm64.dmg` |

The flat directory contains a raw macOS binary and a compressed `.dmg` for each macOS architecture. It contains no standalone `.app` bundles or `LICENSE` file. Each DMG contains the app and an Applications shortcut, with no `LICENSE` file in the app bundle. When distributing these artifacts, give recipients a copy of the repository's [LICENSE](../LICENSE) separately. Windows executables use the GUI subsystem. macOS bundles have escaped and validated plist metadata and retain the standard `file-folder-renamer` internal executable name.

Linux artifacts target Debian Bookworm's glibc 2.36 runtime baseline and require the corresponding OpenGL, X11, Wayland, and xkbcommon runtime libraries on the destination system.

The build verifies file format, architecture, Windows subsystem, macOS deployment target, and DMG integrity. It does not prove that graphics, native dialogs, scaling, or desktop integration work on the destination OS. Perform launch checks on macOS, Linux, and Windows before publishing. Signing, notarization, other installer formats, and publishing remain separate release steps.
