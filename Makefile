live/templ:
	templ generate -path ./view --watch --proxy="http://localhost:8080" --open-browser=false -v

live/server:
	go run github.com/cosmtrek/air@v1.51.0 \
	--build.cmd "go build -o tmp/bin/main ./cmd/app/main.go" --build.bin "tmp/bin/main" --build.delay "100" \
	--build.exclude_dir "node_modules" \
	--build.include_ext "go" \
	--build.stop_on_error "false" \
	--misc.clean_on_exit true

live/tailwind:
	bunx tailwindcss -i ./view/input.css -o ./view/assets/styles.css --minify --watch

live:
	make -j3 live/tailwind live/templ live/server

test:
	@bash ./scripts/run_tests.sh $(if $(ARGS),args="$(ARGS)") $(if $(TEST_DIR),test-dir="$(TEST_DIR)")

generate-sql:
	@bash ./scripts/sql_gen.sh

local/server:
	go run ./cmd/sync_service/main.go

local/db:
	docker compose up

local:
	make -j2 local/server local/db

migrate:
	go run ./cmd/migrate/main.go $(DIR)
