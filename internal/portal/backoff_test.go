package portal

import (
	"errors"
	"testing"
	"time"
)

func TestRetryDelay(t *testing.T) {
	cases := []struct {
		fails, interval int
		want            time.Duration
	}{
		{1, 86400, time.Minute},
		{2, 86400, 2 * time.Minute},
		{4, 86400, 8 * time.Minute},
		{10, 86400, time.Hour},     // capped at one hour
		{10, 300, 5 * time.Minute}, // capped at the source interval
		{1, 0, time.Minute},
	}
	for _, c := range cases {
		if got := retryDelay(c.fails, c.interval); got != c.want {
			t.Errorf("retryDelay(%d, %d) = %s, want %s", c.fails, c.interval, got, c.want)
		}
	}
}

func TestSyncBackoffGatesScheduledRetries(t *testing.T) {
	b := newSyncBackoff()
	t0 := time.Unix(0, 0)
	fail := errors.New("no such host")

	if !b.begin("source:a", false, t0) {
		t.Fatal("first sync should start")
	}
	if b.begin("source:a", true, t0) {
		t.Fatal("a running sync must not start twice, even when forced")
	}
	b.end("source:a", fail, 86400, t0)

	if b.begin("source:a", false, t0.Add(30*time.Second)) {
		t.Fatal("scheduled retry inside the backoff window should be skipped")
	}
	if !b.begin("source:a", true, t0.Add(30*time.Second)) {
		t.Fatal("manual sync should bypass backoff")
	}
	b.end("source:a", fail, 86400, t0.Add(30*time.Second)) // 2nd failure: wait 2m

	if b.begin("source:a", false, t0.Add(2*time.Minute)) {
		t.Fatal("still inside the 2m window")
	}
	if !b.begin("source:a", false, t0.Add(3*time.Minute)) {
		t.Fatal("retry after the window should start")
	}
	b.end("source:a", nil, 86400, t0.Add(3*time.Minute))
	if !b.begin("source:a", false, t0.Add(3*time.Minute)) {
		t.Fatal("success clears the backoff")
	}
}
