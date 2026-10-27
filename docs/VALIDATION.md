# Validation notes

Initial release validation on Windows x64, 2026-10-27.

- `go test ./...`: passed, including isolated installation/rollback tests.
- `go vet ./...`: passed.
- Vault statement coverage: 85.2%; interactive GUI behavior is tested separately.
- Passphrase encryption and decryption were exercised in the native window;
  SHA-256 hashes of the original and recovered test file matched.
- A key was created using the native save dialog, then used for an executable
  encryption/decryption round trip with matching SHA-256 hashes.
- Both release executables were built. `scripts/smoke.ps1` passed against the
  release binary, including tampering and no-overwrite checks.
- `scripts/install-smoke.ps1` passed: actual per-user installation, installed
  executable launch, self-removal, registry and shortcut cleanup, and exact
  restoration of the original user PATH. The test left the app uninstalled.
- Corrupt payloads, wrong credentials, empty files, chunk boundaries, interrupted
  operations, changing inputs, competing writes, and failed writes are covered.
- Windows output access control is checked for a protected DACL with the current
  user's SID; temporary outputs are removed on normal errors.
- PATH tests cover existing entries, absent values, trailing separators, type
  preservation, external changes, and rollback after shortcut creation failure.
- `govulncheck` reports no reachable vulnerabilities and no vulnerabilities in
  imported packages. It flags the unused, unmaintained `x/crypto/openpgp` package
  at module level. Smokey Mirror does not import OpenPGP or SSH.

The Linux race-detector job is configured in CI; it has not run on a hosted
runner for this local repository. This is functional testing, not an independent
cryptographic or application security audit.
