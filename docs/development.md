# Development guide

File & Folder Renamer uses [Fyne](https://fyne.io/) for its desktop interface. Rename, pattern, collision, and rollback behavior lives in UI-independent Go packages. Run commands from the repository root.

## Code flow

The UI sends a preview request to `internal/organizer`. That service scans the selected tree, builds names with the selected pattern, validates the proposals, and seals a copy for display. Apply rebuilds the preview from the sealed request and rejects it if the proposal set changed. `internal/apply` then stages sibling renames, promotes them in depth order, and attempts rollback if a move fails. Settings write complete JSON documents through `internal/atomicfile`; the pattern library uses its multi-file transaction to restore prior files if a write fails.

Keep core rename validation and filesystem mutations in these UI-independent packages. The UI owns presentation state, background scan callbacks, and live translation; saved pattern JSON and filenames use stable values that are not translated.

## Development setup

The canonical Go module is `github.com/eightaugusto/file-folder-renamer` and requires Go 1.22 or newer.

Native development uses plain Go commands and [Fyne's platform prerequisites](https://docs.fyne.io/started/quick/) (a C compiler and graphics development libraries). [Release builds](release-builds.md) require Docker with Buildx. The macOS builder additionally requires a Mac with Xcode Command Line Tools because it supplies the licensed Apple SDK and performs native `.app` and `.dmg` packaging.

After installing the native development requirements, run unit tests and start the app:

```sh
make application.start
```

Run all domain, workflow, persistence, and headless UI tests:

```sh
make test.unit
```

Build the multi-architecture binaries and macOS DMGs:

```sh
make application.build
```

This builds Linux and Windows binaries, cross-compiles macOS binaries for amd64 and arm64, and packages each macOS app inside a `.dmg`. Release files are placed directly in the flat `build/dists/` directory; standalone `.app` bundles are removed after the DMGs are verified. On macOS, open the DMG and launch or install the app inside it rather than the raw binary.

## Desktop layout and visual checks

Organizer, Pattern Studio, and Settings share a leading sidebar. Below 1180 logical points, the same navigation controls move into a compact top row. Navigation and resizing preserve workspace instances, drafts, previews, filters, and scroll positions. Toolbars wrap as needed, and narrow rule forms move their labels above the fields.

Organizer places scan feedback beside its folder controls and preview counts beside **Apply all changes**. Action, Kind, and Status remain fixed at the leading edge of the table; their widths include translated labels and controls. Descriptive columns retain their 30-character defaults, shrink proportionally, and give surplus width to Location. Use the heading to sort and its separate menu button to select filters; both support keyboard focus and Space activation.

The opt-in screenshot fixture exports all three workspaces, both rule editors, Help, filters, dropdowns, scanning feedback, and confirmation dialogs in both languages and themes at 1024×768, 1280×800, and 1920×1080. It uses temporary settings and synthetic proposals:

```sh
UI_CAPTURE_DIR="$PWD/build/.cache/ui-captures/after" go test ./internal/ui -run TestUIVisualFixtures -count=1
```

The normal UI suite covers requested window sizes, navigation and toolbar reflow, translated table measurements, keyboard navigation/sorting/filtering, Help dismissal, text contrast, and preservation of drafts, unfinished input, scans, and scrolling during language changes. Rendered fixtures do not verify native window decorations, OS folder-picker integration, display scaling, or platform keyboard behavior; check those on macOS and Windows before release.

## Adding a language

Translations use `go-i18n`, which is already a Fyne dependency. Go's standard library provides JSON and embedding support but no complete message-catalog or pluralization system. We use `go-i18n` directly because Fyne's localization API does not expose a user-selected language override.

1. Copy `internal/localization/translations/en-US.json` to a file named for the new BCP 47 locale, such as `fr-FR.json`, and translate its values. Keep message IDs, template parameters, regexes, and example input/output pairs unchanged. Long Help sections are catalog entries too.
2. Add the locale and its native display name to the `supported` registry in `internal/localization/localization.go`. This registers it for config validation, system-language matching, embedded loading, and the Settings selector.
3. Add the locale to the top-level `Languages` list in `FyneApp.toml` for native packages.
4. Run `make test.unit` and `make application.build`, then check the UI in that language. Tests check matching catalog keys, valid templates, and preserved interpolation arguments. Include the plural forms required by the new language, with `other` as the fallback.

Use semantic IDs for new messages, such as `studio.pattern_name`. Pass dynamic values through template data (`{{.Name}}`), and use the translator's `Plural` method for count-dependent messages. Use the view's text bindings for persistent control labels so they participate in live language refresh; dynamic editors own a separate binding group that is replaced when the editor changes. Refresh translated dropdowns without calling their change callbacks. Catalogs are embedded in the binary and are never copied into the custom-pattern folder. Rename and pattern packages do not depend on the UI translator.
