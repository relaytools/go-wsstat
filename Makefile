PACKAGES = $(shell go list ./...)

test-all: vet lint test

test:
	go test -v -parallel=8 ${PACKAGES}

test-race:
	go test -v -race ${PACKAGES}

vet:
	go vet ${PACKAGES}

lint:
	go install golang.org/x/lint/golint@latest
	$(shell go list ./... | grep -v vendor | xargs -n1 golint )

cover:
	go test -coverprofile=cover.out
	go tool cover -html cover.out -o coverage.html
	@which xdg-open &> /dev/null && xdg-open coverage.html || open coverage.html || echo "Open coverage.html manually"
	@sleep 1
	@rm -f cover.out coverage.html

.PHONY: test-all test test-race vet lint cover
