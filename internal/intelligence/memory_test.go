package intelligence

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLayeredMemory(t *testing.T) {
	m := NewMemoryAt(filepath.Join(t.TempDir(), "mem.json"))
	if err := m.StoreEx(LayerRepo, "auth", "the auth system uses jwt tokens", StoreOptions{
		Tags: []string{"auth", "jwt"}, Priority: 0.9, Confidence: 0.8, Verified: true,
	}); err != nil {
		t.Fatal(err)
	}
	m.StoreEx(LayerTask, "task-1", "implement login endpoint", StoreOptions{})
	m.StoreEx(LayerTemp, "scratch", "temporary note", StoreOptions{ExpiresAt: time.Now().Add(-time.Hour)})

	if got, ok := m.Get(LayerRepo, "auth"); !ok || got == "" {
		t.Fatal("repo memory missing")
	}

	// Expired entry pruned (search also prunes, so do this first).
	if n := m.PruneExpired(); n != 1 {
		t.Fatalf("expected 1 pruned, got %d", n)
	}
	if _, ok := m.Get(LayerTemp, "scratch"); ok {
		t.Fatal("expired entry should be gone")
	}

	res := m.SearchLayer(LayerRepo, "jwt token auth", 5)
	if len(res) == 0 || res[0].Entry.Key != "auth" {
		t.Fatalf("layer search failed: %+v", res)
	}

	// Search should rank repo layer over task for jwt query.
	res = m.Search("jwt tokens", 5)
	if len(res) == 0 || res[0].Entry.Key != "auth" {
		t.Fatalf("expected auth on top, got %+v", res)
	}
}

func TestMemoryPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mem.json")
	m := NewMemoryAt(path)
	m.StoreEx(LayerFeature, "goal-1", "shipping v0.2.3", StoreOptions{Verified: true, Confidence: 1})
	m.Verify(LayerFeature, "goal-1", true)

	m2 := NewMemoryAt(path)
	content, ok := m2.Get(LayerFeature, "goal-1")
	if !ok || content != "shipping v0.2.3" {
		t.Fatalf("persistence failed: %q %v", content, ok)
	}
}

func TestMemoryConfidenceAndVerify(t *testing.T) {
	m := NewMemoryAt(filepath.Join(t.TempDir(), "mem.json"))
	m.StoreEx(LayerRepo, "k", "some content", StoreOptions{Confidence: 0.3})
	m.SetConfidence(LayerRepo, "k", 0.9)
	m.Verify(LayerRepo, "k", true)
	entries := m.RecallRecent(LayerRepo, 5)
	if len(entries) != 1 {
		t.Fatal("entry missing")
	}
	if entries[0].Confidence != 1.0 || !entries[0].Verified {
		t.Fatalf("expected verified confidence 1.0, got %+v", entries[0])
	}
}
