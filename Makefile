.PHONY: test.unit application.start application.build

test.unit:
	@echo "Running native unit tests: go test ./..."
	go test ./...

application.start: test.unit
	@echo "Running application: go run ./cmd/file-folder-renamer"
	go run ./cmd/file-folder-renamer

application.build:
	@echo "Building Linux: ./build/build.sh linux --arch amd64"
	@./build/build.sh linux --arch amd64
	@echo "Building Windows: ./build/build.sh windows --arch amd64"
	@./build/build.sh windows --arch amd64
	@echo "Building macOS: ./build/build.sh mac --arch amd64 --arch arm64"
	@./build/build.sh mac --arch amd64 --arch arm64
