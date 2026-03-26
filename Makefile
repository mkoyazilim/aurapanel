.PHONY: all core api frontend build clean install

# Variables
CORE_DIR = core
API_DIR = api-gateway
FRONTEND_DIR = frontend
BUILD_DIR = build

all: build

# Build Rust Micro-Core
core:
	@echo "Building Rust Micro-Core..."
	cd $(CORE_DIR) && cargo build --release
	mkdir -p $(BUILD_DIR)/core
	cp $(CORE_DIR)/target/release/aurapanel-core $(BUILD_DIR)/core/

# Build Go API Gateway
api:
	@echo "Building Go API Gateway..."
	cd $(API_DIR) && go build -o apigw main.go
	mkdir -p $(BUILD_DIR)/api
	cp $(API_DIR)/apigw $(BUILD_DIR)/api/

# Build Vue.js Frontend
frontend:
	@echo "Building Vue.js Frontend..."
	cd $(FRONTEND_DIR) && npm install && npm run build
	mkdir -p $(BUILD_DIR)/frontend
	cp -r $(FRONTEND_DIR)/dist/* $(BUILD_DIR)/frontend/

# Build Everything
build: core api frontend
	@echo "All components built successfully in $(BUILD_DIR)/ directory."

# Clean build artifacts
clean:
	@echo "Cleaning artifacts..."
	cd $(CORE_DIR) && cargo clean
	cd $(API_DIR) && rm -f apigw
	cd $(FRONTEND_DIR) && rm -rf dist node_modules
	rm -rf $(BUILD_DIR)

# Package for Distribution
package: build
	@echo "Creating deployment tarball..."
	tar -czvf aurapanel-release.tar.gz -C $(BUILD_DIR) .
	@echo "aurapanel-release.tar.gz created."
