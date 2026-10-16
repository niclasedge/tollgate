package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestMatchPlaceholderGrammar(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openTest(t)
	mustClient(t, s, "a")
	for i, tc := range []struct {
		ph string
		ok bool
	}{
		{"", true},
		{"<redacted:email_1>", true},
		{"<redacted:customer_id_999999>", true},
		{"alice@example.com", false},
		{"<redacted:email_0>", false},
		{"<redacted:Email_1>", false},
		{"<redacted:email_1> x", false},
		{"<redacted:email_1234567>", false},
	} {
		tr := sampleTrace(fmt.Sprintf("tr-%d", i), "a", 0)
		tr.Stages[0].Matches[0].Placeholder = tc.ph
		err := s.RecordTrace(ctx, tr)
		if tc.ok && err != nil {
			t.Errorf("%q rejected: %v", tc.ph, err)
		}
		if !tc.ok && !errors.Is(err, ErrInvalid) {
			t.Errorf("%q: err = %v, want ErrInvalid", tc.ph, err)
		}
	}
	got, err := s.GetTrace(ctx, "tr-1")
	if err != nil || got.Stages[0].Matches[0].Placeholder != "<redacted:email_1>" {
		t.Fatalf("round trip: %v %+v", err, got.Stages)
	}
}

// TestMatchMaskGrammar: only an empty mask or a short mask with a bullet
// and no control character is stored (issue #112).
func TestMatchMaskGrammar(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openTest(t)
	mustClient(t, s, "a")
	for i, tc := range []struct {
		mask string
		ok   bool
	}{
		{"", true},
		{"n•••@e•••.org", true},
		{"••• (13)", true},
		{"geheim123", false},
		{"alice@example.com", false},
		{"a•\nb", false},
		{"•" + strings.Repeat("x", 64), false},
		{"\xff•", false},
	} {
		tr := sampleTrace(fmt.Sprintf("tm-%d", i), "a", 0)
		tr.Stages[0].Matches[0].Mask = tc.mask
		err := s.RecordTrace(ctx, tr)
		if tc.ok && err != nil {
			t.Errorf("%q rejected: %v", tc.mask, err)
		}
		if !tc.ok && !errors.Is(err, ErrInvalid) {
			t.Errorf("%q: err = %v, want ErrInvalid", tc.mask, err)
		}
	}
	got, err := s.GetTrace(ctx, "tm-1")
	if err != nil || got.Stages[0].Matches[0].Mask != "n•••@e•••.org" {
		t.Fatalf("round trip: %v %+v", err, got.Stages)
	}
}

func TestRecentRedactions(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openTest(t)
	mustClient(t, s, "a")
	mustClient(t, s, "b")
	// Three traces with 2, 6 and 4 matches; the newest is tr-3.
	mk := func(id, client string, at time.Time, n int) {
		tr := sampleTrace(id, client, 0)
		tr.StartedAt = at
		tr.Stages[0].Matches = nil
		for i := 1; i <= n; i++ {
			tr.Stages[0].Matches = append(tr.Stages[0].Matches,
				Match{Offset: i * 10, Length: 18, Type: "email", Rule: "email", Placeholder: fmt.Sprintf("<redacted:email_%d>", i), Mask: fmt.Sprintf("•••%d", i)})
		}
		if err := s.RecordTrace(ctx, tr); err != nil {
			t.Fatal(err)
		}
	}
	mk("tr-1", "a", t0, 2)
	mk("tr-2", "b", t0.Add(time.Minute), 6)
	mk("tr-3", "a", t0.Add(2*time.Minute), 4)
	// A trace without matches must not appear.
	empty := sampleTrace("tr-4", "b", 0)
	empty.StartedAt = t0.Add(3 * time.Minute)
	empty.Stages[0].Matches = nil
	if err := s.RecordTrace(ctx, empty); err != nil {
		t.Fatal(err)
	}

	got, err := s.RecentRedactions(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 10 {
		t.Fatalf("len = %d, want 10: %+v", len(got), got)
	}
	var order []string
	for _, r := range got {
		order = append(order, r.TraceID+"/"+r.Placeholder)
	}
	want := []string{
		"tr-3/<redacted:email_1>", "tr-3/<redacted:email_2>", "tr-3/<redacted:email_3>", "tr-3/<redacted:email_4>",
		"tr-2/<redacted:email_1>", "tr-2/<redacted:email_2>", "tr-2/<redacted:email_3>", "tr-2/<redacted:email_4>",
		"tr-2/<redacted:email_5>", "tr-2/<redacted:email_6>",
	}
	if fmt.Sprint(order) != fmt.Sprint(want) {
		t.Fatalf("order\n got %v\nwant %v", order, want)
	}
	first := got[0]
	if first.ClientID != "a" || first.ClientName != "name-a" || first.Stage != "redact" || first.Type != "email" ||
		first.Rule != "email" || first.Mask != "•••1" || !first.StartedAt.Equal(t0.Add(2*time.Minute)) {
		t.Fatalf("first = %+v", first)
	}

	if got, err := s.RecentRedactions(ctx, 3); err != nil || len(got) != 3 {
		t.Fatalf("limit 3: %d %v", len(got), err)
	}
	if got, err := s.RecentRedactions(ctx, 0); err != nil || len(got) != 0 {
		t.Fatalf("limit 0: %d %v", len(got), err)
	}
}
