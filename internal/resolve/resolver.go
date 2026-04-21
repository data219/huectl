package resolve

import (
	"strings"

	"github.com/data219/huectl/internal/domain"
)

type Candidate struct {
	BridgeID     string
	BridgeName   string
	ResourceID   string
	V1ResourceID string
	ResourceName string
	ResourceType string
}

func ResolveTarget(target string, candidates []Candidate, broadcast bool) ([]Candidate, *domain.AppError) {
	needle := strings.TrimSpace(target)
	if needle == "" {
		return nil, domain.NewError("TARGET_REQUIRED", "target must not be empty", domain.ExitUsage)
	}

	matches := make([]Candidate, 0)
	for _, candidate := range candidates {
		if strings.EqualFold(candidate.ResourceID, needle) ||
			strings.EqualFold(candidate.V1ResourceID, needle) ||
			strings.EqualFold(candidate.ResourceName, needle) {
			matches = append(matches, candidate)
		}
	}

	if len(matches) == 0 {
		return nil, &domain.AppError{
			Code:     "TARGET_NOT_FOUND",
			Message:  "target could not be resolved",
			ExitCode: domain.ExitTarget,
			Details: map[string]any{
				"target": needle,
			},
			Hints: []string{"Use list command to inspect available resources", "Use --bridge to narrow the scope"},
		}
	}

	if len(matches) > 1 && !broadcast {
		cands := make([]map[string]any, 0, len(matches))
		for _, match := range matches {
			cands = append(cands, map[string]any{
				"bridge_id":      match.BridgeID,
				"bridge_name":    match.BridgeName,
				"resource_id":    match.ResourceID,
				"v1_resource_id": match.V1ResourceID,
				"resource_name":  match.ResourceName,
				"resource_type":  match.ResourceType,
			})
		}

		return nil, &domain.AppError{
			Code:     "TARGET_AMBIGUOUS",
			Message:  "target matches multiple resources",
			ExitCode: domain.ExitTarget,
			Details: map[string]any{
				"target":     needle,
				"candidates": cands,
			},
			Hints: []string{"Use --bridge", "Use composite id bridge/resource", "Use --broadcast for explicit multi-target write"},
		}
	}

	return matches, nil
}
