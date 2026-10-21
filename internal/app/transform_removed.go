package app

import (
	"bytes"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func removedElementRank(element syntaxElement) (int, bool) {
	if element.attribute != nil {
		if element.attribute.Name == "from" {
			return 0, true
		}
		return 1, true
	}
	if element.block != nil {
		if element.block.Type == "lifecycle" {
			return 2, true
		}
		return 3, true
	}
	return 0, false
}

func removedBlockEdit(parsed *parsedFile, block *hclsyntax.Block) (*sourceEdit, *transformDiagnostic) {
	elements := syntaxElements(block.Body)
	if len(elements) < 2 {
		return nil, nil
	}

	indices, needsSort, _ := rankedElementOrder(elements, removedElementRank)
	if !needsSort {
		return nil, nil
	}
	if block.OpenBraceRange.Start.Line == block.CloseBraceRange.Start.Line {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "single-line removed body is not sorted"}
	}

	chunks, diagnostic := bodyElementChunks(parsed, block, elements, "removed")
	if diagnostic != nil {
		return nil, diagnostic
	}
	var replacement []byte
	for _, index := range indices {
		replacement = append(replacement, chunks[index].text...)
	}
	start := chunks[0].start
	end := chunks[len(chunks)-1].end
	if bytes.Equal(replacement, parsed.source[start:end]) {
		return nil, nil
	}
	return &sourceEdit{start: start, end: end, text: replacement}, nil
}

func removedBlockCandidate(parsed *parsedFile, block *hclsyntax.Block, blockIndex int) ([]byte, *transformDiagnostic) {
	candidate, parsedCandidate, candidateBlock, _, diagnostic := lifecycleBlocksCandidate(parsed, block, blockIndex)
	if diagnostic != nil {
		return nil, diagnostic
	}

	edit, diagnostic := removedBlockEdit(parsedCandidate, candidateBlock)
	if diagnostic != nil {
		return nil, diagnostic
	}
	if edit != nil {
		candidate = applySourceEdits(candidate, []sourceEdit{*edit})
	}
	blockRange := candidateBlock.Range()
	return bytes.Clone(candidate[blockRange.Start.Byte:blockRange.End.Byte]), nil
}

func transformRemovedOrderWithChanges(parsed *parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
	return runBlockTransform(parsed, blockTransformPolicy{
		blockType: "removed",
		rule:      "removed_order",
		reason:    "removed elements should be ordered from, lifecycle, then connection/provisioner in their original relative order",
		candidate: func(parsed *parsedFile, block *hclsyntax.Block, blockIndex int) ([]byte, *transformDiagnostic) {
			return removedBlockCandidate(parsed, block, blockIndex)
		},
		validate: validateRemovedCandidate,
	})
}

func validateRemovedCandidate(original *parsedFile, candidate []byte) *transformDiagnostic {
	return validateSourceCandidate(original, candidate, map[string]sourceBodyPolicy{
		"removed": {
			owner: "removed", rank: removedElementRank,
			children: map[string]sourceBodyPolicy{
				"lifecycle": {owner: "lifecycle", rank: lifecycleElementRank},
			},
		},
	})
}
