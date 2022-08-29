KUBERNETES_VERSION = 1.24.0

KIND_CONFIG := kind-config.yaml

.PHONY: test
test:
	env PATH=$$(pwd)/../bin:$$PATH RUN_E2E=1 \
		go test -v -race . -ginkgo.progress -ginkgo.v -ginkgo.fail-fast

.PHONY: start
start:
	kind create cluster --name=necotiator --config=$(KIND_CONFIG) --image=kindest/node:v$(KUBERNETES_VERSION) --wait 1m
	cd ..; docker build --no-cache -t necotiator:dev .
	kind load docker-image necotiator:dev --name=necotiator
	kubectl apply -f https://github.com/jetstack/cert-manager/releases/latest/download/cert-manager.yaml
	kubectl -n cert-manager wait --for=condition=available --timeout=180s --all deployments
	kustomize build ../config/default | kubectl apply -f -
	kubectl -n necotiator-system wait --for=condition=available --timeout=180s --all deployments

.PHONY: stop
stop:
	kind delete cluster --name=necotiator
	-docker image rm necotiator:dev
	-docker image prune -f
