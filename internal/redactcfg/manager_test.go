package redactcfg

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	tollgate "github.com/niclasedge/tollgate"
	"github.com/niclasedge/tollgate/internal/redact"
	"github.com/niclasedge/tollgate/internal/store"
)

// fakeSealer is AES-256-GCM with a random key and the ID as AAD, like
// cryptobox. It counts calls and remembers the IDs it saw.
type fakeSealer struct {
	aead cipher.AEAD
	mu   sync.Mutex
	ids  []string
}

func newFakeSealer(t *testing.T) *fakeSealer {
	t.Helper()
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	return &fakeSealer{aead: aead}
}

func (f *fakeSealer) Seal(id string, plain []byte) ([]byte, error) {
	f.mu.Lock()
	f.ids = append(f.ids, id)
	f.mu.Unlock()
	nonce := make([]byte, f.aead.NonceSize())
	_, _ = rand.Read(nonce)
	return f.aead.Seal(nonce, nonce, plain, []byte(id)), nil
}

func (f *fakeSealer) Unseal(id string, data []byte) ([]byte, error) {
	n := f.aead.NonceSize()
	if len(data) < n {
		return nil, errors.New("short")
	}
	return f.aead.Open(nil, data[:n], data[n:], []byte(id))
}

var _ redactResolver = (*Manager)(nil)

// redactResolver mirrors redact.Resolver from #13.
type redactResolver interface {
	Redactor(ctx context.Context, version int64) (*redact.Redactor, error)
}

type env struct {
	st     *store.Store
	m      *Manager
	sealer *fakeSealer
	logs   *bytes.Buffer
	dbPath string
}

func newEnv(t *testing.T, withSealer bool) *env {
	t.Helper()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "tollgate.db")
	st, err := store.Open(ctx, dbPath, store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	e := &env{st: st, logs: &bytes.Buffer{}, dbPath: dbPath}
	opts := Options{Logger: slog.New(slog.NewJSONHandler(e.logs, &slog.HandlerOptions{Level: slog.LevelDebug})), Fixtures: tollgate.Fixtures()}
	if withSealer {
		e.sealer = newFakeSealer(t)
		opts.Sealer = e.sealer
	}
	e.m, err = Open(ctx, st, opts)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *env) importYAML(t *testing.T, src string, activate bool) VersionInfo {
	t.Helper()
	v, err := e.m.Import(context.Background(), ImportRequest{YAML: []byte(src), Comment: "test", Activate: activate})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	return v
}

const wordsV2 = "version: 1\nwords:\n  list: [\"Projekt Falke\", \"Geheimprojekt Zander\"]\n"

func TestOpenCreatesDefaultVersion(t *testing.T) {
	e := newEnv(t, false)
	ctx := context.Background()
	if v := e.m.Active().Version; v != 1 {
		t.Fatalf("active = %d, want 1", v)
	}
	id, err := e.st.ActiveConfigVersionID(ctx)
	if err != nil || id != 1 {
		t.Fatalf("store active = %d, %v", id, err)
	}
	stored, _ := e.st.ConfigVersionByID(ctx, 1)
	if stored.Author != AuthorSystem || len(stored.WordsEnc) != 0 {
		t.Fatalf("version 1 = %+v", stored)
	}
	d, err := Parse(stored.Content)
	if err != nil || !reflect.DeepEqual(d, Default()) {
		t.Fatalf("version 1 is not the default: %+v %v", d, err)
	}
	// Reopening does not create another version.
	m2, err := Open(ctx, e.st, Options{})
	if err != nil || m2.Active().Version != 1 {
		t.Fatalf("reopen: %v", err)
	}
	if vs, _ := e.st.ListConfigVersions(ctx, 0); len(vs) != 1 {
		t.Fatalf("versions = %d", len(vs))
	}
	// Version 0 stays the built-in default without store access.
	c0, err := e.m.Get(ctx, 0)
	if err != nil || c0.Version != 0 {
		t.Fatalf("Get(0) = %+v, %v", c0, err)
	}
}

