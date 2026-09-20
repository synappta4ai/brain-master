package store_test

import (
	"path/filepath"
	"testing"
	"time"

	"brain-master/backend/internal/queue"
	"brain-master/backend/internal/store"
)

func mustOpen(t *testing.T) (*store.SQLiteStore, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := store.NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	return s, path
}

func TestUpsertAndLoadRoundtrip(t *testing.T) {
	s, _ := mustOpen(t)
	defer s.Close()

	completed := time.Now()
	job := &queue.GenerationJob{
		ID: "j1", Mode: "video", Prompt: "p", NegativePrompt: "n", Model: "m",
		Width: 1280, Height: 720, Frames: 80, Fps: 16, Seed: 42,
		Status: queue.StatusCompleted, Progress: 100, OutputPath: "/outputs/j1.mp4",
		Step: 1, CreatedAt: time.Now(), CompletedAt: &completed,
	}
	if err := s.UpsertJob(job); err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}
	// Actualización (upsert) del mismo ID.
	job.Progress = 55
	job.Status = queue.StatusProcessing
	if err := s.UpsertJob(job); err != nil {
		t.Fatalf("UpsertJob #2: %v", err)
	}

	got, err := s.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("LoadAll = %d jobs, want 1", len(got))
	}
	g := got[0]
	if g.Status != queue.StatusProcessing || g.Progress != 55 || g.OutputPath != "/outputs/j1.mp4" {
		t.Fatalf("roundtrip inconsistente: %+v", g)
	}
	if g.Width != 1280 || g.Frames != 80 || g.Seed != 42 {
		t.Fatalf("campos estáticos perdidos: %+v", g)
	}
	if g.CompletedAt == nil || g.CreatedAt.IsZero() {
		t.Fatal("timestamps no restaurados")
	}
}

func TestPersistenceAcrossReopen(t *testing.T) {
	s, path := mustOpen(t)
	job := &queue.GenerationJob{
		ID: "survivor", Mode: "image", Prompt: "persist", Model: "m",
		Status: queue.StatusCompleted, Progress: 100, OutputPath: "survivor.png",
		Step: 7, CreatedAt: time.Now(),
	}
	if err := s.UpsertJob(job); err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}
	s.Close()

	s2, err := store.NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("reabrir sqlite: %v", err)
	}
	defer s2.Close()
	got, err := s2.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll tras reabrir: %v", err)
	}
	if len(got) != 1 || got[0].ID != "survivor" || got[0].Step != 7 {
		t.Fatalf("persistencia falló al reabrir: %+v", got)
	}
}

func TestLoadAllOrdersByStep(t *testing.T) {
	s, _ := mustOpen(t)
	defer s.Close()

	base := time.Now().Add(-time.Hour)
	for i, id := range []string{"c", "a", "b"} {
		if err := s.UpsertJob(&queue.GenerationJob{
			ID: id, Mode: "video", Prompt: id, Model: "m",
			Status: queue.StatusCompleted, CreatedAt: base.Add(time.Duration(i) * time.Minute),
			Step: int64(i + 2), // pasos 2,3,4 en orden de inserción desordenado
		}); err != nil {
			t.Fatalf("UpsertJob %s: %v", id, err)
		}
	}
	got, err := s.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	wantOrder := []string{"c", "a", "b"}
	for i, w := range wantOrder {
		if got[i].ID != w {
			t.Fatalf("posición %d = %s, want %s (orden por step)", i, got[i].ID, w)
		}
	}
}

func TestLoadAllEmpty(t *testing.T) {
	s, _ := mustOpen(t)
	defer s.Close()
	got, err := s.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll vacío: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("LoadAll = %d jobs en base nueva", len(got))
	}
}
