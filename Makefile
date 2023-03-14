.PHONY: service-generate
service-generate:
	cd service && go run github.com/99designs/gqlgen generate

.PHONY: client-generate
client-generate:
	cd client && go run github.com/beyondan/gqlgenc

.PHONY: cli-image-build
cli-image-build:
	docker build -f ./docker/arduino-cli.Dockerfile . -t arduino/cli

.PHONY: service-image-build
service-image-build:
	docker build -f ./docker/service.Dockerfile . -t arduino/job-service

.PHONY: launch
launch:
	kubectl kustomize k8s/jobs-namespace | kubectl apply -f -
	kubectl kustomize k8s/dashboard | kubectl apply -f -

.PHONY: prepare-and-launch
prepare-and-launch: cli-image-build service-image-build launch

.PHONY: run-service-dev
run-service-dev: cd ./service && go run .
