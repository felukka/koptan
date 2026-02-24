package alerting

import (
	"testing"

	koptanv1 "github.com/felukka/koptan/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const sha = "0123456789abcdef0123456789abcdef01234567"

func snapshot() Snapshot {
	return Snapshot{
		Service: &koptanv1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "team"},
			Spec:       koptanv1.ServiceSpec{Source: koptanv1.Source{Repo: "https://git.example.com/api.git"}},
			Status:     koptanv1.ServiceStatus{LatestRevision: sha},
		},
		CI: &koptanv1.CI{Status: koptanv1.CIStatus{
			Phase: koptanv1.CIPhaseFailed, BuildingRevision: sha, ObservedGeneration: 2,
			BuildPod: "api-ci-build-x", Message: "build-push failed (exit 1)",
		}},
		CD: &koptanv1.CD{Status: koptanv1.CDStatus{
			Phase: koptanv1.CDPhaseRunning, Image: "ghcr.io/team/api:abc", AvailableReplicas: 2,
		}},
	}
}

func kinds(occ []Occurrence) map[koptanv1.AlertEvent]Occurrence {
	out := map[koptanv1.AlertEvent]Occurrence{}
	for _, o := range occ {
		out[o.Kind] = o
	}
	return out
}

func TestEvents(t *testing.T) {
	got := kinds(Events(snapshot()))
	if len(got) != 3 {
		t.Fatalf("want Push, CIFailed and CDSucceeded, got %v", got)
	}
	if got[koptanv1.AlertPush].Key != sha {
		t.Error("a push is keyed by its commit")
	}
	failed := got[koptanv1.AlertCIFailed]
	if failed.Key != sha+"@2/api-ci-build-x" || failed.Event.Severity != "error" ||
		failed.Event.ID != "team/api/CIFailed/"+failed.Key {
		t.Errorf("unexpected CI failure %+v", failed)
	}
	if got[koptanv1.AlertCDSucceeded].Key != "ghcr.io/team/api:abc" {
		t.Error("a rollout is keyed by its image")
	}

	s := snapshot()
	s.CI.Status.Phase = koptanv1.CIPhaseBuilding
	s.CD.Status.Phase = koptanv1.CDPhaseDeploying
	got = kinds(Events(s))
	if _, ok := got[koptanv1.AlertCIStarted]; !ok {
		t.Error("a building CI is CIStarted")
	}
	if _, ok := got[koptanv1.AlertCDDeploying]; !ok {
		t.Error("a deploying CD is CDDeploying")
	}
	s.CI.Status.Phase = koptanv1.CIPhaseIdle
	if _, ok := kinds(Events(s))[koptanv1.AlertCIStarted]; ok {
		t.Error("an idle CI has no event")
	}
}

func TestPlan(t *testing.T) {
	occ := Events(snapshot())
	channels := []string{"slack", "hook"}

	baseline := Plan(Input{Occurrences: occ, Channels: channels,
		Baseline: map[string]bool{"slack": true, "hook": true}})
	if len(baseline.Send) != 0 || len(baseline.Record) != 6 {
		t.Fatalf("a new Alert records the current state without sending: %+v", baseline)
	}

	// Nothing changed: nothing to do.
	again := Plan(Input{Occurrences: occ, Channels: channels, Sent: baseline.Record})
	if len(again.Send) != 0 || len(again.Record) != 0 {
		t.Errorf("no change must send nothing: %+v", again)
	}

	// A new build fails: both channels get it, once each.
	s := snapshot()
	s.CI.Status.BuildPod = "api-ci-build-y"
	next := Plan(Input{Occurrences: Events(s), Channels: channels, Sent: baseline.Record})
	if len(next.Send) != 2 || next.Send[0].Kind != koptanv1.AlertCIFailed {
		t.Fatalf("want CIFailed on both channels, got %+v", next.Send)
	}
	if next.Send[0].StateKey != "api/CIFailed/slack" {
		t.Errorf("state key = %q", next.Send[0].StateKey)
	}

	// Kinds the Alert does not want are recorded, never sent.
	filtered := Plan(Input{Occurrences: Events(s), Channels: channels, Sent: baseline.Record,
		Wanted: []koptanv1.AlertEvent{koptanv1.AlertCDFailed}})
	if len(filtered.Send) != 0 || len(filtered.Record) != 2 {
		t.Errorf("unwanted kinds must be recorded only: %+v", filtered)
	}
}

func TestStateHelpers(t *testing.T) {
	sent := map[string]string{"api/Push/slack": "a", "web/Push/hook": "b"}
	pruned := Prune(sent, map[string]bool{"api": true})
	if len(pruned) != 1 || pruned["api/Push/slack"] != "a" {
		t.Errorf("Prune = %v", pruned)
	}
	if ch := ChannelsInState(sent); !ch["slack"] || !ch["hook"] || len(ch) != 2 {
		t.Errorf("ChannelsInState = %v", ch)
	}
}
