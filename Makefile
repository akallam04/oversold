VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: tools build test vet images up down ps logs psql clean env-check

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

images: env-check
	@for v in $$(ls targets/medusa/versions); do \
		echo "building oversold-medusa:$$v"; \
		MEDUSA_VERSION=$$v docker compose build medusa-migrate || exit 1; \
	done

up: env-check
	docker compose up -d --build --wait medusa

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
