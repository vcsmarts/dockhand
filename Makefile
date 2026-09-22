GO      ?= go
BINDIR  ?= $(HOME)/.local/bin

.PHONY: build test install clean

build:
	$(GO) build -o dockhand .

test:
	$(GO) vet ./...
	$(GO) test ./...

install: build
	./dockhand setup --bin $(BINDIR)

clean:
	rm -f dockhand
