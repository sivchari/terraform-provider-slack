default: testacc

.PHONY: testacc
testacc:
	TF_ACC=1 GOTOOLCHAIN=go1.22.7 go test -v ./... -race -timeout 1m -shuffle on
