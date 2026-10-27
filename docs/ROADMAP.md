# Roadmap

## Next: labels and batch decryption

- Opt-in local labels over explicitly selected files.
- A reviewable list before bulk decryption, one key selection per batch.
- Independent output names, cancellation, and error reporting per file.
- Moving and missing files handled without automatic disk-wide discovery.
- Atomic catalog updates and an explicit clear-catalog action.
- Tests for mixed keys, stale paths, duplicates, partial failures, and cancellation.

No label or catalog fields are written into v1 encrypted files. Existing `.smoke`
files remain valid standard age files.

## Later

- Signed binaries and release publishing after a repository and signing identity exist.
- Installer upgrades and optional individual integration choices.
- Modern Windows 11 context-menu integration (the current version uses the classic menu).
- ARM64 resources and native-device verification.
- Additional UI languages and accessibility testing at more display scales.
- Independent review of application-level file handling and installation.

