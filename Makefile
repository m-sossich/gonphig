.PHONY: test coverage

test:
	go test -v ./... -count=1

coverage:
	go test -v ./... -count=1 -coverprofile cover.out \
	    && go tool cover -html=cover.out
