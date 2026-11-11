VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: tools build test vet images up down ps logs psql clean env-check repro

tools:
	@sh scripts/check-tools.sh

build:
	go build -ldflags "$(LDFLAGS)" -o bin/oversold ./cmd/oversold

test:
	go test -race ./...

vet:
	go vet ./...
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "these files need gofmt:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

env-check:
	@if [ ! -f .env ]; then \
		echo ".env is missing. Run: cp .env.example .env" >&2; \
		exit 1; \
	fi

images: env-check build
	@for v in $$(./bin/oversold versions); do \
		inputs=$$(./bin/oversold image-inputs --version $$v) || exit 1; \
		echo "building medusa $$v, image inputs $$inputs"; \
		MEDUSA_VERSION=$$v MEDUSA_IMAGE_TAG=$$v OVERSOLD_BUILD_TARGET=release OVERSOLD_VARIANT= OVERSOLD_IMAGE_INPUTS=$$inputs \
			docker compose build medusa-migrate || exit 1; \
	done

up: env-check build
	@inputs=$$(./bin/oversold image-inputs) || exit 1; \
	echo "docker compose up -d --build --wait medusa, image inputs $$inputs"; \
	MEDUSA_IMAGE_TAG= OVERSOLD_BUILD_TARGET=release OVERSOLD_VARIANT= OVERSOLD_IMAGE_INPUTS=$$inputs \
		docker compose up -d --build --wait medusa

repro: env-check build
	docker compose up -d --wait postgres
	./bin/oversold expect expectations/medusa.yaml $(if $(MEDUSA),--version $(MEDUSA))

down:
	docker compose down -v

ps:
	docker compose ps

logs:
	docker compose logs -f

psql: env-check
	docker compose exec postgres sh -c 'psql -U "$$POSTGRES_USER" -d "$(or $(DB),$$POSTGRES_DB)"'

clean:
	rm -rf bin
