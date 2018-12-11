.PHONY: test help default

GIT_COMMIT=$(shell git rev-parse HEAD)
GIT_DIRTY=$(shell test -n "`git status --porcelain`" && echo "+CHANGES" || true)

default: test

help:
	@echo 'Management commands for go-exim:'
	@echo
	@echo 'Usage:'
	@echo '    make get-deps        runs dep ensure, mostly used for ci.'
	
	@echo '    make clean           Clean the directory tree.'
	@echo

get-deps:
	dep ensure

test:
	go test -coverprofile cp.out ./...

test-coverage:
	go tool cover -html=cp.out

