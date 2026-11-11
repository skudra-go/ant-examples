# antfmt lives in the ant checkout next to this repository.
EXAMPLES := examples/hello examples/blog examples/audit examples/ledger

.PHONY: fmt lint

# fmt lays the code out, in two rounds because one tool's line breaks can give the
# other something to do. lint fails on anything fmt would change.
fmt:
	@for round in 1 2; do \
		go run -C ../ant/tools/antfmt . $(CURDIR) || exit 1; \
		for example in $(EXAMPLES); do (cd $$example && golangci-lint run --fix ./...) || exit 1; done; \
	done

lint:
	go run -C ../ant/tools/antfmt . -l $(CURDIR)
	@for example in $(EXAMPLES); do (cd $$example && go vet ./... && golangci-lint run ./...) || exit 1; done
