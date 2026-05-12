BINARY   := bootstrap
ZIPFILE  := function.zip
GOARCH   := amd64
GOOS     := linux

.PHONY: build clean zip deploy test

build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o $(BINARY) -tags lambda.norpc .

zip: build
	zip $(ZIPFILE) $(BINARY)

clean:
	rm -f $(BINARY) $(ZIPFILE)

# Requires AWS CLI and a Lambda function already created.
# Set LAMBDA_FUNCTION_NAME env var before calling.
deploy: zip
	aws lambda update-function-code \
		--function-name $(LAMBDA_FUNCTION_NAME) \
		--zip-file fileb://$(ZIPFILE)

# Run locally (requires TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID to be set).
test-local:
	go run . 2>&1 || true

# Unit tests
test:
	go test ./...
