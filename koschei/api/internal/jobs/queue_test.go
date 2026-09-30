package jobs

import (
	"errors"
	"testing"

	"koschei/api/internal/workerwake"
)

type recordingWakeQueue struct {
	published []Job
	err       error
}

func (q *recordingWakeQueue) Publish(job Job) error {
	q.published = append(q.published, job)
	return q.err
}

func (q *recordingWakeQueue) Close() error { return nil }

func TestNormalizedNATSPrefix(t *testing.T) {
	cases := map[string]string{
		"":                 defaultNATSSubjectPrefix,
		"  koschei.web3  ": "koschei.web3",
		".custom.jobs.":    "custom.jobs",
		"bad > prefix":     defaultNATSSubjectPrefix,
		"bad*prefix":       defaultNATSSubjectPrefix,
	}
	for input, want := range cases {
		if got := normalizedNATSPrefix(input); got != want {
			t.Fatalf("normalizedNATSPrefix(%q)=%q want=%q", input, got, want)
		}
	}
}

func TestNATSJobSubject(t *testing.T) {
	cases := map[string]string{
		"canonical_investigation": "koschei.web3.canonical-investigation",
		"token_scan":              "koschei.web3.token-scan",
		"wallet_score":            "koschei.web3.wallet-score",
		"":                        "koschei.web3.unknown",
		"bad > type":              "koschei.web3.unknown",
	}
	for jobType, want := range cases {
		if got := natsJobSubject("", jobType); got != want {
			t.Fatalf("natsJobSubject(%q)=%q want=%q", jobType, got, want)
		}
	}
}

func TestNATSWakeBindings(t *testing.T) {
	bindings := natsWakeBindings("custom.jobs")
	if len(bindings) != 2 {
		t.Fatalf("bindings=%d want=2", len(bindings))
	}
	want := map[string]string{
		"custom.jobs.canonical-investigation": workerwake.CanonicalInvestigation,
		"custom.jobs.token-scan":              workerwake.CanonicalInvestigation,
	}
	for _, binding := range bindings {
		if got, ok := want[binding.Subject]; !ok || got != binding.WakeName {
			t.Fatalf("unexpected binding subject=%q wake=%q", binding.Subject, binding.WakeName)
		}
		delete(want, binding.Subject)
	}
	if len(want) != 0 {
		t.Fatalf("missing bindings: %#v", want)
	}
}

func TestStoreSignalJobPublishesRemoteHint(t *testing.T) {
	queue := &recordingWakeQueue{}
	store := &Store{WakeQueue: queue}
	job := Job{ID: "job-1", Type: "canonical_investigation"}
	store.signalJob(job)
	if len(queue.published) != 1 || queue.published[0].ID != job.ID {
		t.Fatalf("published=%#v", queue.published)
	}
}

func TestStoreSignalJobRemoteFailureDoesNotChangeDurableOutcome(t *testing.T) {
	queue := &recordingWakeQueue{err: errors.New("wake bus unavailable")}
	store := &Store{WakeQueue: queue}
	store.signalJob(Job{ID: "job-2", Type: "canonical_investigation"})
	if len(queue.published) != 1 {
		t.Fatalf("remote wake attempts=%d want=1", len(queue.published))
	}
}
