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

### Limitations

Currently, ektopistis has been tested on clusters with up to a few dozen nodes.
Its performance may suffer on larger clusters and more work may be needed to
accommodate them.

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

## Installation

You can install ektopistis using Helm:

```
helm repo add ektopistis https://sageailabs.github.io/ektopistis
helm repo update ektopistis
helm update -i ektopistis/ektopistis -n <target-namespace>
```

### Parameters

| Parameter | Description | Default value |
|-----------|-------------|---------------|
| `taintName` | Name of the taint to use for marking nodes for draining   | `ektopistis.io/drain` |
| `extraArgs` | List of arguments to pass to the program | `[]` |
| `image.repository` | Docker repository for images | `sageai/ektopistis` |
| `image.tag` | Image tag to use | Value of `.Chart.AppVersion` |
| `imagePullSecrets` | List of references to secrets to use to pull the container image | `[]` |
| `podAnnotations` | Annotations to set on the pods | `{}` |
| `resources` | Resources to request for the pods | `{"requests":{"cpu":"40m","memory":"80Mi"}, "limits":{"cpu":"50m","memory":"80Mi"}}` |

### Uninstalling

```
helm uninstall ektopistis -n <target-namespace>
```

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

## Contributing

### Bug Reports and Feature Requests

Please use the [issue tracker](https://github.com/sageailabs/ektopistis/issues)
to report any bugs or file feature requests.

## Submitting a Pull Request on GitHub

This project uses GitHub's pull request process. Follow these steps to submit a
pull request:

1. **Fork the Repository**: Create a copy of the main repository in your GitHub
   account.
1. **Make Changes**: Push any changes to a new branch on your forked repository.
1. **Create a Pull Request**: Open a pull request from your branch to the main
   repository.

### Requirements for Merging a Pull Request

Before your pull request can be merged, the following steps must be completed:

1. **Document the Issue**: Ideally, there should be an issue that thoroughly
   documents the problem or feature.
1. **Test Coverage**: Ensure that any new code has a reasonable amount of test
   coverage.
1. **Pass Tests**: All tests must pass.
1. **Review and Approval**: The pull request needs to be reviewed and approved.

Once these steps are completed, a code owner will merge the pull request.
