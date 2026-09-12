.PHONY: core cli gui test vet

# Builds third_party/astropng-core/lib/libastropng_core.a (needs cargo).
core:
	bash scripts/build-core.sh

cli: core
	CGO_ENABLED=1 go build -o astrogopng ./cmd/astrogopng

gui: core
	cd gui && CGO_ENABLED=1 go build -o ../astrogopng-gui .

test: core
	CGO_ENABLED=1 go test ./...

vet: core
	CGO_ENABLED=1 go vet ./...
