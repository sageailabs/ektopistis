# ektopistis
Drains Kubernetes nodes based on specified criteria

This program watches for nodes in a Kubernetes cluster and drains (evicts pods
from) the ones that a tainted with a specified taint.  It tries to evict all the
pods from the node, except for those controlled by daemon sets.  It is intended
to work in tandem with Cluster Autoscaler, which will be terminating the
drained nodes.  In order for Cluster Autoscaler to do that, it has to be
configured to ignore that taint using the command line option `--ignore-taint`.
For example: `--ignore-taint=ektopistis.io/drain`.  You should also configure
Cluster Autoscaler pods to tolerate that taint.

The project is intended as a simple replacement for
[Draino](https://github.com/planetlabs/draino) which has been abandoned by its
authors.

## Building

You can build the program with `go build ./...`.  A Docker image can be built
by running `make build`.  Tests are run with `make test` and the code can be
linted with `make lint`.

## Local development

You can build the program locally by running `go mod download && go build`.
You can run the program locally by against a cluster by providing it with the
`KUBECONFIG` environment variable pointing to the cluster connection
configuration file.  For more extensive diagnostics, provide it with the
`--zap-log-level=2`. Here is the full invocation example:
```
$ KUBECONFIG=<path to kube config> go run ./... --zap-log-level=2
```
The running program can be interrupted by `Ctrl+C`.
