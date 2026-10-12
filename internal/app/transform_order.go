package app

import "sort"

func rankedElementOrder(elements []syntaxElement, rank func(syntaxElement) (int, bool)) ([]int, bool, int) {
	ranks := make([]int, len(elements))
	indices := make([]int, len(elements))
	previousRank := -1
	needsSort := false
	unsupportedIndex := -1

	for index, element := range elements {
		indices[index] = index
		elementRank, supported := rank(element)
		if !supported {
			if unsupportedIndex == -1 {
				unsupportedIndex = index
			}
			continue
		}
		ranks[index] = elementRank
		if elementRank < previousRank {
			needsSort = true
		}
		previousRank = elementRank
	}

	if needsSort && unsupportedIndex == -1 {
		sort.SliceStable(indices, func(left, right int) bool {
			return ranks[indices[left]] < ranks[indices[right]]
		})
	}
	return indices, needsSort, unsupportedIndex
}
