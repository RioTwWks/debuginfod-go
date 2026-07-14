.PHONY: sample delve-sample up down gdb delve demo demo-delve clean health metadata

sample:
	$(MAKE) -C sample

delve-sample:
	$(MAKE) -C delve/sample

up: sample
	docker compose up -d debuginfod

down:
	docker compose down -v

gdb: sample
	docker compose --profile gdb run --rm gdb

delve: delve-sample
	docker compose --profile delve run --rm delve

demo: sample
	docker compose --profile gdb up --build --abort-on-container-exit gdb
	docker compose down

demo-delve: delve-sample
	docker compose --profile delve up --build --abort-on-container-exit delve
	docker compose down

health:
	curl -sf http://localhost:8002/healthz && echo

metadata: sample
	curl -s "http://localhost:8002/metadata?key=file&value=/sample/hello" | head -c 500 && echo

clean:
	$(MAKE) -C sample clean
	$(MAKE) -C delve/sample clean
	docker compose down -v 2>/dev/null || true
