TARGET=bean
SOURCES=$(shell find cmd/bean types -type f -name "*.go")

$(TARGET): $(SOURCES)
	GOOS=linux go build -o $@ ./cmd/bean

.PHONY: release
release: amd64 arm64

.PHONY: amd64
amd64:
	GOOS=linux GOARCH=amd64 go build -ldflags '-extldflags -static -s -w' -o $(TARGET).amd64 ./cmd/bean

.PHONY: arm64
arm64:
	GOOS=linux GOARCH=arm64 go build -ldflags '-extldflags -static -s -w' -o $(TARGET).arm64 ./cmd/bean

.PHONY: clean
clean:
	@rm -f $(TARGET) $(TARGET).amd64 $(TARGET).arm64
