# Skueue

A proof-of-concept of a Kubernetes-based backend infrastructure implementing a job-queue design for the compilation of Arduino sketches.

# Local setup

Relying on Kubernetes the deployment could occur on any Kubernetes cluster in principle.

This hasn't been tested anywhere but on a locally provisioned cluster with Docker Desktop (on MacOS Apple Silicon). Adaptations are likely to be needed in order to work with cloud provided clusters while the backbone should remain consistent.

## Prerequisites

- Install and run [Docker Desktop](https://docs.docker.com/desktop/)
- Enable [Kubernetes in Docker Desktop](https://docs.docker.com/desktop/kubernetes/).
- [Conditional] If you already worked with `kubectl` please ensure to use the `docker-desktop` configuration context in order to operate in the right cluster

## Run

```
make prepare-and-launch
```

This will build the docker images that are used in the Kubernetes manifests and deploy all necessary resources.

If you look into the `Makefile` you can spot how `docker` and `kubectl` commands are executed.

It can also be seen that Kubernetes [Kustomizations](https://kubernetes.io/docs/tasks/manage-kubernetes-objects/kustomization/) are adopted to describe every aspect. This isn't really necessary at this stage as there is only one single version of the manifests but it could help going forward for managing multiple environments and alternatives.

> Makefile is your friend - Unknown

## Usage

See [./examples/compilation_start_and_follow.go] for a showcase of the client package functionalities.

You can also inpect the schema and play around with the api using the [GraphiQL UI](http://localhost:3001).
_Hint!_ Grab the content of [./api/api-definition/query.graphql](./api/api-definition/query.graphql) and paste it in the web interface for a good starting point

## Development

The repository is a Golang [multi-module workspace](https://go.dev/doc/tutorial/workspaces) organized as follow:

- `service` is a Golang package implementing an http service that wraps:
  - a GraphQL endpoint `/query` that handles compilation
    starts, logs and updates subscriptions (through websocket).
  - a file endpoint `/artefacts` used to retrieve compilation build files as tarballs
- `client` is a mostly-autogenerated Golang package implementing a client for the `service` endpoints
  - the only manual customization wraps the http request to get compilation tarballs

The schema of the GraphqlAPI is visible at [./api/api-definition/schema/schema.graphql](api/api-definition/schema/schema.graphql)

The `/artefacts` endpoint has no declarative definition. Its serves requests with pattern `/artefacts?compilationId=<compilationId>`.

For development purposes the service can be run outside of Kubernetes with

```
make run-service-dev
```

In this scenario it uses the local `kubectl` configuration in order to connect to a cluster, normally located at `~/.kube/config`, 
instead of a service account.
