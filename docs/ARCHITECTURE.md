# Architecture

```text
cmd/smokey       CLI dispatch and desktop entry point
cmd/setup        GUI setup, with an embedded release executable
internal/vault   Credentials, age streaming, atomic publication, permissions
internal/cli     Argument parsing, terminal prompts, automation input
internal/ui      Native Walk window, progress, cancellation, safe name suggestions
internal/install Per-user Windows registration, PATH, shortcut, removal
assets           Manifest and generated application icon
scripts          Reproducible build and executable smoke checks
```

The GUI and CLI call the same `vault.Transform` operation. The UI owns a context
and processes the file on a worker goroutine; all widget changes are dispatched
back onto the window thread. A progress callback reports consumed input bytes.
100% is displayed only after successful publication. Closing a busy window
cancels first and waits for the operation to finish.

The vault accepts one input path, an optional output path, a direction, and
exactly one credential type. It does not know about dialogs, shell integration,
labels, or the registry. The CLI emits the output path only after success.

## Decisions

- **age rather than a custom AES envelope:** a documented interoperable format
  with nonce management, chunk authentication, and recipient handling supplied
  by the reference library. ChaCha20-Poly1305 is the content cipher.
- **Native Windows controls:** one small executable without WebView2, a local
  HTTP server, Electron, or a bundled browser. Walk is isolated in the UI package.
- **Console-subsystem executable:** correct synchronous cmd/PowerShell behavior,
  redirection, and exit codes. GUI startup detaches the console. A brief console
  flash on Explorer launch is an accepted first-release tradeoff.
- **Separate installer build:** the same Go project embeds the portable binary
  in a setup exe. No extra installer compiler or installer runtime is required.
- **No automatic file deletion:** successful encryption produces a copy; deletion
  cannot promise secure erasure, especially on SSDs.
- **No global file catalog:** portable mode has no persistent application data.
- **No default association takeover:** install adds explicit shell verbs without
  changing the user's file-opening defaults.

## Extending labels

Keep labels outside the cryptographic engine. A later opt-in catalog can map
user-selected encrypted paths to labels and a non-secret key fingerprint. A
batch coordinator should obtain the key once, run the same vault operation for
each selected file, and report each result independently. Never store secrets
in the catalog or silently enumerate the disk. Moving files and stale catalog
entries need explicit handling. See [ROADMAP.md](ROADMAP.md).
