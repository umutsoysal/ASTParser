# Run the Go tests in a Linux container from a Windows host.
#
# cgo needs a C compiler, which a Windows box does not have, so the toolchain
# runs in Debian instead. The two named volumes persist Go's module and build
# caches between runs -- without them every run recompiles the tree-sitter C
# grammar, which is the difference between ~11s and ~0.3s.
#
# Usage:
#   .\scripts\test.ps1
#   .\scripts\test.ps1 -v
#   .\scripts\test.ps1 -run TestDefinitions

docker volume create go-mod-cache   | Out-Null
docker volume create go-build-cache | Out-Null

docker run --rm `
  -v "${PWD}:/w" -w /w `
  -v go-mod-cache:/go/pkg/mod `
  -v go-build-cache:/root/.cache/go-build `
  golang:1.24-bookworm go test ./... @args

exit $LASTEXITCODE
