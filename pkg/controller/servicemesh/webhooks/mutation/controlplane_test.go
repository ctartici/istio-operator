package mutation

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	maistrav2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

var (
	featureDisabled = false
	featureEnabled  = true
)

func TestNoMutation(t *testing.T) {
	testCases := []struct {
		name         string
		controlPlane func() runtime.Object
	}{
		{
			name: "deleted-allowed.v1",
			controlPlane: func() runtime.Object {
				controlPlane := newControlPlaneV1("istio-system")
				controlPlane.Spec.Version = ""
				controlPlane.Spec.Template = ""
				controlPlane.DeletionTimestamp = now()
				return controlPlane
			},
		},
		{
			name: "deleted-allowed.v2",
			controlPlane: func() runtime.Object {
				controlPlane := newControlPlaneV2("istio-system", versions.DefaultVersion)
				controlPlane.Spec.Version = ""
				controlPlane.Spec.Profiles = nil
				controlPlane.DeletionTimestamp = now()
				return controlPlane
			},
		},
		{
			name: "unwatched-namespace.v1",
			controlPlane: func() runtime.Object {
				controlPlane := newControlPlaneV1("not-watched")
				controlPlane.Spec.Version = ""
				controlPlane.Spec.Template = ""
				return controlPlane
			},
		},
		{
			name: "unwatched-namespace.v2",
			controlPlane: func() runtime.Object {
				controlPlane := newControlPlaneV2("not-watched", versions.DefaultVersion)
				controlPlane.Spec.Version = ""
				controlPlane.Spec.Profiles = nil
				return controlPlane
			},
		},
		{
			name: "no-mutation.v1",
			controlPlane: func() runtime.Object {
				controlPlane := newControlPlaneV1("istio-system")
				controlPlane.Spec.Version = versions.DefaultVersion.String()
				controlPlane.Spec.Template = maistrav1.DefaultTemplate
				return controlPlane
			},
		},
		{
			name: "no-mutation.v2_5 with openshiftRoute disabled",
			controlPlane: func() runtime.Object {
				controlPlane := newControlPlaneV2("istio-system", versions.V2_5)
				controlPlane.Spec.Gateways = &maistrav2.GatewaysConfig{
					OpenShiftRoute: &maistrav2.OpenShiftRouteConfig{
						Enablement: maistrav2.Enablement{
							Enabled: &featureDisabled,
						},
					},
				}
				return controlPlane
			},
		},
		{
			name: "no-mutation.v2_5 with openshiftRoute enabled",
			controlPlane: func() runtime.Object {
				controlPlane := newControlPlaneV2("istio-system", versions.V2_5)
				controlPlane.Spec.Gateways = &maistrav2.GatewaysConfig{
					OpenShiftRoute: &maistrav2.OpenShiftRouteConfig{
						Enablement: maistrav2.Enablement{
							Enabled: &featureEnabled,
						},
					},
				}
				return controlPlane
			},
		},
		{
			name: "no-mutation.v2_6 with tracing type set to Jaeger",
			controlPlane: func() runtime.Object {
				controlPlane := newControlPlaneV2("istio-system", versions.V2_6)
				controlPlane.Spec.Gateways = &maistrav2.GatewaysConfig{
					OpenShiftRoute: &maistrav2.OpenShiftRouteConfig{
						Enablement: maistrav2.Enablement{
							Enabled: &featureDisabled,
						},
					},
				}
				controlPlane.Spec.Tracing = &maistrav2.TracingConfig{
					Type: maistrav2.TracerTypeJaeger,
				}
				return controlPlane
			},
		},
		{
			name: "no-mutation.v2_6 with tracing type set to None",
			controlPlane: func() runtime.Object {
				controlPlane := newControlPlaneV2("istio-system", versions.V2_6)
				controlPlane.Spec.Gateways = &maistrav2.GatewaysConfig{
					OpenShiftRoute: &maistrav2.OpenShiftRouteConfig{
						Enablement: maistrav2.Enablement{
							Enabled: &featureDisabled,
						},
					},
				}
				controlPlane.Spec.Tracing = &maistrav2.TracingConfig{
					Type: maistrav2.TracerTypeNone,
				}
				return controlPlane
			},
		},
		{
			name: "no-mutation.v2_4",
			controlPlane: func() runtime.Object {
				controlPlane := newControlPlaneV2("istio-system", versions.V2_4)
				return controlPlane
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mutator := createControlPlaneMutatorTestFixture()
			mutator.namespaceFilter = "istio-system"
			response := mutator.Handle(ctx, newCreateRequest(tc.controlPlane()))
			assert.DeepEquals(response, acceptWithNoMutation, "Expected mutator to accept ServiceMeshControlPlane with no mutation", t)
		})
	}
}

