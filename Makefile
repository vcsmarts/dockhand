GO      ?= go
BINDIR  ?= $(HOME)/.local/bin

.PHONY: build test install clean

build:
	$(GO) build -o dockhand .

test:
	$(GO) vet ./...
	$(GO) test ./...

install:
	$(GO) build -o $(BINDIR)/dockhand .
	$(BINDIR)/dockhand install --bin $(BINDIR)

clean:
	rm -f dockhand