func TestVersionsDiffAndRollback(t *testing.T) {
	e := newEnv(t, true)
	ctx := context.Background()
	v2 := e.importYAML(t, agentsExample, true)
	if v2.ID != 2 || !v2.Active || !v2.HasWords {
		t.Fatalf("v2 = %+v", v2)
	}
	if !strings.Contains(v2.Diff, "+   - name: kundennummer") || !strings.Contains(v2.Diff, "+     - P••••••••e") {
		t.Fatalf("diff:\n%s", v2.Diff)
	}
	if strings.Contains(v2.Diff, "Projekt Falke") {
		t.Fatalf("diff leaks a word:\n%s", v2.Diff)
	}
	if !strings.Contains(v2.Diff, "- custom: []") {
		t.Fatalf("diff misses removed line:\n%s", v2.Diff)
	}

	before, _ := e.st.ConfigVersionByID(ctx, 2)
	v3, err := e.m.Rollback(ctx, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if v3.ID != 3 || !v3.Active || v3.Author != AuthorAdmin || v3.Comment != "Rollback auf Version 1" {
		t.Fatalf("v3 = %+v", v3)
	}
	if e.m.Active().Version != 3 {
		t.Fatalf("active = %d", e.m.Active().Version)
	}
	after, _ := e.st.ConfigVersionByID(ctx, 2)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("version 2 changed")
	}
	v1, _ := e.st.ConfigVersionByID(ctx, 1)
	s3, _ := e.st.ConfigVersionByID(ctx, 3)
	if !bytes.Equal(v1.Content, s3.Content) {
		t.Fatal("rollback content differs from version 1")
	}
	// Rolling back to a version with words restores them.
	if _, err := e.m.Rollback(ctx, 2, ""); err != nil {
		t.Fatal(err)
	}
	if got := e.m.Active().Redactor.Redact("Projekt Falke"); got != "<redacted:word>" {
		t.Fatalf("rollback to v2 lost words: %q", got)
	}

	vs, err := e.m.Versions(ctx, 0)
	if err != nil || len(vs) != 4 || vs[0].ID != 4 || !vs[0].Active || vs[1].Active {
		t.Fatalf("versions = %+v, %v", vs, err)
	}
	if !vs[0].CreatedAt.Equal(vs[0].CreatedAt.UTC()) || vs[0].CreatedAt.IsZero() {
		t.Fatalf("created_at = %v", vs[0].CreatedAt)
	}
}

func TestImportWithoutActivate(t *testing.T) {
	e := newEnv(t, true)
	v := e.importYAML(t, agentsExample, false)
	if v.Active || e.m.Active().Version != 1 {
		t.Fatalf("import activated: %+v", v)
	}
	if err := e.m.Activate(context.Background(), v.ID); err != nil {
		t.Fatal(err)
	}
	if e.m.Active().Version != v.ID {
		t.Fatal("not activated")
	}
	if err := e.m.Activate(context.Background(), 99); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("activate missing = %v", err)
	}
}

func TestImportRejectsInvalid(t *testing.T) {
	e := newEnv(t, true)
	_, err := e.m.Import(context.Background(), ImportRequest{YAML: []byte("version: 1\nbuiltin:\n  emial: true\n")})
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v", err)
	}
	if vs, _ := e.m.Versions(context.Background(), 0); len(vs) != 1 {
		t.Fatalf("invalid import stored a version")
	}
}

func TestWithoutSealer(t *testing.T) {
	e := newEnv(t, false)
	ctx := context.Background()
	_, err := e.m.Import(ctx, ImportRequest{YAML: []byte(wordsV2), Activate: true})
	if !errors.Is(err, ErrNoSealer) {
		t.Fatalf("err = %v, want ErrNoSealer", err)
	}
	if vs, _ := e.m.Versions(ctx, 0); len(vs) != 1 {
		t.Fatal("version stored without sealer")
	}
	// Without words no sealer is needed.
	e.importYAML(t, "version: 1\nallowlist: [\"KD-000000\"]\n", true)

	// A stored version with words cannot be loaded without the sealer.
	with := newEnv(t, true)
	with.importYAML(t, wordsV2, true)
	if _, err := Open(ctx, with.st, Options{}); !errors.Is(err, ErrNoSealer) {
		t.Fatalf("open without sealer = %v", err)
	}
	// The dry run flags it.
	res, err := e.m.DryRun(ctx, []byte(wordsV2))
	if err != nil || !res.NeedsSealer {
		t.Fatalf("dry run = %+v, %v", res, err)
	}
}

