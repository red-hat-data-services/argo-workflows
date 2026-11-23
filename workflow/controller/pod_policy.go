package controller

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"

	apiv1 "k8s.io/api/core/v1"

	"github.com/argoproj/argo-workflows/v3/errors"
)

// The managed deployment enables this after installing admission for Workflow writes.
const workflowAccountAuthorizationEnv = "ARGO_REQUIRE_WORKFLOW_SERVICE_ACCOUNT_AUTHORIZATION"

func (woc *wfOperationCtx) checkWorkflowServiceAccount(name string) error {
	if os.Getenv(workflowAccountAuthorizationEnv) != "true" || name == "" || name == "default" {
		return nil
	}
	// Only an admission-signed account may be used, including on Workflows
	// already stored before the controller was upgraded.
	if woc.hasWorkflowAccountApproval(name) {
		return nil
	}
	return errors.Errorf(errors.CodeForbidden, "service account %q needs workflow admission approval", name)
}

const workflowApprovalAnnotation = "pipelines.opendatahub.io/service-account-approval"
const workflowApprovalKeyEnv = "ARGO_WORKFLOW_APPROVAL_PUBLIC_KEY"

func (woc *wfOperationCtx) hasWorkflowAccountApproval(account string) bool {
	key, err := base64.RawURLEncoding.DecodeString(os.Getenv(workflowApprovalKeyEnv))
	if err != nil {
		return false
	}
	parsed, err := x509.ParsePKIXPublicKey(key)
	if err != nil {
		return false
	}
	publicKey, ok := parsed.(*ecdsa.PublicKey)
	if !ok || publicKey.Curve != elliptic.P256() {
		return false
	}
	body, signature, ok := strings.Cut(woc.orig.Annotations[workflowApprovalAnnotation], ".")
	if !ok {
		return false
	}
	message, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return false
	}
	sig, err := base64.RawURLEncoding.DecodeString(signature)
	digest := sha256.Sum256(message)
	if err != nil || !ecdsa.VerifyASN1(publicKey, digest[:], sig) {
		return false
	}
	var approval struct {
		Namespace string
		Resource  string
		Name      string
		UID       string
		Account   string
	}
	if err := json.Unmarshal(message, &approval); err != nil {
		return false
	}
	return approval.Resource == "workflows" && approval.Namespace == woc.orig.Namespace &&
		approval.Name == woc.orig.Name && approval.Account == account &&
		(approval.UID == "" || approval.UID == string(woc.orig.UID))
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
