package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
)

func TestWorkflowPodPolicy(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*wfOperationCtx, *wfv1.Template)
		wantError string
	}{
		{"workflow account", func(woc *wfOperationCtx, _ *wfv1.Template) {
			woc.execWf.Spec.ServiceAccountName = "custom"
		}, "service account"},
		{"template account", func(_ *wfOperationCtx, tmpl *wfv1.Template) {
			tmpl.ServiceAccountName = "custom"
		}, "service account"},
		{"executor account", func(_ *wfOperationCtx, tmpl *wfv1.Template) {
			tmpl.Executor = &wfv1.ExecutorConfig{ServiceAccountName: "custom"}
		}, "service account"},
		{"workflow host network", func(woc *wfOperationCtx, _ *wfv1.Template) {
			woc.execWf.Spec.HostNetwork = pointer.Bool(true)
		}, "host network"},
		{"template host volume", func(_ *wfOperationCtx, tmpl *wfv1.Template) {
			tmpl.Volumes = []apiv1.Volume{{
				Name: "host",
				VolumeSource: apiv1.VolumeSource{
					HostPath: &apiv1.HostPathVolumeSource{Path: "/test"},
				},
			}}
			tmpl.Container.VolumeMounts = []apiv1.VolumeMount{{Name: "host", MountPath: "/test"}}
		}, "hostPath"},
		{"workflow patch", func(woc *wfOperationCtx, _ *wfv1.Template) {
			woc.execWf.Spec.PodSpecPatch = "hostIPC: true"
		}, "host network"},
		{"template account alias patch", func(_ *wfOperationCtx, tmpl *wfv1.Template) {
			tmpl.PodSpecPatch = "serviceAccount: custom"
		}, "service account"},
		{"template container patch", func(_ *wfOperationCtx, tmpl *wfv1.Template) {
			tmpl.PodSpecPatch = "initContainers: [{name: init, securityContext: {privileged: true}}]"
		}, "may not run privileged"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			woc := newWoc()
			tmpl := &woc.execWf.Spec.Templates[0]
			test.configure(woc, tmpl)

			_, err := woc.createWorkflowPod(context.Background(), tmpl.Name, []apiv1.Container{*tmpl.Container}, tmpl, &createWorkflowPodOpts{})
			require.ErrorContains(t, err, test.wantError)
			pods, err := listPods(woc)
			require.NoError(t, err)
			require.Empty(t, pods.Items)
		})
	}
}

func TestWorkflowPodPolicyAgentAndArtifactGC(t *testing.T) {
	ctx := context.Background()
	t.Run("agent", func(t *testing.T) {
		woc := newWoc()
		woc.wf.Namespace = "default"
		woc.execWf.Spec.PodSpecPatch = "hostPID: true"
		_, err := woc.createAgentPod(ctx)
		require.ErrorContains(t, err, "host network")
		pods, err := listPods(woc)
		require.NoError(t, err)
		require.Empty(t, pods.Items)
	})
	t.Run("artifact GC", func(t *testing.T) {
		woc := newWoc()
		_, err := woc.createArtifactGCPod(ctx, wfv1.ArtifactGCOnWorkflowCompletion, nil,
			podInfo{podSpecPatch: "serviceAccountName: custom"}, "artifact-gc", nil, nil)
		require.ErrorContains(t, err, "service account")
		pods, err := listPods(woc)
		require.NoError(t, err)
		require.Empty(t, pods.Items)
	})
}

func TestWorkflowServiceAccountPolicy(t *testing.T) {
	woc := newWoc()
	require.NoError(t, woc.checkWorkflowServiceAccount(""))
	require.Error(t, woc.checkWorkflowServiceAccount("runner"))

	woc.controller.Config.AllowedWorkflowServiceAccounts = map[string][]string{
		"team-a": {"runner"},
		"team-b": {},
	}
	woc.wf.Namespace = "team-a"
	require.NoError(t, woc.checkWorkflowServiceAccount("runner"))
	require.Error(t, woc.checkWorkflowServiceAccount(""))
	woc.wf.Namespace = "team-b"
	require.Error(t, woc.checkWorkflowServiceAccount("runner"))
	require.Error(t, woc.checkWorkflowServiceAccount(""))
}
