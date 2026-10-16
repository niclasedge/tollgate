package accounts

import (
	"context"
	"testing"
)

func TestActiveIDFollowsSwitchAndRollback(t *testing.T) {
	m, f, _ := managerEnv(t)
	ctx := context.Background()
	items, err := m.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	id, err := m.ActiveID(ctx)
	if err != nil || id != items[0].ID {
		t.Fatal(id, err)
	}
	b, err := m.Import(ctx, "Second", tokenB)
	if err != nil {
		t.Fatal(err)
	}
	f.failNext = true
	if _, err = m.Activate(ctx, b.ID); err == nil {
		t.Fatal("expected failed upload")
	}
	id, err = m.ActiveID(ctx)
	if err != nil || id != items[0].ID {
		t.Fatal("rollback identity", id, err)
	}
	if _, err = m.Activate(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	id, err = m.ActiveID(ctx)
	if err != nil || id != b.ID {
		t.Fatal("new identity", id, err)
	}
}
