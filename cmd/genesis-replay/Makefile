BINARY      := genesis-replay
EXT_DATA_DIR ?= .docker-data

.PHONY: build keygen up down test-integration run verify clean

## build: compile the genesis-replay binary
build:
	go build -o $(BINARY) .

## keygen: generate a new genesis migration keypair
keygen: build
	./$(BINARY) keygen

## up: start the docker-compose test stack (detached)
up:
	docker compose up -d --wait

## down: tear down the stack and remove Docker-managed named volumes
down:
	docker compose down -v

## test-integration: run the full end-to-end integration test
## Prerequisites: stack must be running (make up)
test-integration:
	go test -v -tags integration -run TestGenesisReplay -timeout 20m ./...

## run: replay entities from a source DB to a bootstrap chain
## Required env vars: GENESIS_SRC_DSN, GENESIS_CHAIN_URL, GENESIS_MIGRATION_PRIVATE_KEY
run: build
	./$(BINARY) run

## verify: diff entities and plays between two discovery-provider databases
## Required env vars: GENESIS_SRC_DSN, GENESIS_DEST_DSN
verify: build
	./$(BINARY) verify

## clean: remove the compiled binary and data directories under EXT_DATA_DIR (default: .docker-data)
clean:
	rm -f $(BINARY)
	rm -rf "$(EXT_DATA_DIR)/test-validator" "$(EXT_DATA_DIR)/test-pg-data"
