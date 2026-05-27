# Build Notes

`go build ./...` fails in the Codex environment because Go module downloads are blocked by proxy/network policy (`GOPROXY`/direct access returns `403 Forbidden`).

This failure is environmental and not caused by repository code changes.

In a normal CI/deploy environment (for example Railway or GitHub Actions) with standard Go module network access, run:

```bash
cd koschei/api
go mod download
go build ./...
```
