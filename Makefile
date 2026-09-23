.PHONY: redis gateway worker run

redis:
	docker compose up -d

gateway:
	cd gateway && go run .

worker:
	cd worker && cargo run --release

run: redis
	@echo "Start worker and gateway in separate terminals:"
	@echo "  make worker"
	@echo "  make gateway"
