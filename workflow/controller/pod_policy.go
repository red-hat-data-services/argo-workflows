package controller

import (
	"os"

	apiv1 "k8s.io/api/core/v1"

	"github.com/argoproj/argo-workflows/v3/errors"
)

// The managed deployment enables this after installing admission for Workflow writes.
const workflowAccountAuthorizationEnv = "ARGO_REQUIRE_WORKFLOW_SERVICE_ACCOUNT_AUTHORIZATION"

func (woc *wfOperationCtx) checkWorkflowServiceAccount(name string) error {
	if os.Getenv(workflowAccountAuthorizationEnv) != "true" || name == "" || name == "default" {
		return nil
	}
	// Admission authorizes the literal account on the original Workflow. Templates,
	// defaults, substitutions, and Pod patches cannot delegate a different account.
	if woc.orig.Spec.ServiceAccountName == name {
		return nil
	}
	return errors.Errorf(errors.CodeForbidden, "service account %q must be authorized on the Workflow", name)
}

func (woc *wfOperationCtx) requiresLocalTemplates() bool {
	account := woc.orig.Spec.ServiceAccountName
	return os.Getenv(workflowAccountAuthorizationEnv) == "true" && account != "" && account != "default"
}

// Check the rendered spec after merging patches and assembling containers and volumes.
func (woc *wfOperationCtx) validateWorkflowPodSpec(spec *apiv1.PodSpec) error {
	if os.Getenv(workflowAccountAuthorizationEnv) != "true" {
		return nil
	}
	account := spec.ServiceAccountName
	if spec.DeprecatedServiceAccount != "" {
		if account != "" && account != spec.DeprecatedServiceAccount {
			return errors.New(errors.CodeForbidden, "workflow pod has conflicting service account fields")
		}
		account = spec.DeprecatedServiceAccount
	}
	if err := woc.checkWorkflowServiceAccount(account); err != nil {
		return err
	}

	if spec.HostNetwork || spec.HostPID || spec.HostIPC {
		return errors.New(errors.CodeForbidden, "workflow pods may not share host network, PID, or IPC namespaces")
	}
	if sc := spec.SecurityContext; sc != nil && sc.WindowsOptions != nil &&
		sc.WindowsOptions.HostProcess != nil && *sc.WindowsOptions.HostProcess {
		return errors.New(errors.CodeForbidden, "workflow pods may not use Windows HostProcess")
	}
	for _, volume := range spec.Volumes {
		if volume.HostPath != nil {
			return errors.Errorf(errors.CodeForbidden, "workflow pod volume %q may not use hostPath", volume.Name)
		}
	}
	for _, containers := range [][]apiv1.Container{spec.Containers, spec.InitContainers} {
		for _, container := range containers {
			if err := validateWorkflowContainerSecurity(container.Name, container.SecurityContext); err != nil {
				return err
			}
		}
	}
	for _, container := range spec.EphemeralContainers {
		if err := validateWorkflowContainerSecurity(container.Name, container.SecurityContext); err != nil {
			return err
		}
	}
	return nil
}

func validateWorkflowContainerSecurity(name string, sc *apiv1.SecurityContext) error {
	if sc == nil {
		return nil
	}
	if sc.Privileged != nil && *sc.Privileged {
		return errors.Errorf(errors.CodeForbidden, "workflow pod container %q may not run privileged", name)
	}
	if sc.WindowsOptions != nil && sc.WindowsOptions.HostProcess != nil && *sc.WindowsOptions.HostProcess {
		return errors.Errorf(errors.CodeForbidden, "workflow pod container %q may not use Windows HostProcess", name)
	}
	return nil
}
