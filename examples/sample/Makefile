# Сборка stripped executable + отдельный .debug для демо debuginfod.
BIN_DIR ?= bin
CC ?= gcc
CFLAGS = -g -O0 -Wall -Wextra

HELLO = $(BIN_DIR)/hello
DEBUG = $(HELLO).debug

.PHONY: all clean
all: $(HELLO)

$(HELLO): hello.c | $(BIN_DIR)
	$(CC) $(CFLAGS) -o $@ $<
	objcopy --only-keep-debug $@ $(DEBUG)
	strip --strip-debug $@

$(BIN_DIR):
	mkdir -p $@

clean:
	rm -rf $(BIN_DIR)
