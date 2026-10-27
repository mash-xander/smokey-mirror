# Security model

Smokey Mirror is a new application around the maintained `filippo.io/age`
library. Its application code has not had an independent security audit.

## Cryptography

Files use the standard binary age v1 format, not a custom container. `.smoke`
only changes the filename. The age library provides streaming authenticated
encryption with ChaCha20-Poly1305, fresh file keys, and authenticated framing.
No application-defined nonce scheme or encryption primitive is used.

- Passphrases use scrypt with log2(N) = 18, r = 8, p = 1 and fresh salts.
- Key files contain a random X25519 secret identity. age handles recipient
  wrapping and HKDF. Key files are limited to 16 KiB and exactly one X25519
  identity; arbitrary files, SSH identities, and plugins are not accepted.
- Decryption caps scrypt work at 18 to bound untrusted-header memory requests.
  Higher-cost age files must be opened using the upstream age CLI.
- Only binary age files are supported; ASCII-armored files are not accepted.
- Old or externally generated short passphrases are accepted for decryption.

The format and library are described at
[age-encryption.org](https://age-encryption.org/) and
[C2SP](https://c2sp.org/age).

## Output safety

Input files are opened read-only. Output is streamed to a randomly named
`.smokey-*.tmp` file in the destination directory. On Windows, before any
content is written, a protected DACL restricts access to the current user and
SYSTEM. Failure to establish these permissions aborts the operation. On other
platforms, temporary files use mode 0600.

Only after all authentication tags and the final stream marker have been
verified, the input stability check has passed, and the result has been flushed
and closed, is the temporary file published. Windows uses a same-directory
non-replacing rename. Unix uses an exclusive hard link followed by removal of
the temporary name. Destination creation races cannot overwrite an existing
file. Filenames inside ciphertext are never interpreted as output paths.

Ordinary failures and cooperative cancellation remove temporary files. A
process kill, power failure, disk failure, or OS crash can leave a temporary
file, including partially decrypted plaintext. Deletion is not secure erasure
and SSDs, backups, snapshots, antivirus, and sync software can retain copies.
Use a trusted local NTFS destination (and disk encryption when appropriate).
Filesystems that cannot apply the required Windows permissions are rejected;
FAT/exFAT destinations may therefore be unsuitable. Keep enough free space for
the complete output. Original files are never deleted or securely wiped.

## Secrets and limits

There is no network communication, telemetry, file index, password persistence,
or secret logging in the app. No secret is accepted in command-line arguments.
GUI secret fields are cleared after each operation. Terminal byte buffers are
cleared, but Go strings and runtime copies cannot be reliably zeroed, locked,
or excluded from swap and crash dumps. This is not protection against malware,
debuggers, administrators, keyloggers, or a compromised operating system.

The application protects file contents. Names, approximate sizes, timestamps,
and method information in the age header are visible. File metadata, alternate
data streams, permissions, and timestamps are not archived or restored. Use an
archive first if those are part of your intended backup. Do not edit source
files during an operation; size and modification-time checks catch ordinary
changes but are not a filesystem snapshot or adversarial locking mechanism.

Cancellation is checked during streaming and before publication. A running
scrypt computation cannot be interrupted; cancellation completes after that
bounded computation finishes.

The installer uses only the current user's registry and directories. Windows
PowerShell is used for a Start menu shortcut and deferred removal of the running
installed executable; paths are quoted as literal strings. Installation rollback
and PATH preservation are tested. An app update currently requires uninstalling
the previous version. The application binaries have no code-signing certificate.

## Reporting

Do not publish real keys, passphrases, or private files with an issue. For a
security report, use private vulnerability reporting on the repository host
once a public repository and contact channel have been configured. Neither is
configured by this local project.
