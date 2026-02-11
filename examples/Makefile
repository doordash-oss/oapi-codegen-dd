lint:
	$(GOBIN)/golangci-lint run ./...

lint-ci:
	$(GOBIN)/golangci-lint run ./... --timeout=5m

generate:
	go generate ./...

test:
	go test -cover ./...

tidy:
	go mod tidy

tidy-ci:
	tidied -verbose

gosec:
	# Exclude rules that are false positives for generated API code:
	# G705: XSS via taint - false positive for server-side API responses (not HTML)
	# G706: Log injection - acceptable in example scaffold code
	# G117: Secret patterns - expected field names in API examples (Password, ClientSecret, etc.)
	gosec -exclude-dir=api -exclude=G705,G706,G117 ./...

check-fmt:
	# check if files are formatted
	@UNFORMATTED=$$(gofmt -l .); \
	if [ -n "$$UNFORMATTED" ]; then \
		echo >&2 "ERROR: The following files are not formatted:"; \
		echo "$$UNFORMATTED"; \
		exit 1; \
	fi
