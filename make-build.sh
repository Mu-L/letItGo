BUILD_DIR=$(pwd)/build

# Build for Linux (x86_64) architecture
GOOS=linux GOARCH=amd64 go build -o $BUILD_DIR/api ./api
GOOS=linux GOARCH=amd64 go build -o $BUILD_DIR/producer ./producer
GOOS=linux GOARCH=amd64 go build -o $BUILD_DIR/consumer ./consumer
