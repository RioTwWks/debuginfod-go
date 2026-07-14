# GDB init-скрипт для демонстрации загрузки debuginfo через debuginfod-go.
#
# Локально:
#   make -C examples/sample
#   DEBUGINFOD_URLS=http://localhost:8002 gdb -x examples/gdb/debug.gdb examples/sample/bin/hello
#
# Docker:
#   make -C examples demo

set pagination off
set confirm off

printf "=== debuginfod-go GDB demo ===\n"
shell echo "DEBUGINFOD_URLS=${DEBUGINFOD_URLS}"

printf "\n--- Файл и build-id ---\n"
info files

printf "\n--- Точка останова в greet() ---\n"
break greet
run debuginfod-user

printf "\n--- Backtrace ---\n"
backtrace full

printf "\n--- Локальные переменные greet() ---\n"
info locals

printf "\n--- Символы (через debuginfod, если бинарник stripped) ---\n"
info functions greet

printf "\n=== Demo complete ===\n"
quit
