BUILD_DIR=$(pwd)/build
GOOS=linux GOARCH=amd64 cd api && go build -o $BUILD_DIR/api && cd ..
GOOS=linux GOARCH=amd64 cd producer && go build -o $BUILD_DIR/producer && cd ..
GOOS=linux GOARCH=amd64 cd consumer && go build -o $BUILD_DIR/consumer && cd ..