BINARY_NAME = distcache
BIN_DIR = bin

build:
	@echo "Building server..."
	go build -o $(BIN_DIR)/$(BINARY_NAME) main.go

run: build
	@echo "Starting Leader on :3000"
	@./$(BIN_DIR)/$(BINARY_NAME) --listenaddr=:3000 --storage=cache.db

runfollower: build
	@echo "Starting Follower on :4000 (Leader at :3000)"
	@./$(BIN_DIR)/$(BINARY_NAME) --listenaddr=:4000 --leaderaddr=:3000 --storage=cache_follower.db

build-client:
	@echo "Building client..."
	go build -o $(BIN_DIR)/client ./client/main.go

runclient: build-client
	@echo "Running client (connecting to localhost:3000)"
	@./$(BIN_DIR)/client localhost 3000

clean:
	@echo "Cleaning..."
	@if exist $(BIN_DIR) rmdir /S /Q $(BIN_DIR)
	@del /Q /F cache*.db 2>nul
	
test:
	@echo "Running tests..."
	go test ./...

fmt:
	@echo "Formatting code..."
	go fmt ./...