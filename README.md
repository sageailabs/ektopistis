# ektopistis

The project's goal is to assist with automated node rotation in Kubernetes
clusters that are controlled by
[Cluster Autoscaler](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler).
It automatically drains Kubernetes nodes based on a specified taint.

This program watches for nodes in a Kubernetes cluster and when it detects a
node having been tainted with a specified taint, it cordons off the node and
drains (evicts pods from) it.  It tries to evict all the pods from the
node, except for those controlled by daemon sets.   It is intended to work in
tandem with Cluster Autoscaler, which will be terminating the drained nodes, and
with a process or a job which will select and taint nodes intended for draining.

The project is intended as a simple replacement for
[Draino](https://github.com/planetlabs/draino) which is no longer maintained.

## Usage

Run the binary with providing it with the taint name to use:
```
ektopistis --drain-taint-name=ektopistis.io/drain
```
You can also run the program from a container:
```
docker run sageai/ektopistis --drain-taint-name=ektopistis.io/drain
```
If you are running the program outside a cluster, point it to the cluster to
connect to using the `--kubeconfig` flag or the `KUBECONFIG` environment
variable.

For the program to properly work with Cluster Autoscaler, you need to configure
the latter to ignore taint used for marking the nodes for draining by using the
command line option `--ignore-taint`.  For example:
`--ignore-taint=ektopistis.io/drain`.  You should also configure Cluster
Autoscaler pods to tolerate that taint.

You can mark specific nodes for draining by tainting them directly or you can
write a script to mark them based on specific criteria.  For example, the script
in [examples/aws-ami-upgrade-tainter.sh](examples/aws-ami-upgrade-tainter.sh)
will taint all nodes in an AWS account which have AMI ID that does not match an
AMI ID in their launch template.

## Building

You can build the program with `make build`.  A Docker image can be built
by running `make docker/build`.  Tests are run with `make test` and the code can be
linted with `make lint`.

## Local development

You can build the program locally by running `make build`.
You can run the program locally by against a cluster by providing it with the
`KUBECONFIG` environment variable pointing to the cluster connection
configuration file.  For more extensive diagnostics, provide the
`--zap-log-level=2` command line flag. Here is the full invocation example:
```
$ KUBECONFIG=<path to kube config> go run ./... --zap-log-level=2
```
The running program can be interrupted by `Ctrl+C`.
