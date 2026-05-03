.PHONY: run test

PORT ?= 8080
ARCH ?=

run:
	./scripts/run_app.sh --port $(PORT) $(if $(ARCH),--arch $(ARCH),)

test:
	./scripts/run_tests.sh $(if $(ARCH),--arch $(ARCH),)
