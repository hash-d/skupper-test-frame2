package skupper

import (
	"fmt"
	"github.com/hash-d/frame2/pkg/frames/f2skupper1"
	"testing"

	frame2 "github.com/hash-d/frame2/pkg"
)

func TestSkupperManifest(t *testing.T) {
	r := &frame2.Run{
		T: t,
	}

	expected := []f2skupper1.SkupperManifestContentImage{
		{
			Name:       "quay.io/skupper/skupper-router:main",
			Repository: "https://github.com/skupperproject/skupper-router",
		},
		{
			Name:       "quay.io/skupper/service-controller:master",
			Repository: "https://github.com/skupperproject/skupper",
		},
		{
			Name:       "quay.io/skupper/config-sync:master",
			Repository: "https://github.com/skupperproject/skupper",
		},
		{
			Name:       "quay.io/skupper/flow-collector:master",
			Repository: "https://github.com/skupperproject/skupper",
		},
		{
			Name:       "quay.io/prometheus/prometheus:v2.42.0",
			Repository: "",
		},
	}

	for _, e := range expected {
		individualPhase := frame2.Phase{
			Runner: r,
			Doc:    fmt.Sprintf("Checks that %q is being checked individually, and also for error", e.Repository),
			MainSteps: []frame2.Step{
				{
					Doc: "Positive check",
					Validator: &f2skupper1.SkupperManifest{
						Path: "testdata/manifest.json",
						Expected: f2skupper1.SkupperManifestContent{
							Images: []f2skupper1.SkupperManifestContentImage{
								{
									Name:       e.Name,
									Repository: e.Repository,
								},
							},
						},
					},
				}, {
					// Today, this is overkill, as we do not check Repository.  In practice, it checks that
					// :noexpected many times, with no additional checks
					Doc: "Negative check",
					Validator: &f2skupper1.SkupperManifest{
						Path: "testdata/manifest.json",
						Expected: f2skupper1.SkupperManifestContent{
							Images: []f2skupper1.SkupperManifestContentImage{
								{
									Name:       "quay.io/skupper/skupper-router:notexpected",
									Repository: e.Repository,
								},
							},
						},
					},
					ExpectError: true,
				},
			},
		}

		individualPhase.Run()
	}
}
