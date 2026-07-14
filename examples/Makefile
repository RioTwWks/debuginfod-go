.PHONY: sample up down gdb demo clean health metadata

sample:
	$(MAKE) -C sample

up: sample
	docker compose up -d debuginfod

down:
	docker compose down -v

gdb: sample
	docker compose --profile gdb run --rm gdb

demo: sample
	docker compose --profile gdb up --build --abort-on-container-exit gdb
	docker compose down

health:
	curl -sf http://localhost:8002/healthz && echo

metadata: sample
	curl -s "http://localhost:8002/metadata?key=file&value=/sample/hello" | head -c 500 && echo

clean:
	$(MAKE) -C sample clean
	docker compose down -v 2>/dev/null || true
