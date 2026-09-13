package memory

import "sort"

func Retrieve(hits []Result, minSimilarity float32, budgetTokens, maxCount int, est TokenEstimator) []Memory {
	var ranked []Result
	for _, h := range hits {
		if h.Score >= minSimilarity {
			ranked = append(ranked, h)
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].Score > ranked[j].Score })

	var out []Memory
	used := 0
	for _, h := range ranked {
		if maxCount > 0 && len(out) >= maxCount {
			break
		}
		n := est.Estimate(h.Memory.Content)
		if budgetTokens > 0 && used+n > budgetTokens {
			continue
		}
		out = append(out, h.Memory)
		used += n
	}
	return out
}
