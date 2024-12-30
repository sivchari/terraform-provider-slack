default: testacc

.PHONY: testacc
testacc:
	TF_ACC=1 go test -v ./... -race -timeout 30m -shuffle on
