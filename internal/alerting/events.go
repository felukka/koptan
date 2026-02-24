// Package alerting decides what an Alert sends. Events derives the current
// pipeline events of a Service from its status and its CI's and CD's; Plan
// compares them with what was already sent. Both are pure, so the Alert
// controller only reads objects, sends, and records.
package alerting

import (
	"fmt"

	koptanv1 "github.com/felukka/koptan/api/v1"
	"github.com/felukka/koptan/internal/notify"
)

// Snapshot is a Service with its CI and CD (either may be missing).
type Snapshot struct {
	Service *koptanv1.Service
	CI      *koptanv1.CI
	CD      *koptanv1.CD
}

// Occurrence is an event plus the key that identifies this occurrence: a
// new key for the same kind means it happened again.
type Occurrence struct {
	Kind  koptanv1.AlertEvent
	Key   string
	Event notify.Event
}

// Events returns at most one occurrence per kind: the latest one the
// status shows. Transitions that happen between two reconciles collapse
// into the latest.
func Events(s Snapshot) []Occurrence {
	var out []Occurrence
	svc := s.Service
	base := notify.Event{Service: svc.Name, Namespace: svc.Namespace, Repo: svc.Spec.Source.Repo}

	if rev := svc.Status.LatestRevision; rev != "" {
		e := base
		e.Revision = rev
		e.Message = fmt.Sprintf("New revision %s of %s", short(rev), svc.Spec.Source.Repo)
		e.Severity = notify.SeverityInfo
		out = append(out, occurrence(koptanv1.AlertPush, rev, e))
	}
	if ci := s.CI; ci != nil {
		out = append(out, ciEvents(base, ci)...)
	}
	if cd := s.CD; cd != nil {
		out = append(out, cdEvents(base, cd)...)
	}
	return out
}

func ciEvents(base notify.Event, ci *koptanv1.CI) []Occurrence {
	rev := ci.Status.BuildingRevision
	if rev == "" {
		return nil
	}
	// One build = one revision at one spec generation in one Pod.
	key := fmt.Sprintf("%s@%d/%s", rev, ci.Status.ObservedGeneration, ci.Status.BuildPod)
	e := base
	e.Revision = rev
	e.Phase = string(ci.Status.Phase)
	switch ci.Status.Phase {
	case koptanv1.CIPhaseBuilding:
		e.Message = fmt.Sprintf("Building %s", short(rev))
		e.Severity = notify.SeverityInfo
		return []Occurrence{occurrence(koptanv1.AlertCIStarted, key, e)}
	case koptanv1.CIPhaseSucceeded:
		e.Image = ci.Status.Image
		e.Message = fmt.Sprintf("Built %s", ci.Status.Image)
		e.Severity = notify.SeveritySuccess
		return []Occurrence{occurrence(koptanv1.AlertCISucceeded, key, e)}
	case koptanv1.CIPhaseFailed:
		e.Message = "Build failed: " + ci.Status.Message
		e.Severity = notify.SeverityError
		return []Occurrence{occurrence(koptanv1.AlertCIFailed, key, e)}
	}
	return nil
}

func cdEvents(base notify.Event, cd *koptanv1.CD) []Occurrence {
	image := cd.Status.Image
	if image == "" {
		return nil
	}
	e := base
	e.Revision = cd.Status.Revision
	e.Image = image
	e.Phase = string(cd.Status.Phase)
	switch cd.Status.Phase {
	case koptanv1.CDPhaseDeploying:
		e.Message = fmt.Sprintf("Rolling out %s", image)
		e.Severity = notify.SeverityInfo
		return []Occurrence{occurrence(koptanv1.AlertCDDeploying, image, e)}
	case koptanv1.CDPhaseRunning:
		e.Message = fmt.Sprintf("%s is running (%d replicas available)", image, cd.Status.AvailableReplicas)
		e.Severity = notify.SeveritySuccess
		return []Occurrence{occurrence(koptanv1.AlertCDSucceeded, image, e)}
	case koptanv1.CDPhaseFailed:
		e.Message = "Deployment failed: " + cd.Status.Message
		e.Severity = notify.SeverityError
		return []Occurrence{occurrence(koptanv1.AlertCDFailed, image, e)}
	}
	return nil
}

func occurrence(kind koptanv1.AlertEvent, key string, e notify.Event) Occurrence {
	e.Kind = string(kind)
	e.ID = fmt.Sprintf("%s/%s/%s/%s", e.Namespace, e.Service, kind, key)
	return Occurrence{Kind: kind, Key: key, Event: e}
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
