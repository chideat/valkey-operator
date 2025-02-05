# How to Contribute

We'd love to accept your patches and contributions to this project. There are
just a few small guidelines you need to follow.

## GitHub issues

Valkey Operator uses GitHub issues for feature development and bug tracking.
The issues have specific information as to what the feature should do and what problem or
use case is trying to resolve. Bug reports have a description of the actual behaviour and
the expected behaviour, along with repro steps when possible. It is important to provide
repro when possible, as it speeds up the triage and potential fix.

For support questions, we strongly encourage you to provide a way to
reproduce the behavior you're observing, or at least sharing as much
relevant information as possible on the Issue. This would include YAML manifests, Kubernetes version,
Valkey Operator debug logs and any other relevant information that might help
to diagnose the problem.

## Makefile

This project contains a Makefile to perform common development operation. If you want to build, test or deploy a local copy of the repository, keep reading.

#### Make targets

- **controller-gen** Install controller-gen if not in local bin path
- **kustomize** Install kustomize if not in local bin path
- **envtest** Install setup-envtest tools if not in local bin path
- **operator-sdk** Install operator-sdk CLI if not in local bin path
- **manifests** Generate manifests e.g. CRD, RBAC etc.
- **generate** Generate code
- **fmt** Run go fmt against code
- **vet** Run go vet against code
- **lint** Run golangci-lint linter
- **test** Run unit tests
- **test-e2e** Run the e2e tests against a Kind k8s instance that is spun up.
- **integration-tests** Run integration tests (TODO)
- **build** Build operator binary
- **run** Run operator binary locally against the configured Kubernetes cluster in ~/.kube/config
- **docker-build** Build the docker image
- **docker-push** Push the docker image
- **docker-buildx** Build the docker image using buildx
- **build-install** Generate a consolidated YAML with CRDs and deployment
- **install** Install CRDs into a cluster
- **uninstall** Uninstall CRDs from a cluster
- **deploy** Deploy operator in the configured Kubernetes cluster in ~/.kube/config
- **undeploy** Undeploy operator from the configured Kubernetes cluster in ~/.kube/config
- **bundle** Generate bundle manifests and metadata, then validate generated manifests
- **bundle-build** Build the bundle image
- **bundle-push** Push the bundle image

### Testing

Before submitting a pull request, ensure necessary unit tests added and all local tests pass:
- `make test`

Also, run the system tests with your local changes against a Kubernetes cluster:
- `make deploy`

## Pull Requests

Valkey Operator project uses pull requests to discuss, collaborate on and accept code contributions.
Pull requests are the primary place of discussing code changes.

Here's the recommended workflow:

 * [Fork the repository][github-fork] or repositories you plan on contributing to. If multiple
   repositories are involved in addressing the same issue, please use the same branch name
   in each repository
 * Create a branch with a descriptive name
 * Make your changes, run tests (usually with `make test`), commit with a
   [descriptive message][git-commit-msgs], push to your fork
 * Submit pull requests with an explanation what has been changed and **why**
 * We will get to your pull request within one week. Usually within the next day or two you'll get a response.

### Code Conventions

This project follows the [Kubernetes Code Conventions for Go](https://github.com/kubernetes/community/blob/master/contributors/guide/coding-conventions.md#code-conventions), which in turn mostly refer to [Effective Go](https://golang.org/doc/effective_go.html) and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments). Please ensure your pull requests follow these guidelines.

## Code reviews

All submissions, including submissions by project members, require review. We
use GitHub pull requests for this purpose. Consult
[GitHub Help](https://help.github.com/articles/about-pull-requests/) for more
information on using pull requests.
