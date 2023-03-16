lint:
	golangci-lint run --max-issues-per-linter=0 --max-same-issues=0 --sort-results

lint-fix:
	golangci-lint run --fix

tidy:
	GONOSUMDB=github.com/codecomet-io GOPRIVATE=github.com/codecomet-io go mod tidy
