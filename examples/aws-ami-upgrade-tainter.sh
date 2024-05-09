#!/usr/bin/env bash

function usage {
  echo >&2 "Usage: $0 [-p <aws-profile>] [-r <aws-region>] [-n] [-t <taint-name>]"
  exit 1
}

taint_label=ektopistis.io/draining=ami-upgrade
taint_name=ektopistis.io/drain
aws_opts=('--output=json')
aws_profile=
aws_region=
dry_run_opts=()

while getopts t:p:r:a:n OPTNAME ; do
  case $OPTNAME in
    t) taint_name="$OPTARG" ;;
    p) aws_profile="$OPTARG" ;;
    r) aws_region="$OPTARG" ;;
    n) dry_run_opts+=('--dry-run=server') ;;
    *) usage ;;
  esac
done
shift $((OPTIND - 1))

if test -n "$aws_profile" ; then
  aws_opts+=("--profile=$aws_profile")
fi

if test -n "$aws_region" ; then
  aws_opts+=("--region=$aws_region")
fi

nodes=$(kubectl get nodes -o json)
auto_scaling_groups=$(
  aws "${aws_opts[@]}" autoscaling describe-auto-scaling-groups)

for node_name in $(echo "$nodes" | jq -r '.items[].metadata.name') ; do
  node_json=$(
    echo "$nodes" | jq ".items[] | select(.metadata.name == \"$node_name\")")
  node_provider_id=$(echo "$node_json" | jq -r '.spec.providerID')
  instance_id="${node_provider_id##*/}"
  launch_template_handle=$(
    echo "$auto_scaling_groups" \
      | jq ".AutoScalingGroups[] |
          select(
            .Instances |
            map(select(.InstanceId == \"$instance_id\")) |
            length > 0) |
          .LaunchTemplate")
  if test -z "$launch_template_handle" ; then
    echo >&2 "Node $node_name not found in autoscaling groups"
    continue
  fi
  launch_template_id=$(echo "$launch_template_handle" | jq -r .LaunchTemplateId)
  launch_template_version=$(echo "$launch_template_handle" | jq -r .Version)
  ami_id=$(aws "${aws_opts[@]}" ec2 describe-launch-template-versions \
    --launch-template-id="$launch_template_id" \
    --versions="$launch_template_version" \
    | jq -r '.LaunchTemplateVersions[].LaunchTemplateData.ImageId')
  node_ami_id=$(aws "${aws_opts[@]}" ec2 describe-instances \
    --instance-ids="$instance_id" \
    | jq -r '.Reservations[].Instances[].ImageId')
  if test "$node_ami_id" != "$ami_id" ; then
    kubectl label node \
      "$node_name" \
      "$taint_label" \
      --overwrite \
      "${dry_run_opts[@]}"
  fi
done

if test "${#nodes_to_taint[@]}" -eq 0 ; then
  echo >&2 "Skipping node tainting in dry-run mode."
  exit
fi

nodes_to_taint=$(kubectl get nodes -l "$taint_label" 2>/dev/null)

if test "${#nodes_to_taint[@]}" -eq 0 ; then
  echo >&2 "No nodes found to taint; exiting."
  exit
fi

kubectl taint nodes \
  -l "$taint_label" \
  "$taint_name=true:NoSchedule" \
  --overwrite \
  "${dry_run_opts[@]}"
