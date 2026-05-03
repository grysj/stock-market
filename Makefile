.PHONY: run test

PORT     ?= 8080
PLATFORM ?= linux

run:
	./scripts/run_app.sh --port $(PORT) --platform $(PLATFORM)

test:
	./scripts/run_tests.sh
