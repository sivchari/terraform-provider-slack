default: testacc

.PHONY: testacc
testacc:
	TF_ACC=1 go test -v ./... -race -timeout 1m -shuffle on
