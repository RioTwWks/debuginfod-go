# Stripped Go executable + отдельный .debug (GNU build-id через external linker).
BIN_DIR ?= bin
GO ?= go
LDFLAGS = -linkmode=external
GCFLAGS = all=-N -l

HELLO = $(BIN_DIR)/hello
DEBUG = $(HELLO).debug
FULL = $(HELLO).full

.PHONY: all clean
all: $(HELLO)

$(HELLO): main.go | $(BIN_DIR)
	$(GO) build -gcflags="$(GCFLAGS)" -ldflags="$(LDFLAGS)" -o $(FULL) .
	objcopy --only-keep-debug $(FULL) $(DEBUG)
	objcopy --strip-debug $(FULL) $(HELLO)
	rm -f $(FULL)

$(BIN_DIR):
	mkdir -p $@

clean:
	rm -rf $(BIN_DIR)
