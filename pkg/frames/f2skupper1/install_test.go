package f2skupper1_test

import (
	"fmt"
	"github.com/hash-d/frame2/pkg/frames/f2skupper1"
	"github.com/hash-d/frame2/pkg/frames/f2skupper1/disruptor"
	"github.com/hash-d/frame2/pkg/frames/f2skupper1/f2sk1const"
	"testing"
	"time"

	frame2 "github.com/hash-d/frame2/pkg"
	"github.com/hash-d/frame2/pkg/disruptors"
	"github.com/hash-d/frame2/pkg/frames/f2k8s"
	"github.com/hash-d/frame2/pkg/frames/f2ocp"
	"github.com/hash-d/frame2/pkg/frames/f2skupper1/topology"
	"github.com/hash-d/frame2/pkg/frames/f2skupper1/topology/topologies"
	"github.com/hash-d/frame2/pkg/subrunner"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
)

// A shallow UI test: test each skupper init flag individually (and
// in some combos), and then check their effects.
//
// TODO This is currently based on https://skupper.io/docs/cli/index.html;
// it needs to be made more complete, covering all skupper init flags
//
// TODO Currently, there is a mix of white and black box tests.  It should
// be ok to leave the white box tests, but all tests should ensure at
// least one black box validation was run
//
// TODO RouterCheck is a good example of frame to be used on this test; most
// if not all tests below should move to a frame like that, perhaps a
// f2skupper.ConfigurationCheck.  That would make the test more readable,
// and the frame could be have version-specific behavior, making this
// test more useful, especially on upgrade testing
func TestSkupperInstallEffects(t *testing.T) {

	testBase := f2k8s.NewTestBase("install-effects")
	runner := &frame2.Run{
		T: t,
	}
	runner.AllowDisruptors([]frame2.Disruptor{
		&disruptor.UpgradeAndFinalize{},
		&disruptor.AlternateSkupper{},
		&disruptor.ConsoleAuth{},
		&disruptor.SkipManifestCheck{},
		&disruptors.KeepWalking{},
		&disruptor.EdgeOnPrivate{},
	})

	var topo topology.Basic

	topo = &topologies.Single{
		Name:              "skupper-install-effects",
		TestBase:          testBase,
		Type:              f2k8s.Private,
		SkipSkupperDeploy: true,
	}
	build := topology.TopologyBuild{
		Topology:     &topo,
		AutoTearDown: true,
	}

	setup := frame2.Phase{
		Runner: runner,
		Doc:    "Create a single, skupper-less namespace",
		Setup: []frame2.Step{
			{
				Modify: &build,
			},
		},
	}
	assert.Assert(t, setup.Run())

	ns, err := topo.Get(f2k8s.Private, 1)
	assert.Assert(t, err)

	basicWait := frame2.RetryOptions{
		Timeout:    time.Minute * 2,
		KeepTrying: true,
	}

	phase := subrunner.Effects[f2skupper1.CliSkupperInstall, *f2skupper1.CliSkupperInstall]{
		ExecutionProfile: subrunner.BOTH,
		BaseFrame: &f2skupper1.CliSkupperInstall{
			Namespace: ns,
		},
		TearDown: []frame2.Step{
			{
				Modify: &f2skupper1.SkupperDelete{
					Namespace: ns,
				},
				Validator: &f2k8s.Pods{
					Namespace: ns,
					Labels: map[string]string{
						"app.kubernetes.io/part-of": "skupper",
					},
					ExpectNone: true,
				},
				ValidatorRetry: frame2.RetryOptions{
					KeepTrying: true,
					Timeout:    time.Minute * 2,
				},
			},
		},
		Combos: map[string][]string{
			// Ensures all deployable resources are present, and check annotations
			"annotations-full": []string{"console", "annotations", "console-auth-openshift"},
		},
		// TODO: move all validators below to individual frames.
		// - Something like validateskupper.Console{}
		// Those frames should
		// - Be version-aware
		// - Allow to pick what type of testing to run (filter)
		//   - K8S resource verification
		//   - Application effect (eg curl)
		//   - or: white box vs black box
		Effects: map[string]subrunner.CauseEffect[f2skupper1.CliSkupperInstall]{
			"defaults": {
				// Do not do combos with this one, as its validations might conflict
				// with other CauseEffect items
				Doc: "Confirm a plain skupper install is successful and with expected default values",
				Patch: f2skupper1.CliSkupperInstall{
					EnableConsole:       false,
					EnableFlowCollector: false,
				},
				ValidatorsRetry: basicWait,
				Validators: []frame2.Validator{
					&f2k8s.Pods{
						Namespace: ns,
						Labels: map[string]string{
							"app.kubernetes.io/part-of": "skupper",
						},
						ExpectExactly:         2,
						NegativeContainerList: []string{"flow-collector"},
					},
					&f2k8s.ConfigMapValidate{
						Namespace: ns,
						Name:      "skupper-site",
						Values: map[string]string{
							"console":                "false",
							"console-authentication": "internal",
							"flow-collector":         "false",
							"service-sync":           "true",
						},
						AbsentKeys: []string{
							"ingress-host",
						},
					},
					&f2skupper1.RouterCheck{
						Namespace: ns,
						Mode:      "interior",
						LogLevel:  "error+",
					},
				},
			},
			"annotations": {
				Doc: "Annotations must be present on all elements created by Skupper",
				Patch: f2skupper1.CliSkupperInstall{
					Annotations: []string{
						"skupper.io/qe-test=true",
						"skupper.io/qe-test-name=annotations",
					},
				},
				ValidatorsRetry: basicWait,
				Validators: []frame2.Validator{
					&f2k8s.Pods{
						Namespace: ns,
						Labels: map[string]string{
							"app.kubernetes.io/part-of": "skupper",
						},
						OtherAnnotations: map[string]string{
							"skupper.io/qe-test":      "true",
							"skupper.io/qe-test-name": "annotations",
						},
					},
					// TODO: others, too?  Services, etc
				},
			},
			"flow-collector": {
				Doc: "Check flow collector without the console",
				Patch: f2skupper1.CliSkupperInstall{
					EnableFlowCollector: true,
				},
				ValidatorsRetry: basicWait,
				Validators: []frame2.Validator{
					&f2k8s.Pods{
						Namespace:     ns,
						Labels:        map[string]string{"skupper.io/component": "service-controller"},
						ContainerList: []string{"flow-collector"},
						ExpectExactly: 1,
					},
					&f2k8s.Pods{
						Namespace: ns,
						Labels: map[string]string{
							"app.kubernetes.io/part-of": "skupper",
						},
						ExpectMin: 3,
					},
					&f2k8s.ConfigMapValidate{
						Namespace: ns,
						Name:      "skupper-site",
						Values: map[string]string{
							"console":        "false",
							"flow-collector": "true",
						},
					},
				},
			},
			"console": {
				Doc: "Check a basic console",
				Patch: f2skupper1.CliSkupperInstall{
					EnableConsole:       true,
					EnableFlowCollector: true,
				},
				ValidatorsRetry: basicWait,
				Validators: []frame2.Validator{
					&f2k8s.Pods{
						Namespace:     ns,
						Labels:        map[string]string{"skupper.io/component": "service-controller"},
						ExpectPhase:   corev1.PodRunning,
						ContainerList: []string{"flow-collector"},
						ExpectExactly: 1,
					},
					&f2k8s.Pods{
						Namespace: ns,
						Labels: map[string]string{
							"app.kubernetes.io/part-of": "skupper",
						},
						ExpectMin: 3,
					},
					&f2k8s.ConfigMapValidate{
						Namespace: ns,
						Name:      "skupper-site",
						Values: map[string]string{
							"console":        "true",
							"flow-collector": "true",
						},
					},
				},
			},
			"console-user": {
				Doc: "Confirms that the console user can be configured",
				Patch: f2skupper1.CliSkupperInstall{
					EnableConsole:       true,
					EnableFlowCollector: true,
					ConsoleUser:         "testuser",
				},
				ValidatorsRetry: basicWait,
				Validators: []frame2.Validator{
					&f2k8s.SecretGet{
						Namespace: ns,
						Name:      "skupper-console-users",
						Keys:      []string{"testuser"},
					},
				},
			},
			"console-password": {
				Doc: "Confirms that the console password can be configured",
				Patch: f2skupper1.CliSkupperInstall{
					EnableConsole:       true,
					EnableFlowCollector: true,
					ConsolePassword:     "testpassword",
				},
				ValidatorsRetry: basicWait,
				Validators: []frame2.Validator{
					&f2k8s.SecretGet{
						Namespace: ns,
						Name:      "skupper-console-users",
						Expect:    map[string][]byte{"admin": []byte("testpassword")},
					},
				},
			},
			"console-user-password": {
				Doc: "Confirms that the console user and password can be configured",
				Patch: f2skupper1.CliSkupperInstall{
					EnableConsole:       true,
					EnableFlowCollector: true,
					ConsoleUser:         "testuser",
					ConsolePassword:     "testpassword",
				},
				ValidatorsRetry: basicWait,
				Validators: []frame2.Validator{
					&f2k8s.SecretGet{
						Namespace: ns,
						Name:      "skupper-console-users",
						Expect:    map[string][]byte{"testuser": []byte("testpassword")},
					},
				},
			},
			"console-auth-internal": {
				Doc: "Confirms that the console authentication can be set to internal",
				Patch: f2skupper1.CliSkupperInstall{
					EnableConsole:       true,
					EnableFlowCollector: true,
					ConsoleAuth:         "internal",
				},
				ValidatorsRetry: basicWait,
				Validators: []frame2.Validator{
					&f2k8s.SecretGet{
						Namespace: ns,
						Name:      "skupper-console-users",
						Keys:      []string{"admin"},
					},
				},
			},
			"console-auth-unsecured": {
				Doc: "Confirms that the console authentication can be set to unsecured",
				Patch: f2skupper1.CliSkupperInstall{
					EnableConsole:       true,
					EnableFlowCollector: true,
					ConsoleAuth:         "unsecured",
				},
				ValidatorsRetry: basicWait,
				Validators: []frame2.Validator{
					&f2k8s.SecretGet{
						Namespace:    ns,
						Name:         "skupper-console-users",
						ExpectAbsent: true,
					},
				},
			},
			"console-auth-openshift": {
				Doc: "Confirms that the console authentication can be set to openshift",
				Patch: f2skupper1.CliSkupperInstall{
					EnableConsole:       true,
					EnableFlowCollector: true,
					ConsoleAuth:         "openshift",
				},
				ValidatorsRetry: basicWait,
				Validators: []frame2.Validator{
					&f2k8s.SecretGet{
						Namespace:    ns,
						Name:         "skupper-console-users",
						ExpectAbsent: true,
					},
					&f2k8s.ContainerValidate{
						Namespace:     ns,
						PodSelector:   f2sk1const.ServiceControllerSelector,
						ContainerName: "oauth-proxy",
					},
				},
			},
			"network-policy": {
				Doc: "Skupper init is able to create its NetworkPolicy",
				Patch: f2skupper1.CliSkupperInstall{
					CreateNetworkPolicy: true,
				},
				ValidatorsRetry: basicWait,
				Validators: []frame2.Validator{
					&f2k8s.NetworkPolicyValidate{
						Namespace: ns,
						Name:      "skupper",
					},
				},
			},
			"cluster-permissions": {
				// TODO: this may give a false negative, if the cluster
				// already had the ClusterRoleBinding before.  Currently,
				// they do not get removed on skupper delete.
				//
				// See
				// - https://github.com/skupperproject/skupper/issues/813
				// - https://github.com/skupperproject/skupper/issues/857
				Doc: "Skupper init can enable cluster-wide permissions.  Attention to false negatives",
				Patch: f2skupper1.CliSkupperInstall{
					EnableClusterPermissions: true,
				},
				ValidatorsRetry: basicWait,
				Validators: []frame2.Validator{
					&f2k8s.ClusterRoleBindingGet{
						Namespace: ns,
						Name: fmt.Sprintf(
							"skupper-service-controller-extended-%s",
							ns.GetNamespaceName(),
						),
					},
					&f2k8s.ConfigMapValidate{
						Namespace: ns,
						Name:      "skupper-site",
						Values:    map[string]string{"cluster-permissions": "true"},
					},
				},
			},
			"site-name": {
				Doc: "Site name is set on both the skupper-site configmap and on the Router configuration",
				Patch: f2skupper1.CliSkupperInstall{
					SiteName: "custom-site-name",
				},
				ValidatorsRetry: basicWait,
				Validators: []frame2.Validator{
					&f2k8s.ConfigMapValidate{
						Namespace: ns,
						Name:      "skupper-site",
						Values:    map[string]string{"name": "custom-site-name"},
					},
					&f2skupper1.RouterCheck{
						Namespace: ns,
						SiteName:  "custom-site-name",
					},
				},
			},
			"router-logging": {
				Doc: "Skupper init set a site name",
				Patch: f2skupper1.CliSkupperInstall{
					RouterLogging: "trace",
				},
				ValidatorsRetry: basicWait,
				Validators: []frame2.Validator{
					&f2k8s.ConfigMapValidate{
						Namespace: ns,
						Name:      "skupper-site",
						Values:    map[string]string{"router-logging": "trace"},
					},
					&f2skupper1.RouterCheck{
						Namespace: ns,
						LogLevel:  "trace+",
					},
				},
			},
			"router-mode-edge": {
				Doc: "Define router mode to edge",
				Patch: f2skupper1.CliSkupperInstall{
					RouterMode: "edge",
				},
				ValidatorsRetry: basicWait,
				Validators: []frame2.Validator{
					&f2skupper1.RouterCheck{
						Namespace: ns,
						Mode:      "edge",
					},
				},
			},
			"ingress-none": {
				Doc: "Ingress: none",
				Patch: f2skupper1.CliSkupperInstall{
					Ingress: "none",
				},
				ValidatorsRetry: basicWait,
				Validators: []frame2.Validator{
					&f2k8s.ConfigMapValidate{
						Namespace: ns,
						Name:      "skupper-site",
						Values:    map[string]string{"ingress": "none"},
					},
				},
				FailValidators: []frame2.Validator{
					// TODO: it is ok to make sure that the routes do not exist,
					// regardless of environment.  However, we need to check that the
					// skupper-router service is not LoadBalancer for non-OCP
					&f2ocp.RouteGet{
						Namespace: ns,
						Name:      "skupper-inter-router",
					},
					&f2ocp.RouteGet{
						Namespace: ns,
						Name:      "skupper-edge",
					},
				},
			},
			"ingress-host": {
				Doc: "Ingress host",
				Patch: f2skupper1.CliSkupperInstall{
					IngressHost: "localhost",
				},
				ValidatorsRetry: basicWait,
				Validators: []frame2.Validator{
					&f2k8s.ConfigMapValidate{
						Namespace: ns,
						Name:      "skupper-site",
						Values:    map[string]string{"ingress-host": "localhost"},
					},
				},
			},
			"service-sync-disable": {
				Doc: "Disable service sync",
				Patch: f2skupper1.CliSkupperInstall{
					DisableServiceSync: true,
				},
				ValidatorsRetry: basicWait,
				Validators: []frame2.Validator{
					&f2k8s.ConfigMapValidate{
						Namespace: ns,
						Name:      "skupper-site",
						Values:    map[string]string{"service-sync": "false"},
					},
				},
			},
			"router-cpu": {
				Doc: "Set the number of CPUs on the router",
				Patch: f2skupper1.CliSkupperInstall{
					RouterCPU: "134m",
				},
				ValidatorsRetry: basicWait,
				Validators: []frame2.Validator{
					&f2k8s.ConfigMapValidate{
						Namespace: ns,
						Name:      "skupper-site",
						Values:    map[string]string{"router-cpu": "134m"},
					},
					&f2k8s.ContainerValidate{
						Namespace:     ns,
						PodSelector:   f2sk1const.RouterSelector,
						ContainerName: "router",
						ExpectExactly: 1,
						CPURequest:    "134m",
						CPULimit:      "134m",
					},
				},
			},
		},
	}.GetPhase(runner)

	assert.Assert(t, phase.Run())

}