func TestSealerAAD(t *testing.T) {
	e := newEnv(t, true)
	e.importYAML(t, wordsV2, true)
	e.importYAML(t, agentsExample, true)
	if got := e.sealer.ids; !reflect.DeepEqual(got, []string{"config_versions/2", "config_versions/3"}) {
		t.Fatalf("seal ids = %q", got)
	}
	ctx := context.Background()
	v2, _ := e.st.ConfigVersionByID(ctx, 2)
	if _, err := e.sealer.Unseal("config_versions/3", v2.WordsEnc); err == nil {
		t.Fatal("ciphertext opens under another version ID")
	}
}

func TestPinnedVersionKeepsRules(t *testing.T) {
	e := newEnv(t, true)
	ctx := context.Background()
	if _, err := e.st.CreateClient(ctx, store.Client{ID: "c1", Name: "laptop", KeyHash: sum("k"), KeyHint: "tg_abcd", Status: store.ClientActive}); err != nil {
		t.Fatal(err)
	}
	key := sum("c1|session")
	s, err := e.st.PinSession(ctx, store.Session{KeyHash: key, ClientID: "c1", ConfigVersionID: e.m.Active().Version})
	if err != nil {
		t.Fatal(err)
	}
	v2 := e.importYAML(t, wordsV2, true)
	// The session asks again; the store keeps version 1.
	s, err = e.st.PinSession(ctx, store.Session{KeyHash: key, ClientID: "c1", ConfigVersionID: e.m.Active().Version})
	if err != nil || s.ConfigVersionID != 1 {
		t.Fatalf("pinned = %+v, %v", s, err)
	}
	text := "Projekt Falke startet"
	old, err := e.m.Redactor(ctx, s.ConfigVersionID)
	if err != nil {
		t.Fatal(err)
	}
	if got := old.Redact(text); got != text {
		t.Fatalf("pinned session sees new words: %q", got)
	}
	cur, _ := e.m.Redactor(ctx, v2.ID)
	if got := cur.Redact(text); got != "<redacted:word> startet" {
		t.Fatalf("new version: %q", got)
	}
}

func sum(s string) []byte { h := sha256.Sum256([]byte(s)); return h[:] }

