# syntax=docker/dockerfile:1

# The tree-sitter bindings are cgo: the Python grammar's parser.c is compiled
# into the binary. Every stage that runs the Go toolchain therefore needs a C
# compiler and CGO_ENABLED=1, and the runtime image needs the same libc as the
# builder — hence bookworm on both sides.

# ---------------------------------------------------------------- deps -----
FROM golang:1.24-bookworm AS deps
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# --------------------------------------------------------------- build -----
FROM deps AS build
ENV CGO_ENABLED=1
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -o /out/astparser .

# ---------------------------------------------------------------- test -----
# Build this stage to run the suite in the container:
#   docker build --target test --progress=plain .
FROM deps AS test
ENV CGO_ENABLED=1
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go vet ./... && go test ./... -v

# ------------------------------------------------------------- runtime -----
FROM debian:bookworm-slim AS runtime
RUN useradd --create-home --uid 10001 app
COPY --from=build /out/astparser /usr/local/bin/astparser
COPY --from=build /src/parser/testdata /opt/astparser/testdata
USER app
WORKDIR /work
ENTRYPOINT ["astparser"]
CMD ["/opt/astparser/testdata/sample.py"]
