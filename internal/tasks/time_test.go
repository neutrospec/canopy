package tasks

import "testing"

func TestLoadOrdersByInstantWithStableInvalidFallback(t *testing.T) {
	w := testWiki(t)
	for id, created := range map[string]string{
		"later": "2026-10-10T02:00:00Z", "earlier": "2026-10-10T10:00:00+09:00",
		"tie-a": "2026-10-10T01:00:00Z", "bad-a": "invalid-a", "bad-b": "invalid-b",
	} {
		if err := writeTask(w, id, &Task{Version: 1, ID: id, Type: TypeEdit, Status: StatusPending, Created: created}); err != nil {
			t.Fatal(err)
		}
	}
	list, err := Load(w)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"earlier", "tie-a", "later", "bad-a", "bad-b"}
	for i, task := range list {
		if task.ID != want[i] {
			t.Fatalf("position %d = %s, want %s", i, task.ID, want[i])
		}
	}
}
