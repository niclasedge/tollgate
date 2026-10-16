package redact

import (
	"bytes"
	"strings"
	"testing"
)

// The return path reports how many placeholders it translated back (#87),
// across delta boundaries and blocks, and the stream stays as before.
func TestCopyRehydratedStatsCountsReplacements(t *testing.T) {
	in := msgStart + blockStart(0, "text") +
		textDelta(0, "Mail an <redac") + textDelta(0, "ted:email") + textDelta(0, "_1> und <redacted:email_1>.") +
		blockStop(0) + blockStart(1, "tool_use") + jsonDelta(1, `{"to":"<redacted:email_1>","x":"<redacted:unknown_9>"}`) + blockStop(1) +
		msgDelta + msgStop
	var out bytes.Buffer
	var st ReturnStats
	if _, err := CopyRehydratedStats(&out, nil, strings.NewReader(in), testMapping, nil, &st); err != nil {
		t.Fatal(err)
	}
	if st.Replacements != 3 {
		t.Fatalf("replacements = %d, want 3", st.Replacements)
	}
	if st.Duration < 0 {
		t.Fatalf("negative duration %v", st.Duration)
	}
	if want := rewrite(t, in, testMapping); out.String() != want {
		t.Fatal("stats variant changes the stream")
	}

	var none ReturnStats
	if _, err := CopyRehydratedStats(&out, nil, strings.NewReader(in), nil, nil, &none); err != nil || none != (ReturnStats{}) {
		t.Fatalf("without mapping: %+v %v", none, err)
	}
}

func TestRehydrateBodyCount(t *testing.T) {
	body := []byte(`{"content":[{"type":"text","text":"an <redacted:email_1> und <redacted:email_1>"},{"type":"thinking","thinking":"<redacted:email_1>"}]}`)
	out, n := RehydrateBodyCount(body, testMapping)
	if n != 2 || !bytes.Equal(out, RehydrateBody(body, testMapping)) {
		t.Fatalf("n = %d, out %s", n, out)
	}
	if _, n := RehydrateBodyCount([]byte(`not json <redacted:email_1>`), testMapping); n != 0 {
		t.Fatalf("unchanged body counted %d", n)
	}
}
