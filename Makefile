PACKAGES = $(shell go list ./...)

test-all: vet lint test

test:
	go test -v -parallel=8 ${PACKAGES}

test-race:
	go test -v -race ${PACKAGES}

vet:
	go vet ${PACKAGES}

lint:
	@go get golang.org/x/lint/golint
	go list ./... | grep -v vendor | xargs -n1 golint 

cover:
	@go get golang.org/x/tools/cmd/cover		
	go test -coverprofile=cover.out
	go tool cover -html cover.out
	sleep 1
	rm cover.out

.PHONY: test test-race vet lint cover	