// Test if the webhook defaults Version to the existing Version on an update
func TestCreate(t *testing.T) {
	testCases := []struct {
		name             string
		controlPlanes    func() runtime.Object
		expectedResponse admission.Response
	}{
		{
			name: "default.v2",
			controlPlanes: func() runtime.Object {
				return newEmptyControlPlaneV2("istio-system")
			},
			expectedResponse: acceptV2WithDefaultMutation,
		},
		{
			name: "default.v1",
			controlPlanes: func() runtime.Object {
				return newEmptyControlPlaneV1("istio-system")
			},
			expectedResponse: acceptV1WithDefaultMutation,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			controlPlane := tc.controlPlanes()
			mutator := createControlPlaneMutatorTestFixture()
			response := mutator.Handle(ctx, newCreateRequest(controlPlane))
			assert.DeepEquals(response, tc.expectedResponse, "Expected the response to set the version on create", t)
		})
	}
}

func TestUpgradingToV2_5(t *testing.T) {
	t.Run("upgrading from v2_4 to v2_5", func(t *testing.T) {
		mutator := createControlPlaneMutatorTestFixture()
		response := mutator.Handle(ctx, newUpdateRequest(newControlPlaneV2("istio-system", versions.V2_4), newControlPlaneV2("istio-system", versions.V2_5)))
		assert.DeepEquals(response, acceptWithNoMutation, "Expected mutator to accept ServiceMeshControlPlane with no mutation", t)
	})
}

func TestUpgradingToV2_6(t *testing.T) {
	t.Run("upgrading from v2_5 to v2_6", func(t *testing.T) {
		mutator := createControlPlaneMutatorTestFixture()
		response := mutator.Handle(ctx, newUpdateRequest(newControlPlaneV2("istio-system", versions.V2_5), newControlPlaneV2("istio-system", versions.V2_6)))
		assert.DeepEquals(response, acceptWithNoMutation, "Expected mutator to accept ServieMeshControlPlane with no mutation", t)
	})
}

func TestVersionIsDefaultedToOldSMCPVersionOnUpdate(t *testing.T) {
	testCases := []struct {
		name         string
		controlPlane func() cpadapter
		version      string
	}{
		{
			name: "version.legacy.v1",
			controlPlane: func() cpadapter {
				return &cpv1adapter{ServiceMeshControlPlane: newControlPlaneV1("istio-system")}
			},
		},
		{
			name: "version.legacy.v2",
			controlPlane: func() cpadapter {
				return &cpv2adapter{ServiceMeshControlPlane: newControlPlaneV2("istio-system", versions.DefaultVersion)}
			},
		},
		{
			name: "version.v2.0.v1",
			controlPlane: func() cpadapter {
				return &cpv1adapter{ServiceMeshControlPlane: newControlPlaneV1("istio-system")}
			},
			version: "v2.0",
		},
		{
			name: "version.v2.0.v2",
			controlPlane: func() cpadapter {
				return &cpv2adapter{ServiceMeshControlPlane: newControlPlaneV2("istio-system", versions.DefaultVersion)}
			},
			version: "v2.0",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			controlPlane := tc.controlPlane()
			controlPlane.setVersion(tc.version)
			controlPlane.setTemplate(maistrav1.DefaultTemplate)

			updatedControlPlane := controlPlane.DeepCopy()
			updatedControlPlane.setVersion("")
			updatedControlPlane.SetLabels(map[string]string{"newLabel": "newValue"})

			mutatedControlPlane := updatedControlPlane.DeepCopy()
			mutatedControlPlane.setVersion(tc.version)

			mutator := createControlPlaneMutatorTestFixture(controlPlane.Object())
			response := mutator.Handle(ctx, newUpdateRequest(controlPlane.Object(), updatedControlPlane.Object()))
			expectedResponse := PatchResponse(toRawExtension(updatedControlPlane.Object()), mutatedControlPlane.Object())
			if len(expectedResponse.Patches) == 0 {
				// PatchResponse() always creates a Patches array, so set it to nil if it's empty
				expectedResponse.Patches = nil
			}
			assert.DeepEquals(response, expectedResponse, "Expected the response to set the version to previously AppliedVersion on update", t)
		})
	}
}

