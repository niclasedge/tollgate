package redactcfg

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/niclasedge/tollgate/internal/compress"
	"github.com/niclasedge/tollgate/internal/redact"
	"github.com/niclasedge/tollgate/internal/store"
)

// ErrNoSealer is returned when a word list would have to be stored or read
// without encryption. tollgate never stores the word lists in plain text.
var ErrNoSealer = errors.New("Wortliste braucht Verschlüsselung, es ist kein Schlüssel angeschlossen")

// Sealer encrypts the word lists (ADR-0009: AES-256-GCM, the ID as AAD).
// *cryptobox.Box from #28 satisfies it.
type Sealer interface {
	Seal(id string, plain []byte) ([]byte, error)
	Unseal(id string, data []byte) ([]byte, error)
}

// Store is the part of *store.Store the manager needs.
type Store interface {
	CreateConfigVersion(ctx context.Context, n store.NewConfigVersion) (store.ConfigVersion, error)
	ConfigVersionByID(ctx context.Context, id int64) (store.ConfigVersion, error)
	ListConfigVersions(ctx context.Context, limit int) ([]store.ConfigVersion, error)
	ActiveConfigVersionID(ctx context.Context) (int64, error)
	SetActiveConfigVersion(ctx context.Context, id int64) error
}

// Options configures Open.
type Options struct {
	// Sealer encrypts the word list. Without it, configurations with a word
	// list are rejected and stored versions with one cannot be loaded.
	Sealer Sealer
	// Fixtures holds the *.json request fixtures for the dry run
	// (testdata/fixtures). Optional.
	Fixtures fs.FS
	// Logger receives version IDs and error kinds only. Optional.
	Logger *slog.Logger
	// CacheSize is the number of compiled versions kept (default 8).
	CacheSize int
	// CompressEnabled tells the compress view whether the stage runs.
	CompressEnabled bool
}

// Compiled is one immutable, compiled configuration version. Version 0 is
// the built-in default that exists without the store.
type Compiled struct {
	Version  int64
	Redactor *redact.Redactor
	doc      *Document
}

// Manager owns the versioned runtime configuration: the active version
// behind an atomic pointer (hot reload) and a cache of compiled versions for
// pinned sessions. It is safe for concurrent use.
type Manager struct {
	st       Store
	sealer   Sealer
	fixtures fs.FS
	log      *slog.Logger
	builtin  *Compiled
	compress bool // the compress stage runs (Options.CompressEnabled)

	active atomic.Pointer[Compiled]
	write  sync.Mutex // serialises version creation and activation

	mu        sync.Mutex
	cache     map[int64]*Compiled
	lru       []int64 // least recently used first
	cacheSize int
}

// Author names used for versions created by tollgate itself.
const (
	AuthorSystem = "system"
	AuthorImport = "import"
	AuthorAdmin  = "admin"
)

// Open loads the active version. On an empty database it stores the
// built-in default as version 1 and activates it, so every session pins a
// real version from its first request on.
func Open(ctx context.Context, st Store, opts Options) (*Manager, error) {
	m := &Manager{st: st, sealer: opts.Sealer, fixtures: opts.Fixtures, log: opts.Logger,
		cache: map[int64]*Compiled{}, cacheSize: opts.CacheSize, compress: opts.CompressEnabled}
	if m.log == nil {
		m.log = slog.New(slog.DiscardHandler)
	}
	if m.cacheSize <= 0 {
		m.cacheSize = 8
	}
	def := Default()
	r, err := redact.Compile(def.RedactConfig())
	if err != nil {
		return nil, err
	}
	m.builtin = &Compiled{Version: 0, Redactor: r, doc: def}

	id, err := st.ActiveConfigVersionID(ctx)
	if errors.Is(err, store.ErrNotFound) {
		m.write.Lock()
		v, cerr := m.create(ctx, Default(), AuthorSystem, "Standardkonfiguration (builtin-v1)")
		m.write.Unlock()
		if cerr != nil {
			return nil, fmt.Errorf("redactcfg: create default version: %w", cerr)
		}
		if err := st.SetActiveConfigVersion(ctx, v.ID); err != nil {
			return nil, fmt.Errorf("redactcfg: activate default version: %w", err)
		}
		id, err = v.ID, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redactcfg: read active version: %w", err)
	}
	c, err := m.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("redactcfg: load active version %d: %w", id, err)
	}
	m.active.Store(c)
	return m, nil
}

// Active returns the active configuration. New sessions pin its version.
func (m *Manager) Active() *Compiled { return m.active.Load() }

