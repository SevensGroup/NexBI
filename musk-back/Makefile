APP_NAME := autonomous-agent
CONFIG ?= configs/agent.yaml

.PHONY: tidy test vet build run mockmcp docker-build docker-up docker-down selftest test-live

SELFTEST_TIMEOUT ?= 12m
MUSK_BACK_URL ?= http://127.0.0.1:8080

tidy:
	go mod tidy

test:
	go test ./...

vet:
	go vet ./...

build:
	mkdir -p bin
	go build -o bin/$(APP_NAME) ./cmd/server
	go build -o bin/mockmcp ./cmd/mockmcp

run:
	@set -a; \
	if [ -f .env ]; then . ./.env; fi; \
	set +a; \
	go run ./cmd/server -config $(CONFIG)

mockmcp:
	go run ./cmd/mockmcp

docker-build:
	docker build -t $(APP_NAME):latest --target agent .

docker-up:
	docker compose up --build

docker-down:
	docker compose down

# Live SiliconFlow self-test. Loads musk-back/.env at runtime (never commit it).
# Spends real LLM tokens. Requires: make run (this Makefile sources .env).
selftest test-live:
	@set -a; \
	if [ -f .env ]; then . ./.env; fi; \
	set +a; \
	if [ -z "$$LLM_API_KEY" ] || [ "$$LLM_API_KEY" = "sk-your-key" ]; then \
		echo "selftest: musk-back/.env must set LLM_API_KEY (do not commit .env)"; \
		exit 1; \
	fi; \
	if [ "$$LLM_BASE_URL" != "https://api.siliconflow.cn/v1" ]; then \
		echo "selftest: LLM_BASE_URL must be https://api.siliconflow.cn/v1"; \
		exit 1; \
	fi; \
	if [ "$$LLM_MODEL" != "deepseek-ai/DeepSeek-V4-Pro" ]; then \
		echo "selftest: LLM_MODEL must be deepseek-ai/DeepSeek-V4-Pro"; \
		exit 1; \
	fi; \
	if ! curl -sf "$(MUSK_BACK_URL)/health" >/dev/null; then \
		echo "selftest: musk-back is down. start musk-back with musk-back/.env"; \
		echo "  cd musk-back && make run"; \
		exit 1; \
	fi; \
	echo "selftest: live LLM path (SiliconFlow DeepSeek-V4-Pro) — this spends real tokens"; \
	MUSK_SELFTEST_ARTIFACT="$(CURDIR)/../musk-web/e2e/.last-live-run.json" \
	go test -tags live -count=1 -timeout $(SELFTEST_TIMEOUT) ./internal/selftest/