func TestTemplateIsDefaultedOnUpdate(t *testing.T) {
	testCases := []struct {
		name         string
		controlPlane func() cpadapter
	}{
		{
			name: "v1",
			controlPlane: func() cpadapter {
				return &cpv1adapter{ServiceMeshControlPlane: newControlPlaneV1("istio-system")}
			},
		},
		{
			name: "v2",
			controlPlane: func() cpadapter {
				return &cpv2adapter{ServiceMeshControlPlane: newControlPlaneV2("istio-system", versions.DefaultVersion)}
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			origControlPlane := tc.controlPlane()
			origControlPlane.setTemplate("")

			updatedControlPlane := origControlPlane.DeepCopy()
			updatedControlPlane.SetLabels(map[string]string{"newLabel": "newValue"})

			mutatedControlPlane := updatedControlPlane.DeepCopy()
			mutatedControlPlane.setProfiles([]string{maistrav1.DefaultTemplate})

			mutator := createControlPlaneMutatorTestFixture()
			response := mutator.Handle(ctx, newUpdateRequest(origControlPlane.Object(), updatedControlPlane.Object()))
			expectedResponse := PatchResponse(toRawExtension(updatedControlPlane.Object()), mutatedControlPlane.Object())
			assert.DeepEquals(response, expectedResponse, "Expected the response to set the template on update", t)
		})
	}
}

func createControlPlaneMutatorTestFixture(clientObjects ...runtime.Object) *ControlPlaneMutator {
	cl, _ := test.CreateClient(clientObjects...)
	decoder, err := admission.NewDecoder(test.GetScheme())
	if err != nil {
		panic(fmt.Sprintf("Could not create decoder: %s", err))
	}
	validator := NewControlPlaneMutator("")

	err = validator.InjectClient(cl)
	if err != nil {
		panic(fmt.Sprintf("Could not inject client: %s", err))
	}

	err = validator.InjectDecoder(decoder)
	if err != nil {
		panic(fmt.Sprintf("Could not inject decoder: %s", err))
	}

	return validator
}

type cpadapter interface {
	metav1.Object
	DeepCopy() cpadapter
	Object() runtime.Object
	setVersion(version string)
	setProfiles(profiles []string)
	setTemplate(template string)
}

type cpv1adapter struct {
	*maistrav1.ServiceMeshControlPlane
}

var _ cpadapter = (*cpv1adapter)(nil)

func (a *cpv1adapter) Object() runtime.Object {
	return a.ServiceMeshControlPlane
}

func (a *cpv1adapter) setVersion(version string) {
	a.Spec.Version = version
}

func (a *cpv1adapter) setProfiles(profiles []string) {
	a.Spec.Profiles = profiles
}

func (a *cpv1adapter) setTemplate(template string) {
	a.Spec.Template = template
	a.Spec.Profiles = nil
}

func (a *cpv1adapter) DeepCopy() cpadapter {
	return &cpv1adapter{ServiceMeshControlPlane: a.ServiceMeshControlPlane.DeepCopy()}
}

type cpv2adapter struct {
	*maistrav2.ServiceMeshControlPlane
}

var _ cpadapter = (*cpv2adapter)(nil)

func (a *cpv2adapter) Object() runtime.Object {
	return a.ServiceMeshControlPlane
}

func (a *cpv2adapter) setVersion(version string) {
	a.Spec.Version = version
}

func (a *cpv2adapter) setProfiles(profiles []string) {
	a.Spec.Profiles = profiles
}

func (a *cpv2adapter) setTemplate(template string) {
	if template != "" {
		a.Spec.Profiles = []string{template}
	} else {
		a.Spec.Profiles = nil
	}
}

func (a *cpv2adapter) DeepCopy() cpadapter {
	return &cpv2adapter{ServiceMeshControlPlane: a.ServiceMeshControlPlane.DeepCopy()}
}

func newControlPlaneV1(namespace string) *maistrav1.ServiceMeshControlPlane {
	return &maistrav1.ServiceMeshControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-smcp",
			Namespace: namespace,
		},
		Spec: maistrav1.ControlPlaneSpec{
			Version:  versions.DefaultVersion.String(),
			Template: maistrav1.DefaultTemplate,
		},
	}
}

func newEmptyControlPlaneV1(namespace string) *maistrav1.ServiceMeshControlPlane {
	return &maistrav1.ServiceMeshControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-smcp",
			Namespace: namespace,
		},
		Spec: maistrav1.ControlPlaneSpec{},
	}
}

func newControlPlaneV2(namespace string, version versions.Version) *maistrav2.ServiceMeshControlPlane {
	return &maistrav2.ServiceMeshControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-smcp",
			Namespace: namespace,
		},
		Spec: maistrav2.ControlPlaneSpec{
			Version:  version.String(),
			Profiles: []string{maistrav1.DefaultTemplate},
		},
	}
}

func newEmptyControlPlaneV2(namespace string) *maistrav2.ServiceMeshControlPlane {
	return &maistrav2.ServiceMeshControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-smcp",
			Namespace: namespace,
		},
		Spec: maistrav2.ControlPlaneSpec{},
	}
}
