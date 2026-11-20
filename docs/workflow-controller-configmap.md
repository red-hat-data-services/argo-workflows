# Workflow Controller Config Map

## Introduction

The Workflow Controller Config Map is used to set controller-wide settings.

For a detailed example, please see [`workflow-controller-configmap.yaml`](./workflow-controller-configmap.yaml).

## Workflow service accounts and Pod restrictions

By default, workflows can use only the namespace's `default` service account.
Set `allowedWorkflowServiceAccounts` to list the accounts workflows can use in
each namespace. A namespace entry replaces the implicit default. An empty
list denies all service accounts in that namespace.

```yaml
data:
  allowedWorkflowServiceAccounts: |
    team-pipelines:
      - default
      - pipeline-runner
```

Anyone who can submit or change executable workflow definitions in a namespace
can use its allowed accounts. Keep the controller ConfigMap writable only by
trusted administrators and grant allowed accounts only the permissions that
all of those workflow authors may use. Keep higher-privilege credentials,
including legacy service account token Secrets, in namespaces where untrusted
authors cannot run workflows.

The controller checks the rendered Pod after applying workflow and template
patches. It rejects host namespaces, hostPath volumes, privileged containers,
and Windows HostProcess for task, agent, and artifact cleanup Pods. Cluster
admission still applies to Pods, including changes made by admission webhooks.

## Alternate Structure

In all versions, the configuration may be under a `config: |` key:

```yaml
# This file describes the config settings available in the workflow controller configmap
apiVersion: v1
kind: ConfigMap
metadata:
  name: workflow-controller-configmap
data:
  config: |
    instanceID: my-ci-controller
    artifactRepository:
      archiveLogs: true
      s3:
        endpoint: s3.amazonaws.com
        bucket: my-bucket
        region: us-west-2
        insecure: false
        accessKeySecret:
          name: my-s3-credentials
          key: accessKey
        secretKeySecret:
          name: my-s3-credentials
          key: secretKey

```

In version 2.7+, the `config: |` key is optional. However, if the `config: |` key is not used, all nested maps under top level
keys should be strings. This makes it easier to generate the map with some configuration management tools like Kustomize.

```yaml
# This file describes the config settings available in the workflow controller configmap
apiVersion: v1
kind: ConfigMap
metadata:
  name: workflow-controller-configmap
data:                      # "config: |" key is optional in 2.7+!
  instanceID: my-ci-controller
  artifactRepository: |    # However, all nested maps must be strings
   archiveLogs: true
   s3:
     endpoint: s3.amazonaws.com
     bucket: my-bucket
     region: us-west-2
     insecure: false
     accessKeySecret:
       name: my-s3-credentials
       key: accessKey
     secretKeySecret:
       name: my-s3-credentials
       key: secretKey
```
