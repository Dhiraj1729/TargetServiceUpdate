SHELL := /bin/bash
.DEFAULT_GOAL := help
.PHONY: help tools doctor deps setup cluster-up cluster-verify cluster-down shell-check
help:
	@echo 'App: test build image deploy (alias: hsm-deploy) hsm-verify demo smoke port-forward'
	@echo 'make setup          Install missing tools, download Go dependencies, start infrastructure'
	@echo 'make doctor         Inspect host tools without starting a VM'
	@echo 'make cluster-up     Start/reuse only the HSM environment'
	@echo 'make cluster-verify Verify nodes, DNS and storage provisioner only'
	@echo 'make cluster-down   Stop HSM and preserve its data'
	@echo 'make shell-check    Check setup script syntax'
tools:
	bash scripts/install-tools.sh
doctor:
	bash scripts/doctor.sh
deps:
	go mod download
setup:
	$(MAKE) tools
	$(MAKE) deps
	$(MAKE) cluster-up
cluster-up cluster-verify cluster-down:
	bash scripts/$@.sh
shell-check:
	@for script in scripts/*.sh scripts/lib/*.sh; do bash -n "$$script" || exit; done

.PHONY: test build image deploy hsm-deploy hsm-verify demo smoke port-forward
test: shell-check
	CGO_ENABLED=0 go test ./...
	CGO_ENABLED=0 go vet ./...
build:
	mkdir -p bin
	CGO_ENABLED=0 go build -o bin/manager ./cmd/manager
image:
	bash scripts/images.sh
deploy hsm-deploy:
	bash scripts/hsm-deploy.sh
hsm-verify:
	bash scripts/hsm-verify.sh
demo:
	bash scripts/demo.sh
smoke:
	bash scripts/smoke.sh
port-forward:
	KUBECONFIG="$(CURDIR)/work/setup/kubeconfig" kubectl --context=hsm -n hsm-system port-forward --address=127.0.0.1 service/service-target-manager 8080:8080
