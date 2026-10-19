package app

import "sort"

func identityAttributeRank(element syntaxElement) (int, bool) {
	if element.attribute != nil {
		switch element.attribute.Name {
		case "id":
			return 1, true
		case "name":
			return 2, true
		case "display_name":
			return 3, true
		}
	}
	return 0, false
}

func commonElementRank(element syntaxElement) (int, bool) {
	if rank, identity := identityAttributeRank(element); identity {
		return rank, true
	}
	if isTagElement(element) {
		return 7, true
	}
	if element.attribute != nil {
		switch element.attribute.Name {
		case "count", "for_each":
			return 0, true
		case "provider":
			return 4, true
		case "depends_on":
			return 9, true
		default:
			return 5, true
		}
	}
	if element.block != nil {
		if element.block.Type == "lifecycle" {
			return 8, true
		}
		return 6, true
	}
	return 0, false
}

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

// isTagElement reports provider-specific tag/tags attributes or blocks, which
// are placed after general blocks and before lifecycle/depends_on.
func isTagElement(element syntaxElement) bool {
	if element.attribute != nil {
		return element.attribute.Name == "tag" || element.attribute.Name == "tags"
	}
	if element.block != nil {
		return element.block.Type == "tag" || element.block.Type == "tags"
	}
	return false
}
