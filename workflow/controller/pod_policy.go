package controller

import (
	"slices"

	apiv1 "k8s.io/api/core/v1"

	"github.com/argoproj/argo-workflows/v3/errors"
)

func (woc *wfOperationCtx) checkWorkflowServiceAccount(name string) error {
	if name == "" {
		name = "default"
	}
	namespace := woc.wf.Namespace
	allowed, configured := woc.controller.Config.AllowedWorkflowServiceAccounts[namespace]
	if slices.Contains(allowed, name) || (!configured && name == "default") {
		return nil
	}
	return errors.Errorf(errors.CodeForbidden, "service account %q is not allowed for workflows in namespace %q", name, namespace)
}

// Check the rendered spec after merging patches and assembling containers and volumes.
func (woc *wfOperationCtx) validateWorkflowPodSpec(spec *apiv1.PodSpec) error {
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