func TestHotReloadParallelReaders(t *testing.T) {
	e := newEnv(t, true)
	ctx := context.Background()
	v2 := e.importYAML(t, wordsV2, true)
	text := "Projekt Falke und KD-123456"
	want := map[int64]string{1: text, v2.ID: "<redacted:word> und KD-123456"}

	var wg sync.WaitGroup
	stop := make(chan struct{})
	errs := make(chan string, 16)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				c := e.m.Active()
				if got := c.Redactor.Redact(text); got != want[c.Version] {
					select {
					case errs <- c.Redactor.Redact(text):
					default:
					}
					return
				}
				if r, err := e.m.Redactor(ctx, 1); err != nil || r.Redact(text) != text {
					select {
					case errs <- "pinned version 1 changed":
					default:
					}
					return
				}
			}
		}()
	}
	for i := range 40 {
		id := int64(1)
		if i%2 == 0 {
			id = v2.ID
		}
		if err := e.m.Activate(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	close(stop)
	wg.Wait()
	close(errs)
	for msg := range errs {
		t.Errorf("reader saw an inconsistent version: %s", msg)
	}
}

func TestCacheEviction(t *testing.T) {
	e := newEnv(t, false)
	ctx := context.Background()
	m, err := Open(ctx, e.st, Options{CacheSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	for i := range 4 {
		e.importYAML(t, "version: 1\nallowlist: [\"KD-00000"+string(rune('0'+i))+"\"]\n", false)
	}
	for _, id := range []int64{1, 2, 3, 4, 2, 5} {
		c, err := m.Get(ctx, id)
		if err != nil || c.Version != id {
			t.Fatalf("Get(%d) = %+v, %v", id, c, err)
		}
	}
	if len(m.cache) != 2 || !reflect.DeepEqual(m.lru, []int64{2, 5}) {
		t.Fatalf("cache = %v, lru = %v", len(m.cache), m.lru)
	}
	if _, err := m.Get(ctx, 42); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing version = %v", err)
	}
}

func TestDryRun(t *testing.T) {
	e := newEnv(t, true)
	res, err := e.m.DryRun(context.Background(), []byte(agentsExample))
	if err != nil {
		t.Fatal(err)
	}
	if res.ActiveVersion != 1 || res.NeedsSealer {
		t.Fatalf("res = %+v", res)
	}
	var fr *FixtureResult
	for i := range res.Fixtures {
		if res.Fixtures[i].Name == "redact-dry-run-messages.json" {
			fr = &res.Fixtures[i]
		}
	}
	if fr == nil || fr.Error != "" || fr.Targets != 3 {
		t.Fatalf("fixture result = %+v", res.Fixtures)
	}
	count := func(hs []Hit, typ string) int {
		n := 0
		for _, h := range hs {
			if h.Type == typ {
				n++
			}
		}
		return n
	}
	// Active (default): e-mail and connection password only.
	if count(fr.Active, "word") != 0 || count(fr.Active, "customer_id") != 0 || count(fr.Active, "email") != 1 || count(fr.Active, "connection_password") != 1 {
		t.Errorf("active hits = %+v", fr.Active)
	}
	// Candidate: words (incl. line break), customer IDs; noreply@ stays allowed.
	if count(fr.Candidate, "word") != 3 || count(fr.Candidate, "customer_id") != 2 || count(fr.Candidate, "email") != 1 {
		t.Errorf("candidate hits = %+v", fr.Candidate)
	}
	for _, h := range fr.Candidate {
		if h.Path == "" || h.End <= h.Start {
			t.Errorf("hit without position: %+v", h)
		}
	}
	// Invalid YAML is rejected before any fixture runs.
	if _, err := e.m.DryRun(context.Background(), []byte("version: 2\n")); err == nil {
		t.Fatal("invalid YAML accepted")
	}
}

func TestDryRunBadFixture(t *testing.T) {
	e := newEnv(t, false)
	e.m.fixtures = fstest.MapFS{
		"a.json": {Data: []byte(`{"body": {"messages": [{"role": "user", "content": "KD-123456"}]}}`)},
		"b.json": {Data: []byte(`not json`)},
		"c.txt":  {Data: []byte(`ignored`)},
	}
	res, err := e.m.DryRun(context.Background(), []byte("version: 1\ncustom:\n  - {name: k, pattern: 'KD-\\d{6}', placeholder: customer_id, examples: [KD-123456]}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fixtures) != 2 || res.Fixtures[0].Name != "a.json" || len(res.Fixtures[0].Candidate) != 1 || res.Fixtures[1].Error == "" {
		t.Fatalf("res = %+v", res.Fixtures)
	}
}

func TestLiveTest(t *testing.T) {
	e := newEnv(t, true)
	ctx := context.Background()
	e.importYAML(t, agentsExample, true)
	res, err := e.m.Test(ctx, TestRequest{Text: "KD-123456 an jane@muster-gmbh.invalid zu Projekt Falke"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Version != 2 || res.Redacted != "<redacted:customer_id> an <redacted:email> zu <redacted:word>" || len(res.Hits) != 3 {
		t.Fatalf("res = %+v", res)
	}
	if h := res.Hits[0]; h.Start != 0 || h.End != 9 || h.Rule != "custom:kundennummer" {
		t.Fatalf("hit = %+v", h)
	}
	res, err = e.m.Test(ctx, TestRequest{Text: "Projekt Falke", Version: 1})
	if err != nil || res.Redacted != "Projekt Falke" {
		t.Fatalf("version 1: %+v, %v", res, err)
	}
	res, err = e.m.Test(ctx, TestRequest{Text: "Muster AG", YAML: []byte("version: 1\nwords:\n  list: [Muster AG]\n")})
	if err != nil || res.Version != -1 || res.Redacted != "<redacted:word>" {
		t.Fatalf("draft: %+v, %v", res, err)
	}
}

// TestWordListLeak imports a word list, uses every read path and checks
// that no word appears in the database files, the logs or the responses.
func TestWordListLeak(t *testing.T) {
	e := newEnv(t, true)
	ctx := context.Background()
	words := []string{"Geheimprojekt Zander", "Projekt Falke"}
	v := e.importYAML(t, wordsV2, true)

	var outputs []string
	add := func(s string) { outputs = append(outputs, s) }
	vs, _ := e.m.Versions(ctx, 0)
	for _, x := range vs {
		add(x.Diff + x.Comment)
	}
	d, err := e.m.Version(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	add(d.YAML + d.Diff)
	exp, _ := e.m.Export(ctx, v.ID, ExportOmitWords)
	add(string(exp))
	exp, _ = e.m.Export(ctx, v.ID, ExportMaskedWords)
	add(string(exp))
	if _, err := e.m.Rollback(ctx, 1, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := e.m.Rollback(ctx, v.ID, ""); err != nil {
		t.Fatal(err)
	}
	_, err = e.m.Import(ctx, ImportRequest{YAML: []byte("version: 1\nwords:\n  list: [\"Geheimprojekt Zander\", \"\", 7]\n")})
	if err == nil {
		t.Fatal("invalid import accepted")
	}
	add(err.Error())
	res, _ := e.m.Test(ctx, TestRequest{Text: "Geheimprojekt Zander"})
	add(res.Redacted)
	// Word-list operations (#119): result, errors and the new version's diff.
	keep := []byte("version: 1\nwords:\n  list: []\n")
	ops := WordOps{WordsRemove: []int{1}, WordsAdd: []string{"Geheimprojekt Zander", "projekt falke"}}
	opv, err := e.m.Import(ctx, ImportRequest{YAML: keep, WordsFrom: v.ID, WordOps: ops})
	if err != nil {
		t.Fatal(err)
	}
	add(opv.Diff)
	if _, err = e.m.Import(ctx, ImportRequest{YAML: keep, WordsFrom: v.ID, WordOps: WordOps{WordsRemove: []int{9}}}); err == nil {
		t.Fatal("invalid index accepted")
	}
	add(err.Error())
	if _, err = e.m.DryRunDraft(ctx, Draft{YAML: keep, WordsFrom: v.ID, WordOps: WordOps{WordsAdd: []string{"Projekt Falke" + maskDots}}}); err == nil {
		t.Fatal("masked word accepted")
	}
	add(err.Error())

	// Plain export is the only place with the words, behind the warning.
	plain, _ := e.m.Export(ctx, v.ID, ExportPlainWords)
	if !strings.HasPrefix(string(plain), PlainWordsWarning) || !strings.Contains(string(plain), "Geheimprojekt Zander") {
		t.Fatalf("plain export:\n%s", plain)
	}

	db := readDBFiles(t, e.dbPath)
	for _, w := range words {
		if bytes.Contains(db, []byte(w)) {
			t.Errorf("word %q in database files", w)
		}
		if strings.Contains(e.logs.String(), w) {
			t.Errorf("word %q in logs", w)
		}
		for i, o := range outputs {
			if strings.Contains(o, w) {
				t.Errorf("word %q in output %d: %s", w, i, o)
			}
		}
	}
	if e.logs.Len() == 0 {
		t.Error("expected some log lines")
	}
}

func readDBFiles(t *testing.T, path string) []byte {
	t.Helper()
	var all []byte
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		b, err := os.ReadFile(p)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		all = append(all, b...)
	}
	return all
}
