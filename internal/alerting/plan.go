package alerting

import (
	"strings"

	koptanv1 "github.com/felukka/koptan/api/v1"
)

// Delivery is one occurrence to send to one channel.
type Delivery struct {
	Occurrence
	Channel string
	// StateKey is where the occurrence's key is recorded once sent.
	StateKey string
}

// Decision says what to send and what to record without sending.
type Decision struct {
	Send []Delivery
	// Record are state entries to store as handled without a notification:
	// baselines, and kinds the Alert does not send.
	Record map[string]string
}

// Input is everything Plan needs.
type Input struct {
	Occurrences []Occurrence
	Channels    []string
	// Wanted are the event kinds to send; empty means all.
	Wanted []koptanv1.AlertEvent
	// Sent is status.lastNotified.
	Sent map[string]string
	// Baseline channels record the current state without sending: all of
	// them when the Alert is new, or a channel just added to it.
	Baseline map[string]bool
}

// StateKey is the status.lastNotified key of a service, event and channel.
func StateKey(service string, kind koptanv1.AlertEvent, channel string) string {
	return service + "/" + string(kind) + "/" + channel
}

// Plan decides which occurrences each channel still needs.
func Plan(in Input) Decision {
	d := Decision{Record: map[string]string{}}
	wanted := map[koptanv1.AlertEvent]bool{}
	for _, w := range in.Wanted {
		wanted[w] = true
	}
	for _, o := range in.Occurrences {
		for _, ch := range in.Channels {
			k := StateKey(o.Event.Service, o.Kind, ch)
			if in.Sent[k] == o.Key {
				continue
			}
			if in.Baseline[ch] || (len(wanted) > 0 && !wanted[o.Kind]) {
				d.Record[k] = o.Key
				continue
			}
			d.Send = append(d.Send, Delivery{Occurrence: o, Channel: ch, StateKey: k})
		}
	}
	return d
}

// Prune drops state entries of services the Alert no longer selects.
func Prune(sent map[string]string, services map[string]bool) map[string]string {
	out := make(map[string]string, len(sent))
	for k, v := range sent {
		if service, _, _ := strings.Cut(k, "/"); services[service] {
			out[k] = v
		}
	}
	return out
}

// ChannelsInState lists the channels that appear in state keys.
func ChannelsInState(sent map[string]string) map[string]bool {
	out := map[string]bool{}
	for k := range sent {
		if i := strings.LastIndex(k, "/"); i >= 0 {
			out[k[i+1:]] = true
		}
	}
	return out
}
