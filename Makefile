.PHONY: all service api frontend build clean package

SERVICE_DIR = panel-service
API_DIR = api-gateway
FRONTEND_DIR = frontend
BUILD_DIR = build

all: build

# Build Go Panel Service
service:
	@echo "Building Go Panel Service..."
	cd $(SERVICE_DIR) && go build -o panel-service .
	mkdir -p $(BUILD_DIR)/service
	cp $(SERVICE_DIR)/panel-service $(BUILD_DIR)/service/

# Build Go API Gateway
api:
	@echo "Building Go API Gateway..."
	cd $(API_DIR) && go build -o apigw .
	mkdir -p $(BUILD_DIR)/api
	cp $(API_DIR)/apigw $(BUILD_DIR)/api/

# Build Vue.js Frontend
frontend:
	@echo "Building Vue.js Frontend..."
	cd $(FRONTEND_DIR) && npm install && npm run build
	mkdir -p $(BUILD_DIR)/frontend
	cp -r $(FRONTEND_DIR)/dist/* $(BUILD_DIR)/frontend/

# Build Everything
build: service api frontend
	@echo "All components built successfully in $(BUILD_DIR)/ directory."

# Clean build artifacts
clean:
	@echo "Cleaning artifacts..."
	cd $(SERVICE_DIR) && rm -f panel-service
	cd $(API_DIR) && rm -f apigw
	cd $(FRONTEND_DIR) && rm -rf dist node_modules
	rm -rf $(BUILD_DIR)

# Package for Distribution
package: build
	@echo "Creating deployment tarball..."
	tar -czvf aurapanel-release.tar.gz -C $(BUILD_DIR) .
	@echo "aurapanel-release.tar.gz created."
