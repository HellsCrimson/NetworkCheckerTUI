.PHONY: build cross-build

BIN_DIR=bin

build: bin_dir
	go build -o ${BIN_DIR}/network-check

cross-build: bin_dir
	GOOS=linux GOARCH=amd64 go build -o ${BIN_DIR}/network-check-linux-amd64
	GOOS=windows GOARCH=amd64 go build -o ${BIN_DIR}/network-check-windows-amd64.exe
    # GOOS=darwin GOARCH=amd64 go build -o ${BIN_DIR}/network-check-darwin-amd64
    # GOOS=darwin GOARCH=arm64 go build -o ${BIN_DIR}/network-check-darwin-arm64

bin_dir:
	mkdir -p ${BIN_DIR}
