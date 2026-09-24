# XBVR Synology package. `make` alone = vet + test. Run from this dir.
BINARY := xbvr
VERSION ?= 0.4.40
ARCH ?= avoton
ICON ?= ../ui/public/icons/xbvr-256.png

.PHONY: all vet test pack clean

all: vet test

vet:
	go vet ./...

test:
	go test ./...

# Assemble a DSM 7 .spk for one arch (needs a matching linux binary first):
#   make pack BINARY=dist/xbvr-linux-amd64 VERSION=0.4.40 ARCH=avoton
#
# The binary MUST be built with CGO against glibc <= 2.26 (DSM 7.1 runs
# 2.26; a builder-glibc binary dies with "GLIBC_2.x not found").
# Upstream's own release binary needs 2.28 and does NOT run. Use the
# SynoCommunity avoton DSM 7.1 toolchain, or an old-distro CI container
# (debian stretch = glibc 2.24) with: CGO_ENABLED=1 go build -tags json1 .
pack:
	go run ./tools/mkspk --binary $(BINARY) \
		--arch $(ARCH) --version $(VERSION) --rev 1 \
		--icon $(ICON) --out dist

clean:
	rm -rf dist