// Get returns the compiled configuration of a version, independent of which
// version is active (pinned sessions keep theirs). Version 0 is the built-in
// default.
func (m *Manager) Get(ctx context.Context, version int64) (*Compiled, error) {
	if version == 0 {
		return m.builtin, nil
	}
	if version < 0 {
		return nil, store.ErrNotFound
	}
	m.mu.Lock()
	if c, ok := m.cache[version]; ok {
		m.touch(version)
		m.mu.Unlock()
		return c, nil
	}
	m.mu.Unlock()

	d, err := m.load(ctx, version)
	if err != nil {
		return nil, err
	}
	r, err := redact.Compile(d.RedactConfig())
	if err != nil {
		return nil, err
	}
	c := &Compiled{Version: version, Redactor: r, doc: d}

	m.mu.Lock()
	defer m.mu.Unlock()
	if prev, ok := m.cache[version]; ok { // a concurrent Get won; keep one instance
		m.touch(version)
		return prev, nil
	}
	m.cache[version] = c
	m.lru = append(m.lru, version)
	if len(m.lru) > m.cacheSize {
		delete(m.cache, m.lru[0])
		m.lru = m.lru[1:]
	}
	return c, nil
}

// touch moves version to the most recently used end. m.mu must be held.
func (m *Manager) touch(version int64) {
	for i, v := range m.lru {
		if v == version {
			m.lru = append(append(m.lru[:i:i], m.lru[i+1:]...), version)
			return
		}
	}
}

// Redactor implements redact.Resolver (#13): the redactor of a pinned
// version.
func (m *Manager) Redactor(ctx context.Context, version int64) (*redact.Redactor, error) {
	c, err := m.Get(ctx, version)
	if err != nil {
		return nil, err
	}
	return c.Redactor, nil
}

// Compressor implements compress.Resolver (#19): the compress rules of a
// pinned version, so new rules only reach new sessions.
func (m *Manager) Compressor(ctx context.Context, version int64) (*compress.Config, error) {
	c, err := m.Get(ctx, version)
	if err != nil {
		return nil, err
	}
	return c.doc.CompressConfig(), nil
}

func aad(id int64) string { return "config_versions/" + strconv.FormatInt(id, 10) }

// load reads a stored version and restores its word lists.
func (m *Manager) load(ctx context.Context, id int64) (*Document, error) {
	v, err := m.st.ConfigVersionByID(ctx, id)
	if err != nil {
		return nil, err
	}
	d, err := Parse(v.Content)
	if err != nil {
		return nil, fmt.Errorf("redactcfg: stored version %d is invalid: %w", id, err)
	}
	if len(v.WordsEnc) > 0 {
		if m.sealer == nil {
			return nil, ErrNoSealer
		}
		plain, err := m.sealer.Unseal(aad(id), v.WordsEnc)
		if err != nil {
			return nil, fmt.Errorf("redactcfg: unseal word list of version %d failed", id)
		}
		sw, err := decodeSealed(plain)
		if err != nil {
			return nil, fmt.Errorf("redactcfg: word list of version %d is unreadable", id)
		}
		d.Words.List, d.Words.Customers, d.Words.FirstNames, d.Words.LastNames = sw.List, sw.Customers, sw.FirstNames, sw.LastNames
		d.normalize()
	}
	return d, nil
}

// create appends a version: content without word lists, the lists sealed
// with the new ID as AAD, masked diff to the latest version. m.write must be
// held.
func (m *Manager) create(ctx context.Context, d *Document, author, comment string) (store.ConfigVersion, error) {
	if d.Words.any() && m.sealer == nil {
		return store.ConfigVersion{}, ErrNoSealer
	}
	prev := ""
	latest, err := m.st.ListConfigVersions(ctx, 1)
	if err != nil {
		return store.ConfigVersion{}, err
	}
	if len(latest) == 1 {
		pd, err := m.load(ctx, latest[0].ID)
		if err != nil {
			return store.ConfigVersion{}, err
		}
		prev = string(Export(pd, ExportMaskedWords))
	}
	nv := store.NewConfigVersion{
		Author:  author,
		Comment: comment,
		Content: Export(d, ExportOmitWords),
		Diff:    lineDiff(prev, string(Export(d, ExportMaskedWords))),
	}
	if d.Words.any() {
		plain, err := encodeSealed(d.Words)
		if err != nil {
			return store.ConfigVersion{}, err
		}
		nv.Seal = func(id int64) ([]byte, error) { return m.sealer.Seal(aad(id), plain) }
	}
	v, err := m.st.CreateConfigVersion(ctx, nv)
	if err != nil {
		return store.ConfigVersion{}, err
	}
	m.log.Info("redactcfg: version created", "version", v.ID, "author", author)
	return v, nil
}

