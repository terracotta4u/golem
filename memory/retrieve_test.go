package memory

import "testing"

func TestRetrieveDropsBelowMinSimilarity(t *testing.T) {
	hits := []Result{
		hit("a", 0.82, "high"),
		hit("b", 0.61, "mid"),
		hit("c", 0.19, "low"),
	}
	got := Retrieve(hits, 0.50, 800, 0, byContent{"high": 1, "mid": 1, "low": 1})
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("got %+v, want a then b", ids(got))
	}
}

func TestRetrieveAllBelowFloorIsEmpty(t *testing.T) {
	hits := []Result{
		hit("a", 0.3, "x"),
		hit("b", 0.1, "y"),
	}
	got := Retrieve(hits, 0.50, 800, 0, byContent{"x": 1, "y": 1})
	if len(got) != 0 {
		t.Errorf("got %v, want empty", ids(got))
	}
}

func TestRetrieveExactBudgetBoundary(t *testing.T) {
	hits := []Result{
		hit("a", 0.9, "first"),
		hit("b", 0.8, "second"),
	}
	est := byContent{"first": 5, "second": 5}
	got := Retrieve(hits, 0, 10, 0, est)
	if len(got) != 2 {
		t.Fatalf("got %v, want both at exact budget", ids(got))
	}

	got = Retrieve(hits, 0, 9, 0, est)
	if len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("got %v, want only first when second would exceed", ids(got))
	}
}

func TestRetrieveSkipsOversizedAndContinues(t *testing.T) {
	hits := []Result{
		hit("big", 0.9, "huge"),
		hit("small", 0.8, "ok"),
	}
	got := Retrieve(hits, 0, 10, 0, byContent{"huge": 50, "ok": 3})
	if len(got) != 1 || got[0].ID != "small" {
		t.Fatalf("got %v, want small after skipping oversized", ids(got))
	}
}

func TestRetrieveEmptyContentCostsZero(t *testing.T) {
	hits := []Result{
		hit("empty", 0.9, ""),
		hit("next", 0.8, "body"),
	}
	got := Retrieve(hits, 0, 5, 0, byContent{"": 0, "body": 5})
	if len(got) != 2 || got[0].ID != "empty" || got[1].ID != "next" {
		t.Fatalf("got %v, want empty then next", ids(got))
	}
}

func TestRetrieveNeverExceedsBudget(t *testing.T) {
	hits := []Result{
		hit("a", 0.9, "a"),
		hit("b", 0.8, "b"),
		hit("c", 0.7, "c"),
	}
	got := Retrieve(hits, 0, 10, 0, byContent{"a": 6, "b": 6, "c": 4})
	used := 0
	est := byContent{"a": 6, "b": 6, "c": 4}
	for _, m := range got {
		used += est.Estimate(m.Content)
	}
	if used > 10 {
		t.Errorf("used %d, want <= 10; ids %v", used, ids(got))
	}
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "c" {
		t.Errorf("got %v, want a then c (b skipped as oversized remainder)", ids(got))
	}
}

func TestRetrieveCountCap(t *testing.T) {
	hits := []Result{
		hit("a", 0.9, "a"),
		hit("b", 0.8, "b"),
		hit("c", 0.7, "c"),
	}
	got := Retrieve(hits, 0, 800, 1, byContent{"a": 1, "b": 1, "c": 1})
	if len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("got %v, want only a", ids(got))
	}
}

func hit(id string, score float32, content string) Result {
	return Result{Memory: Memory{ID: id, Content: content}, Score: score}
}

func ids(ms []Memory) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.ID
	}
	return out
}

type byContent map[string]int

func (m byContent) Estimate(s string) int { return m[s] }
