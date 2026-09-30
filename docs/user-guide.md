# User guide

This guide covers the Organizer, Pattern Studio, and application settings. The [README](../README.md) has the quickest route to downloading and trying the app.

## Features

- Recursive file and folder previews with unchanged and changed counts
- Configurable ignored-name list with conservative OS metadata and temporary-file defaults
- Explicit confirmation before applying renames
- Stale-preview detection immediately before applying
- Protection against overwriting unchanged targets
- Staged rename chains, swaps, case-only changes, and nested folders
- Best-effort rollback if an operation fails partway through a batch
- Visual Pattern Studio with rule pipeline/settings above saved test cases
- Saved file and folder test cases as the sole visual pattern validation
- Home-directory `config.json` with an absolute custom-pattern path
- Read-only built-in patterns and separate JSON files for custom tests
- Apply All plus guarded per-row rename actions
- Adaptive sidebar/top navigation, light and dark themes, and keyboard shortcuts
- English (United States) and Spanish (Mexico), with system detection and a saved language preference
- Localized About dialog showing the version embedded in the running application

## Using the Organizer

1. Open **Organizer** and select **Choose** to open Fyne's standard folder chooser.
2. The application immediately scans the selected folder and displays the proposed names.
3. Select files, folders, or both, or choose a different saved pattern; the preview refreshes automatically.
4. Resolve any reported collision or occupied target and let the preview refresh.
5. Select **Apply all changes** to execute the safe batch, or use **Apply** on one changed row to rename only that item.

Changing the folder, target types, or pattern invalidates the current preview. Immediately before applying, the application rescans and compares the proposed names and paths. If that proposal set changed, no rename begins and a fresh preview is required. File contents and modification times are not part of this check.

Once mutation begins it cannot be canceled. The app stages sibling renames under unique temporary names, applies deeper paths before their parents, and records a reverse journal. If a later rename fails, completed operations are rolled back where the filesystem permits it. A process crash or rollback failure cannot be made fully atomic by a portable filesystem API and is reported explicitly.

Individual actions perform the same stale-preview and occupied-target checks. A rename that depends on another row, such as a swap or chain, may require **Apply all changes** so the engine can stage the complete group safely.

## Pattern Studio

Pattern Studio can add, remove, reorder, and configure these rules:

- `case`
- `replace_runes`
- `remove_diacritics`

`remove_diacritics` removes decomposable Unicode marks from letters in enabled
scripts. It currently provides a `latin` flag and preserves characters that
require transliteration, such as `ø`, `ł`, `æ`, and `ß`. At least one script
flag must be enabled.

`replace_runes` can also contain `exclude_matches`: a list of regular expressions
whose matched spans are left unchanged by that one rule. This is useful when a
general delimiter rule should leave structured text intact. The compiled default
pattern, for example, formats ordinary hyphens with spaces while preserving
two- or three-part numeric dates. Their year can contain two or four digits and
can appear in any position, such as `25-07`, `07-2025`, or `05-12-2025`.
Exclusions do not stop later rules from processing the same text. Pattern Studio
presents these entries as a validated table; invalid or duplicate expressions
cannot be added.

Saved test cases are the Pattern Studio validation interface. Each case records an input, its explicit `file` or `folder` kind, and a read-only expected name calculated automatically when the case is added. The expected value then remains fixed while the Actual and Result columns recalculate after every rule change. Every saved case must pass before the pattern can be committed to the library.

Built-in patterns are read-only: they cannot be edited, replaced, or deleted. Their compiled validation cases, including the complete default-pattern fixture, appear as read-only saved tests. Duplicate one to create a new editable pattern with a different name. Selecting **New** or **Duplicate** prompts for the new pattern name; there is no separate name field. Unsaved changes must be saved or discarded before switching patterns, and closing the app with a draft prompts for a decision.

On first launch, the app creates this layout in your home directory. The final
folder name is taken from the executable's build name (for example,
`file-folder-renamer`):

```text
~/.eightaugusto/file-folder-renamer/
  config.json
  patterns/
    my-pattern.json
    my-pattern.tests.json
```

Built-in patterns such as `default` exist only inside the compiled binary and are never copied into the resource folder. The `patterns` directory contains custom patterns only.

`~/.eightaugusto/<build-name>/config.json` contains the absolute custom-pattern directory and Organizer defaults. It has no version, resource path, or default-pattern field. The **Settings** workspace presents one **Settings folder** location; it contains `config.json`, and its sibling `patterns/` folder is managed automatically. Selecting another folder validates its existing `config.json` before filling the form, and activates it only after **Save settings** succeeds. If it has no `config.json`, Save creates one with defaults.