// activate switches the store and the in-memory pointer. m.write must be
// held.
func (m *Manager) activate(ctx context.Context, id int64) error {
	if id <= 0 {
		return store.ErrNotFound
	}
	c, err := m.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := m.st.SetActiveConfigVersion(ctx, id); err != nil {
		return err
	}
	m.active.Store(c)
	m.log.Info("redactcfg: version activated", "version", id)
	return nil
}

// Activate makes a stored version active without restart. Requests already
// running keep their configuration; sessions keep their pinned version.
func (m *Manager) Activate(ctx context.Context, id int64) error {
	m.write.Lock()
	defer m.write.Unlock()
	return m.activate(ctx, id)
}

// Draft is a YAML draft whose word lists may come from a stored version
// (WordsFrom > 0; the draft lists must then be empty).
type Draft struct {
	YAML      []byte
	WordsFrom int64
	WordOps
}

// ImportRequest is a confirmed import.
type ImportRequest struct {
	YAML     []byte
	Author   string
	Comment  string
	Activate bool
	// WordsFrom takes the word lists of this stored version (0 = off).
	WordsFrom int64
	WordOps
}

// Import validates the YAML and stores it as a new version, optionally
// activating it. Validation errors are *ValidationError.
func (m *Manager) Import(ctx context.Context, in ImportRequest) (VersionInfo, error) {
	d, _, err := m.parseDraft(ctx, Draft{YAML: in.YAML, WordsFrom: in.WordsFrom, WordOps: in.WordOps})
	if err != nil {
		return VersionInfo{}, err
	}
	if in.Author == "" {
		in.Author = AuthorImport
	}
	m.write.Lock()
	defer m.write.Unlock()
	v, err := m.create(ctx, d, in.Author, in.Comment)
	if err != nil {
		return VersionInfo{}, err
	}
	if in.Activate {
		if err := m.activate(ctx, v.ID); err != nil {
			return VersionInfo{}, err
		}
	}
	return m.info(v), nil
}

// Rollback stores the content of version target as a new version and
// activates it. History stays linear; no version is changed.
func (m *Manager) Rollback(ctx context.Context, target int64, author string) (VersionInfo, error) {
	if author == "" {
		author = AuthorAdmin
	}
	m.write.Lock()
	defer m.write.Unlock()
	d, err := m.load(ctx, target)
	if err != nil {
		return VersionInfo{}, err
	}
	v, err := m.create(ctx, d, author, fmt.Sprintf("Rollback auf Version %d", target))
	if err != nil {
		return VersionInfo{}, err
	}
	if err := m.activate(ctx, v.ID); err != nil {
		return VersionInfo{}, err
	}
	return m.info(v), nil
}

func (m *Manager) info(v store.ConfigVersion) VersionInfo {
	return VersionInfo{ID: v.ID, CreatedAt: v.CreatedAt, Author: v.Author, Comment: v.Comment, Diff: v.Diff,
		Active: v.ID == m.Active().Version, HasWords: len(v.WordsEnc) > 0}
}

// Versions lists stored versions, newest first.
func (m *Manager) Versions(ctx context.Context, limit int) ([]VersionInfo, error) {
	vs, err := m.st.ListConfigVersions(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]VersionInfo, len(vs))
	for i, v := range vs {
		out[i] = m.info(v)
	}
	return out, nil
}

// Version returns one version with its masked YAML.
func (m *Manager) Version(ctx context.Context, id int64) (VersionDetail, error) {
	v, err := m.st.ConfigVersionByID(ctx, id)
	if err != nil {
		return VersionDetail{}, err
	}
	d, err := m.load(ctx, id)
	if err != nil {
		return VersionDetail{}, err
	}
	return VersionDetail{VersionInfo: m.info(v), YAML: string(Export(d, ExportMaskedWords))}, nil
}

// Export writes a stored version as YAML; id <= 0 means the active version.
func (m *Manager) Export(ctx context.Context, id int64, mode ExportMode) ([]byte, error) {
	if id <= 0 {
		id = m.Active().Version
	}
	var d *Document
	if id == 0 {
		d = m.builtin.doc
	} else {
		var err error
		if d, err = m.load(ctx, id); err != nil {
			return nil, err
		}
	}
	return Export(d, mode), nil
}

