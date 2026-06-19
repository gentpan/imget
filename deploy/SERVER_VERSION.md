# Server Version Snapshot

This records the production state observed on `91.99.237.99` on 2026-06-19.

## Git state

- Local branch: `main`
- GitHub default branch: `main`
- Local/GitHub commit: `44ce933ac5f78f66dec5c2c525a0027496501d23`

## Production services

- `imget.service`: `/opt/imget/imget serve`
- `imget-format-proxy.service`: `/usr/bin/python3 /opt/imget-format-proxy/imget-format-proxy.py`

The deployed service files are tracked in `deploy/systemd/`.
The deployed proxy script is tracked in `deploy/imget-format-proxy.py`.

## Production binary

Observed binary:

```text
/opt/imget/imget
size: 26928200 bytes
sha256: daa9e17a8e8b8003e26d181483e34bf7086a54d7124842c6937b1397b387eaa9
mtime: 2026-06-11 19:53 UTC
Go: go1.26.2
module: imget (devel)
```

The server did not contain an `imget` Git checkout, Go source tree, source
archive, or patch that could recover differences from the deployed binary.

Rebuilding GitHub commit `44ce933ac5f78f66dec5c2c525a0027496501d23` on the
server produced a different binary checksum. The dependency versions and command
surface matched, but the exact deployed Go source/build cannot be proven from
the binary alone.
