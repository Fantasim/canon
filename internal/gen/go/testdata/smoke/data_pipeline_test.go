package potions_test

import (
	"strings"
	"sync"
	"testing"
	"time"

	potions "example.com/data/pipeline/out/go"
)

// Smoke test of the pipeline in data mode: examples/pipeline/expected/potions.json loads.
func TestLoadAndReload(t *testing.T) {
	if potions.Store.Current() != nil {
		t.Fatal("a snapshot before the first Reload")
	}
	if err := potions.Store.Reload("testdata/good"); err != nil {
		t.Fatal(err)
	}
	snap := potions.Store.Current()
	ps := snap.Potions()
	l, ok := ps.Find("II_POT_HEAL_L")
	if ps.Len() != 2 || !ok || l.Name() != "IDS_PROPITEM_TXT_POT_L" || l.Heal() != 2500 || l.Cooldown() != 8*time.Second || l.Stack() != 20 || !l.IsStrong() {
		t.Errorf("II_POT_HEAL_L: %+v", l)
	}
	if s := ps.At(1); s.ID() != "II_POT_HEAL_S" || s.Stack() != 99 || s.IsStrong() || s.Cooldown() != 3*time.Second {
		t.Errorf("II_POT_HEAL_S: %+v", s)
	}
	err := potions.Store.Reload("testdata/stale")
	if err == nil || !strings.Contains(err.Error(), "pipeline.Potion@00000000") || !strings.Contains(err.Error(), potions.PotionsSchema) {
		t.Errorf("a stale file: %v, want both fingerprints", err)
	}
	if err := potions.Store.Reload("testdata/missing"); err == nil {
		t.Error("a missing file loaded")
	}
	if potions.Store.Current() != snap {
		t.Error("a failed Reload replaced the snapshot")
	}
}

// Readers keep a consistent snapshot while reloads succeed and fail (run with -race).
func TestReloadRace(t *testing.T) {
	if err := potions.Store.Reload("testdata/good"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if s := potions.Store.Current(); s == nil || s.Potions().Len() != 2 {
					t.Error("a reader saw no snapshot or a partial one")
					return
				}
			}
		}()
	}
	for i := range 20 {
		dir, fails := "testdata/good", i%2 == 1
		if fails {
			dir = "testdata/stale"
		}
		if err := potions.Store.Reload(dir); (err != nil) != fails {
			t.Errorf("reload %d from %s: %v", i, dir, err)
		}
	}
	close(stop)
	wg.Wait()
}