func hits(path string, fs []redact.Finding) []Hit {
	out := make([]Hit, len(fs))
	for i, f := range fs {
		out[i] = Hit{Path: path, Start: f.Start, End: f.End, Type: f.Type, Rule: f.Rule}
	}
	return out
}

// TestRequest runs a text through a version (0 = active) or a draft YAML.
type TestRequest struct {
	Text      string
	Version   int64
	YAML      []byte
	WordsFrom int64
	WordOps
}

// Test is the live test of the filter view: text in, findings out. A draft
// reports Version -1.
func (m *Manager) Test(ctx context.Context, in TestRequest) (TestResult, error) {
	var r *redact.Redactor
	version := int64(-1)
	switch {
	case len(in.YAML) > 0:
		_, cr, err := m.parseDraft(ctx, Draft{YAML: in.YAML, WordsFrom: in.WordsFrom, WordOps: in.WordOps})
		if err != nil {
			return TestResult{}, err
		}
		r = cr
	case in.Version == 0:
		c := m.Active()
		r, version = c.Redactor, c.Version
	default:
		c, err := m.Get(ctx, in.Version)
		if err != nil {
			return TestResult{}, err
		}
		r, version = c.Redactor, c.Version
	}
	fs := r.Find(in.Text)
	red := redact.Replace(in.Text, fs, func(f redact.Finding) string { return "<redacted:" + f.Type + ">" })
	return TestResult{Version: version, Hits: hits("", fs), Redacted: red}, nil
}

// parseDraft parses a draft. With WordsFrom > 0 the word lists of that stored
// version apply, so the filter view never needs them in plain text; the draft
// lists must then be empty. An unknown version is store.ErrNotFound. WordOps
// then edit the general list (or the draft list) and replace single name lists;
// invalid indices are *WordOpError.
func (m *Manager) parseDraft(ctx context.Context, d Draft) (*Document, *redact.Redactor, error) {
	doc, r, err := parse(d.YAML)
	if err != nil || (d.WordsFrom <= 0 && d.WordOps.empty()) {
		return doc, r, err
	}
	if d.WordsFrom > 0 {
		for k, l := range doc.Words.lists() {
			if len(*l) > 0 {
				root, _ := decodeYAML(d.YAML) // parse succeeded, so this does too
				return nil, nil, &ValidationError{Issues: []Issue{issueAt(root, []string{"words", listKeys[k]},
					fmt.Sprintf("Wortliste angegeben und zugleich aus Version %d übernommen – nur eins von beiden", d.WordsFrom))}}
			}
		}
		src, err := m.load(ctx, d.WordsFrom)
		if err != nil {
			return nil, nil, err
		}
		dl, sl := doc.Words.lists(), src.Words.lists()
		for i := range dl {
			*dl[i] = append([]string{}, *sl[i]...)
		}
	}
	if doc.Words.List, err = applyWordOps(doc.Words.List, d.WordOps, doc.Words.CaseSensitive); err != nil {
		return nil, nil, err
	}
	if d.ListsSet != nil {
		ls, err := applyListsSet(*d.ListsSet, doc.Words.CaseSensitive)
		if err != nil {
			return nil, nil, err
		}
		for _, l := range []struct {
			set *[]string
			dst *[]string
		}{{ls.Customers, &doc.Words.Customers}, {ls.FirstNames, &doc.Words.FirstNames}, {ls.LastNames, &doc.Words.LastNames}} {
			if l.set != nil {
				*l.dst = *l.set
			}
		}
	}
	doc.normalize()
	if r, err = redact.Compile(doc.RedactConfig()); err != nil {
		return nil, nil, errors.New("redactcfg: compile draft with the edited word lists failed")
	}
	return doc, r, nil
}

// Lists returns the plain text of the customer, first-name and last-name
// lists of a stored version; id <= 0 means the active one. It is the only
// read path of the lists besides the plain export (ADR-0021): the caller is
// the admin API, which serves it only on an explicit request. Only the
// version is logged.
func (m *Manager) Lists(ctx context.Context, id int64) (NameLists, error) {
	if id <= 0 {
		id = m.Active().Version
	}
	m.log.Info("redactcfg: name lists read", "version", id)
	if id == 0 {
		return Default().Words.nameLists(), nil
	}
	d, err := m.load(ctx, id)
	if err != nil {
		return NameLists{}, err
	}
	return d.Words.nameLists(), nil
}