```json
{
  "patterns_path": "/absolute/path/to/.eightaugusto/file-folder-renamer/patterns",
  "language": "system",
  "include_files": true,
  "include_folders": true,
  "ignored_names": [
    ".DS_Store",
    "._*",
    ".AppleDouble",
    ".LSOverride",
    ".Spotlight-V100",
    ".TemporaryItems",
    ".Trashes",
    ".fseventsd",
    "Thumbs.db",
    "ehthumbs.db",
    "Desktop.ini",
    "$RECYCLE.BIN",
    "*.stackdump",
    ".directory",
    ".Trash-*",
    ".nfs*",
    "*~"
  ]
}
```

Ignored names support filename wildcards and are matched case-insensitively. Matching folders and their contents are excluded from traversal. Settings and regex exclusions show explicit Delete buttons in their tables.

Each custom pattern has a definition JSON and a separate `.tests.json` file. The test file is created even when it has no cases and stores both file and folder cases. Invalid or corrupt custom files are reported and left untouched while compiled built-in patterns remain available.

For example, `my-pattern.tests.json` contains:

```json
{
  "version": 1,
  "pattern": "my pattern",
  "cases": [
    { "input": "FILE.TXT", "kind": "file", "expected": "file.txt" },
    { "input": "FOLDER", "kind": "folder", "expected": "folder" }
  ]
}
```

## UI language

In **Settings > Language**, choose **System default**, **English (United States)**, or **Español (México)**, then select **Save settings**. The UI updates immediately without restarting. A language-only save preserves unsaved pattern drafts (including unfinished form input), previews, scans, table selections and filters. Other settings changes still require saving or discarding a pattern draft first, and settings cannot be saved while a rename is running.

Existing modal dialogs keep their language until dismissed; newly opened dialogs use the current language. An open table-filter popup refreshes its labels while preserving unfinished checkbox selections.

The `language` field in application `config.json` accepts `system`, `en-US`, and `es-MX`. Older files without the field use `system`. English system locales use `en-US`, Spanish locales use `es-MX`, and other locales fall back to `en-US`. Selecting another settings folder also loads its language preference into the form.

Translation applies to application controls, enum labels, Help, and messages. Filenames, custom pattern names and descriptions, regexes, test data, and JSON identifiers stay as entered. Fyne's standard folder-picker controls follow the system locale; technical error details retain their original text.

## Pattern import and export

Pattern Studio imports and exports a pattern bundle. This is separate from the application’s `config.json`: an export contains the selected pattern name, current Organizer file and folder choices, and all custom patterns with their tests. Import adds those patterns to the current library, selects the named pattern if it is available, and sets the Organizer file and folder choices. It does not replace Settings defaults, ignored names, or language. Test cases are optional, so bundles without them remain valid:

```json
{
  "pattern": "slug",
  "files": true,
  "folders": true,
  "patterns": [
    {
      "name": "slug",
      "rules": [
        { "kind": "case", "mode": "lower" },
        {
          "kind": "replace_runes",
          "remove": [" "],
          "replacement": "-"
        }
      ],
      "tests": [
        { "input": "My File.TXT", "kind": "file", "expected": "my-file.txt" },
        { "input": "My Folder", "kind": "folder", "expected": "my-folder" }
      ]
    }
  ]
}
```

Imports with an existing custom pattern name require an explicit choice: replace it, keep the existing definition, or import a renamed copy. A built-in conflict cannot be replaced; it can only be kept or imported as a copy.

## Configuration ownership

The application uses only its current configuration directory and Fyne application
ID. Startup creates fresh defaults when settings are absent; it does not import
settings, pattern libraries, or preferences from previous application identities.
Existing files elsewhere are left untouched. Settings require an absolute
`patterns_path`; embedded default patterns remain compiled into the executable.

Configuration roots follow the executable's build name. Version and architecture-suffixed
matrix binaries therefore use separate roots. macOS bundles contain the standard
`file-folder-renamer` executable and use `~/.eightaugusto/file-folder-renamer`.
To build a standalone executable with the standard name on macOS, run
`go build -trimpath -o build/.cache/dev/file-folder-renamer ./cmd/file-folder-renamer`.

## Built-in default pattern

The `default` pattern first removes decomposable diacritics from Latin letters. It then normalizes common delimiters, spaces and case, removes unpreserved special runes, lowercases file extensions, recognizes common compound archive extensions, and leaves protected dotfile stems unchanged.

The required default fixture in `internal/pattern/assets/test_pattern_default.json` contains more than 100 portable filename cases and remains part of the test suite.
