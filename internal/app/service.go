package app

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/data219/huectl/internal/domain"
	"github.com/data219/huectl/internal/hue/common"
	v1client "github.com/data219/huectl/internal/hue/v1"
	v2client "github.com/data219/huectl/internal/hue/v2"
	"github.com/data219/huectl/internal/resolve"
	"github.com/data219/huectl/internal/store/config"
)

type ActionInput struct {
	ID           string
	Name         string
	Address      string
	ResourceType string
	Body         map[string]any
	RawBody      string
	Path         string
	Method       string
	Duration     time.Duration
	File         string
	LightSet     *LightSetInput
	Scene        *SceneInput
	Automation   *AutomationInput
	Sensor       *SensorInput
	Assignment   *GroupAssignmentInput
	Update       *UpdateInput
	RawAPI       *RawAPIInput
}

type LightSetInput struct {
	On           *bool
	Brightness   *int
	Kelvin       *int
	XYX          *float64
	XYY          *float64
	Effect       string
	TransitionMS *int
	AlertAction  string
}

type SceneInput struct {
	Dynamic    *bool
	DurationMS *int
}

type AutomationInput struct {
	Enabled     *bool
	ScriptID    string
	TriggerType string
	Time        string
	Recurrence  string
}

type SensorInput struct {
	Sensitivity *int
	Enabled     *bool
}

type GroupAssignmentInput struct {
	ChildIDs []string
}

type UpdateInput struct {
	Force *bool
}

type RawAPIInput struct {
	Method string
	Path   string
	Body   string
}

type resourceSpec struct {
	PrimaryType  string
	FallbackType string
	ListTypes    []string
}

type Service struct {
	store      *config.FileStore
	httpClient *common.HTTPClient
	v2         *v2client.Client
	v1         *v1client.Client
}

func NewService(configPath string, timeout time.Duration) *Service {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	httpClient := common.NewHTTPClient(common.HTTPClientConfig{
		Timeout:    timeout,
		MaxRetries: 2,
		BaseDelay:  250 * time.Millisecond,
	})
	return &Service{
		store:      config.NewFileStore(configPath),
		httpClient: httpClient,
		v2:         v2client.NewClient(httpClient),
		v1:         v1client.NewClient(httpClient),
	}
}

func (s *Service) Execute(
	ctx context.Context,
	cmdCtx domain.CommandContext,
	domainName string,
	action string,
	input ActionInput,
) (any, bool, *domain.AppError) {
	domainName = strings.TrimSpace(domainName)
	action = strings.TrimSpace(action)

	switch domainName {
	case "bridge":
		return s.executeBridge(ctx, cmdCtx, action, input)
	case "api":
		return s.executeAPI(ctx, cmdCtx, action, input)
	case "backup":
		return s.executeBackup(ctx, cmdCtx, action, input)
	case "diagnose":
		return s.executeDiagnose(ctx, cmdCtx, action, input)
	default:
		return s.executeResourceDomain(ctx, cmdCtx, domainName, action, input)
	}
}

func (s *Service) executeBridge(
	ctx context.Context,
	cmdCtx domain.CommandContext,
	action string,
	input ActionInput,
) (any, bool, *domain.AppError) {
	switch action {
	case "discover":
		results, err := DiscoverBridges(ctx, 3*time.Second)
		if err != nil {
			return nil, false, domain.WrapError("DISCOVERY_FAILED", "failed to discover bridges", domain.ExitNetwork, err)
		}
		return map[string]any{"bridges": results}, false, nil
	case "list":
		cfg, appErr := s.store.Load()
		if appErr != nil {
			return nil, false, appErr
		}
		return map[string]any{"bridges": cfg.Bridges, "default_bridge": cfg.DefaultBridge}, false, nil
	case "show":
		bridges, appErr := s.resolveBridgeScope(cmdCtx)
		if appErr != nil {
			return nil, false, appErr
		}
		return map[string]any{"bridges": bridges}, false, nil
	case "add":
		return s.addBridge(input)
	case "link":
		return s.linkBridge(ctx, cmdCtx)
	case "rename":
		return s.renameBridge(cmdCtx.Bridge, input.Name)
	case "remove":
		return s.removeBridge(cmdCtx.Bridge)
	case "health":
		return s.bridgeHealth(ctx, cmdCtx)
	case "capabilities":
		return s.bridgeCapabilities(ctx, cmdCtx)
	default:
		return nil, false, domain.NewError("COMMAND_UNSUPPORTED", "unsupported bridge action", domain.ExitUsage)
	}
}

func (s *Service) executeAPI(
	ctx context.Context,
	cmdCtx domain.CommandContext,
	action string,
	input ActionInput,
) (any, bool, *domain.AppError) {
	bridges, appErr := s.resolveBridgeScope(cmdCtx)
	if appErr != nil {
		return nil, false, appErr
	}
	if appErr = s.verifyFingerprints(ctx, bridges); appErr != nil {
		return nil, false, appErr
	}

	method := strings.ToUpper(action)
	if input.Method != "" {
		method = strings.ToUpper(input.Method)
	}
	if method == "" {
		method = http.MethodGet
	}

	bodyBytes, appErr := encodeRawBody(input)
	if appErr != nil {
		return nil, false, appErr
	}

	results := make([]domain.BridgeResult, 0, len(bridges))
	partial := false
	for _, bridge := range bridges {
		payload, status, reqErr := s.v2.Raw(ctx, bridge, method, input.Path, bodyBytes)
		if reqErr != nil {
			results = append(results, domain.BridgeResult{
				BridgeID: bridge.ID, BridgeName: bridge.Name, Success: false, Error: reqErr.Error(), StatusCode: status,
			})
			partial = true
			continue
		}
		var parsed any
		if unmarshalErr := json.Unmarshal(payload, &parsed); unmarshalErr != nil {
			parsed = string(payload)
		}
		results = append(results, domain.BridgeResult{
			BridgeID: bridge.ID, BridgeName: bridge.Name, Success: true, StatusCode: status,
			Data: map[string]any{"response": parsed},
		})
	}
	return aggregateResults(results), partial, nil
}

func (s *Service) executeBackup(
	ctx context.Context,
	cmdCtx domain.CommandContext,
	action string,
	input ActionInput,
) (any, bool, *domain.AppError) {
	switch action {
	case "export":
		return s.backupExport(ctx, cmdCtx, input.File)
	case "import":
		return s.backupImport(input.File)
	case "diff":
		return s.backupDiff(ctx, cmdCtx, input.File)
	default:
		return nil, false, domain.NewError("COMMAND_UNSUPPORTED", "unsupported backup action", domain.ExitUsage)
	}
}

func (s *Service) executeDiagnose(
	ctx context.Context,
	cmdCtx domain.CommandContext,
	action string,
	input ActionInput,
) (any, bool, *domain.AppError) {
	bridges, appErr := s.resolveBridgeScope(cmdCtx)
	if appErr != nil {
		return nil, false, appErr
	}
	if appErr = s.verifyFingerprints(ctx, bridges); appErr != nil {
		return nil, false, appErr
	}

	switch action {
	case "ping", "latency":
		results := make([]domain.BridgeResult, 0, len(bridges))
		partial := false
		for _, bridge := range bridges {
			start := time.Now()
			_, status, err := s.v2.List(ctx, bridge, "bridge")
			latency := time.Since(start).Milliseconds()
			if err != nil {
				results = append(results, domain.BridgeResult{BridgeID: bridge.ID, BridgeName: bridge.Name, Success: false, Error: err.Error(), StatusCode: status})
				partial = true
				continue
			}
			results = append(results, domain.BridgeResult{
				BridgeID: bridge.ID, BridgeName: bridge.Name, Success: true, StatusCode: status,
				Data: map[string]any{"latency_ms": latency},
			})
		}
		return aggregateResults(results), partial, nil
	case "events":
		duration := input.Duration
		if duration <= 0 {
			duration = 10 * time.Second
		}
		return s.collectEvents(ctx, bridges, duration)
	case "logs":
		return map[string]any{
			"message": "diagnostic logs are provided via command outputs and JSON envelopes",
			"hint":    "use --json and redirect output to your log collector",
		}, false, nil
	default:
		return nil, false, domain.NewError("COMMAND_UNSUPPORTED", "unsupported diagnose action", domain.ExitUsage)
	}
}

func (s *Service) executeResourceDomain(
	ctx context.Context,
	cmdCtx domain.CommandContext,
	domainName string,
	action string,
	input ActionInput,
) (any, bool, *domain.AppError) {
	spec, ok := resourceSpecs()[domainName]
	if !ok {
		return nil, false, domain.NewError("DOMAIN_UNSUPPORTED", "unsupported domain", domain.ExitUsage)
	}

	bridges, appErr := s.resolveBridgeScope(cmdCtx)
	if appErr != nil {
		return nil, false, appErr
	}
	if appErr = s.verifyFingerprints(ctx, bridges); appErr != nil {
		return nil, false, appErr
	}

	switch action {
	case "list", "check", "status":
		return s.listResources(ctx, bridges, spec)
	case "show":
		target := firstNonEmpty(input.ID, input.Name)
		if target == "" {
			return nil, false, domain.NewError("TARGET_REQUIRED", "show requires --id or --name", domain.ExitUsage)
		}
		return s.showResource(ctx, bridges, spec, target, cmdCtx.Broadcast)
	case "create", "search":
		if len(bridges) > 1 && !cmdCtx.Broadcast {
			return nil, false, &domain.AppError{Code: "WRITE_SCOPE_AMBIGUOUS", Message: "create/search across multiple bridges requires --broadcast or --bridge", ExitCode: domain.ExitTarget, Hints: []string{"Use --bridge <id|name>", "Use --broadcast for explicit multi-bridge writes"}}
		}
		return s.createResource(ctx, bridges, spec, input)
	case "update", "rename", "delete", "activate", "clone", "enable", "disable", "run", "assign", "unassign", "identify", "sensitivity", "install", "on", "off", "toggle", "set", "effect", "flash", "start", "stop":
		return s.writeResourceAction(ctx, cmdCtx, bridges, domainName, action, spec, input)
	default:
		return nil, false, domain.NewError("COMMAND_UNSUPPORTED", "unsupported domain action", domain.ExitUsage)
	}
}

func (s *Service) addBridge(input ActionInput) (any, bool, *domain.AppError) {
	if strings.TrimSpace(input.Address) == "" {
		return nil, false, domain.NewError("ADDRESS_REQUIRED", "bridge add requires --address", domain.ExitUsage)
	}
	cfg, appErr := s.store.Load()
	if appErr != nil {
		return nil, false, appErr
	}

	bridgeID := strings.TrimSpace(input.ID)
	if bridgeID == "" {
		bridgeID = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(input.Name), " ", "-"))
	}
	if bridgeID == "" {
		bridgeID = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(input.Address), ".", "-"))
	}
	if bridgeID == "" {
		return nil, false, domain.NewError("BRIDGE_ID_REQUIRED", "unable to infer bridge id", domain.ExitUsage)
	}
	for _, bridge := range cfg.Bridges {
		if bridge.ID == bridgeID || strings.EqualFold(bridge.Name, input.Name) {
			return nil, false, domain.NewError("BRIDGE_EXISTS", "bridge with same id or name already exists", domain.ExitUsage)
		}
	}

	bridge := domain.Bridge{
		ID:      bridgeID,
		Name:    firstNonEmpty(strings.TrimSpace(input.Name), bridgeID),
		Address: strings.TrimSpace(input.Address),
	}
	cfg.Bridges = append(cfg.Bridges, bridge)
	if cfg.DefaultBridge == "" {
		cfg.DefaultBridge = bridge.ID
	}
	if saveErr := s.store.Save(cfg); saveErr != nil {
		return nil, false, saveErr
	}
	return map[string]any{"bridge": bridge}, false, nil
}

func (s *Service) linkBridge(ctx context.Context, cmdCtx domain.CommandContext) (any, bool, *domain.AppError) {
	bridges, appErr := s.resolveBridgeScope(cmdCtx)
	if appErr != nil {
		return nil, false, appErr
	}
	if len(bridges) != 1 {
		return nil, false, &domain.AppError{Code: "LINK_SCOPE_INVALID", Message: "link requires exactly one bridge scope", ExitCode: domain.ExitUsage, Hints: []string{"Use --bridge <id|name>"}}
	}
	bridge := bridges[0]

	username, clientKey, linkErr := s.v1.Link(ctx, bridge, "huectl#linux")
	if linkErr != nil {
		return nil, false, linkErr
	}
	fingerprint, fpErr := common.FetchCertFingerprint(ctx, bridge.Address, 5*time.Second)
	if fpErr != nil {
		return nil, false, domain.WrapError("FINGERPRINT_FAILED", "linked but failed to capture bridge certificate fingerprint", domain.ExitNetwork, fpErr)
	}

	cfg, loadErr := s.store.Load()
	if loadErr != nil {
		return nil, false, loadErr
	}
	for idx := range cfg.Bridges {
		if cfg.Bridges[idx].ID == bridge.ID {
			cfg.Bridges[idx].Username = username
			cfg.Bridges[idx].ClientKey = clientKey
			cfg.Bridges[idx].CertFingerprint = fingerprint
			cfg.Bridges[idx].APIBaseV2 = common.BridgeBaseV2(cfg.Bridges[idx])
			cfg.Bridges[idx].APIBaseV1 = common.BridgeBaseV1(cfg.Bridges[idx])
		}
	}
	if saveErr := s.store.Save(cfg); saveErr != nil {
		return nil, false, saveErr
	}

	return map[string]any{
		"bridge_id":        bridge.ID,
		"cert_fingerprint": fingerprint,
	}, false, nil
}

func (s *Service) renameBridge(target string, newName string) (any, bool, *domain.AppError) {
	if strings.TrimSpace(target) == "" {
		return nil, false, domain.NewError("TARGET_REQUIRED", "rename requires --bridge <id|name>", domain.ExitUsage)
	}
	if strings.TrimSpace(newName) == "" {
		return nil, false, domain.NewError("NAME_REQUIRED", "rename requires --name", domain.ExitUsage)
	}
	cfg, appErr := s.store.Load()
	if appErr != nil {
		return nil, false, appErr
	}
	for idx := range cfg.Bridges {
		if cfg.Bridges[idx].ID == target || strings.EqualFold(cfg.Bridges[idx].Name, target) {
			cfg.Bridges[idx].Name = newName
			if saveErr := s.store.Save(cfg); saveErr != nil {
				return nil, false, saveErr
			}
			return map[string]any{"bridge": cfg.Bridges[idx]}, false, nil
		}
	}
	return nil, false, domain.NewError("TARGET_NOT_FOUND", "bridge not found", domain.ExitTarget)
}

func (s *Service) removeBridge(target string) (any, bool, *domain.AppError) {
	if strings.TrimSpace(target) == "" {
		return nil, false, domain.NewError("TARGET_REQUIRED", "remove requires --bridge <id|name>", domain.ExitUsage)
	}
	cfg, appErr := s.store.Load()
	if appErr != nil {
		return nil, false, appErr
	}
	bridges := make([]domain.Bridge, 0, len(cfg.Bridges))
	removed := false
	removedID := ""
	for _, bridge := range cfg.Bridges {
		if bridge.ID == target || strings.EqualFold(bridge.Name, target) {
			removed = true
			removedID = bridge.ID
			continue
		}
		bridges = append(bridges, bridge)
	}
	if !removed {
		return nil, false, domain.NewError("TARGET_NOT_FOUND", "bridge not found", domain.ExitTarget)
	}
	cfg.Bridges = bridges
	if cfg.DefaultBridge == removedID {
		cfg.DefaultBridge = ""
		if len(cfg.Bridges) > 0 {
			cfg.DefaultBridge = cfg.Bridges[0].ID
		}
	}
	if saveErr := s.store.Save(cfg); saveErr != nil {
		return nil, false, saveErr
	}
	return map[string]any{"removed": target}, false, nil
}

func (s *Service) bridgeHealth(ctx context.Context, cmdCtx domain.CommandContext) (any, bool, *domain.AppError) {
	bridges, appErr := s.resolveBridgeScope(cmdCtx)
	if appErr != nil {
		return nil, false, appErr
	}
	if appErr = s.verifyFingerprints(ctx, bridges); appErr != nil {
		return nil, false, appErr
	}
	results := make([]domain.BridgeResult, 0, len(bridges))
	partial := false
	for _, bridge := range bridges {
		start := time.Now()
		_, status, err := s.v2.List(ctx, bridge, "bridge")
		if err != nil {
			results = append(results, domain.BridgeResult{BridgeID: bridge.ID, BridgeName: bridge.Name, Success: false, Error: err.Error(), StatusCode: status})
			partial = true
			continue
		}
		results = append(results, domain.BridgeResult{
			BridgeID: bridge.ID, BridgeName: bridge.Name, Success: true, StatusCode: status,
			Data: map[string]any{"latency_ms": time.Since(start).Milliseconds()},
		})
	}
	return aggregateResults(results), partial, nil
}

func (s *Service) bridgeCapabilities(ctx context.Context, cmdCtx domain.CommandContext) (any, bool, *domain.AppError) {
	bridges, appErr := s.resolveBridgeScope(cmdCtx)
	if appErr != nil {
		return nil, false, appErr
	}
	if appErr = s.verifyFingerprints(ctx, bridges); appErr != nil {
		return nil, false, appErr
	}
	results := make([]domain.BridgeResult, 0, len(bridges))
	partial := false
	for _, bridge := range bridges {
		data, status, err := s.v2.List(ctx, bridge, "bridge")
		if err != nil {
			results = append(results, domain.BridgeResult{BridgeID: bridge.ID, BridgeName: bridge.Name, Success: false, Error: err.Error(), StatusCode: status})
			partial = true
			continue
		}
		results = append(results, domain.BridgeResult{BridgeID: bridge.ID, BridgeName: bridge.Name, Success: true, StatusCode: status, Data: map[string]any{"capabilities": data}})
	}
	return aggregateResults(results), partial, nil
}

func (s *Service) listResources(
	ctx context.Context,
	bridges []domain.Bridge,
	spec resourceSpec,
) (any, bool, *domain.AppError) {
	results := make([]domain.BridgeResult, 0, len(bridges))
	partial := false
	for _, bridge := range bridges {
		resources, status, err := s.listForBridge(ctx, bridge, spec)
		if err != nil {
			results = append(results, domain.BridgeResult{BridgeID: bridge.ID, BridgeName: bridge.Name, Success: false, Error: err.Error(), StatusCode: status})
			partial = true
			continue
		}
		results = append(results, domain.BridgeResult{BridgeID: bridge.ID, BridgeName: bridge.Name, Success: true, StatusCode: status, Data: map[string]any{"resources": resources}})
	}
	return aggregateResults(results), partial, nil
}

func (s *Service) showResource(
	ctx context.Context,
	bridges []domain.Bridge,
	spec resourceSpec,
	target string,
	broadcast bool,
) (any, bool, *domain.AppError) {
	candidates, appErr := s.findCandidates(ctx, bridges, spec, target)
	if appErr != nil {
		return nil, false, appErr
	}
	resolved, resolveErr := resolve.ResolveTarget(target, candidates, broadcast)
	if resolveErr != nil {
		return nil, false, resolveErr
	}

	resultRows := make([]domain.BridgeResult, 0, len(resolved))
	partial := false
	for _, candidate := range resolved {
		bridge, ok := findBridgeByID(bridges, candidate.BridgeID)
		if !ok {
			partial = true
			resultRows = append(resultRows, domain.BridgeResult{BridgeID: candidate.BridgeID, BridgeName: candidate.BridgeName, Success: false, Error: "bridge not available in scope"})
			continue
		}
		item, status, err := s.v2.Get(ctx, bridge, spec.PrimaryType, candidate.ResourceID)
		if err != nil && status == http.StatusNotFound && spec.FallbackType != "" {
			v1ResourceID := firstNonEmpty(candidate.V1ResourceID, candidate.ResourceID)
			v1Item, v1Status, v1Err := s.v1.Get(ctx, bridge, spec.FallbackType, v1ResourceID)
			if v1Err != nil {
				partial = true
				resultRows = append(resultRows, domain.BridgeResult{BridgeID: bridge.ID, BridgeName: bridge.Name, Success: false, Error: v1Err.Error(), StatusCode: v1Status})
				continue
			}
			resultRows = append(resultRows, domain.BridgeResult{BridgeID: bridge.ID, BridgeName: bridge.Name, Success: true, StatusCode: v1Status, Data: map[string]any{"resource": v1Item, "source": "v1"}})
			continue
		}
		if err != nil {
			partial = true
			resultRows = append(resultRows, domain.BridgeResult{BridgeID: bridge.ID, BridgeName: bridge.Name, Success: false, Error: err.Error(), StatusCode: status})
			continue
		}
		resultRows = append(resultRows, domain.BridgeResult{BridgeID: bridge.ID, BridgeName: bridge.Name, Success: true, StatusCode: status, Data: map[string]any{"resource": item, "source": "v2"}})
	}
	return aggregateResults(resultRows), partial, nil
}

func (s *Service) createResource(
	ctx context.Context,
	bridges []domain.Bridge,
	spec resourceSpec,
	input ActionInput,
) (any, bool, *domain.AppError) {
	if len(input.Body) == 0 {
		return nil, false, domain.NewError("BODY_REQUIRED", "create/search requires domain-specific flags to build a payload", domain.ExitUsage)
	}
	results := make([]domain.BridgeResult, 0, len(bridges))
	partial := false
	for _, bridge := range bridges {
		path := fmt.Sprintf("/resource/%s", spec.PrimaryType)
		payload, marshalErr := json.Marshal(input.Body)
		if marshalErr != nil {
			return nil, false, domain.WrapError("PAYLOAD_SERIALIZE", "failed to serialize body", domain.ExitUsage, marshalErr)
		}
		response, status, err := s.v2.Raw(ctx, bridge, http.MethodPost, path, payload)
		if err != nil {
			partial = true
			results = append(results, domain.BridgeResult{BridgeID: bridge.ID, BridgeName: bridge.Name, Success: false, Error: err.Error(), StatusCode: status})
			continue
		}
		var parsed any
		if unmarshalErr := json.Unmarshal(response, &parsed); unmarshalErr != nil {
			parsed = string(response)
		}
		results = append(results, domain.BridgeResult{BridgeID: bridge.ID, BridgeName: bridge.Name, Success: true, StatusCode: status, Data: map[string]any{"response": parsed}})
	}
	return aggregateResults(results), partial, nil
}

func (s *Service) writeResourceAction(
	ctx context.Context,
	cmdCtx domain.CommandContext,
	bridges []domain.Bridge,
	domainName string,
	action string,
	spec resourceSpec,
	input ActionInput,
) (any, bool, *domain.AppError) {
	target := firstNonEmpty(input.ID, input.Name)
	if target == "" {
		return nil, false, domain.NewError("TARGET_REQUIRED", "write action requires --id or --name", domain.ExitUsage)
	}

	candidates, appErr := s.findCandidates(ctx, bridges, spec, target)
	if appErr != nil {
		return nil, false, appErr
	}
	resolved, resolveErr := resolve.ResolveTarget(target, candidates, cmdCtx.Broadcast)
	if resolveErr != nil {
		return nil, false, resolveErr
	}

	results := make([]domain.BridgeResult, 0, len(resolved))
	partial := false
	for _, candidate := range resolved {
		bridge, ok := findBridgeByID(bridges, candidate.BridgeID)
		if !ok {
			partial = true
			results = append(results, domain.BridgeResult{BridgeID: candidate.BridgeID, BridgeName: candidate.BridgeName, Success: false, Error: "bridge not available in scope"})
			continue
		}

		payload, method, path, buildErr := buildActionRequest(ctx, s, bridge, domainName, action, spec, candidate.ResourceID, input)
		if buildErr != nil {
			partial = true
			results = append(results, domain.BridgeResult{BridgeID: bridge.ID, BridgeName: bridge.Name, Success: false, Error: buildErr.Error()})
			continue
		}

		response, status, err := s.v2.Raw(ctx, bridge, method, path, payload)
		if err != nil {
			if status == http.StatusNotFound && spec.FallbackType != "" {
				v1ResourceID := firstNonEmpty(candidate.V1ResourceID, candidate.ResourceID)
				v1Payload := buildV1FallbackPayload(domainName, action, input, payload)
				v1Resp, v1Status, v1Err := s.v1.Write(ctx, bridge, method, spec.FallbackType, v1ResourceID, v1Payload)
				if v1Err != nil {
					partial = true
					results = append(results, domain.BridgeResult{BridgeID: bridge.ID, BridgeName: bridge.Name, Success: false, Error: v1Err.Error(), StatusCode: v1Status})
					continue
				}
				results = append(results, domain.BridgeResult{BridgeID: bridge.ID, BridgeName: bridge.Name, Success: true, StatusCode: v1Status, Data: map[string]any{"response": v1Resp, "source": "v1"}})
				continue
			}

			partial = true
			results = append(results, domain.BridgeResult{BridgeID: bridge.ID, BridgeName: bridge.Name, Success: false, Error: err.Error(), StatusCode: status})
			continue
		}
		var parsed any
		if unmarshalErr := json.Unmarshal(response, &parsed); unmarshalErr != nil {
			parsed = string(response)
		}
		results = append(results, domain.BridgeResult{BridgeID: bridge.ID, BridgeName: bridge.Name, Success: true, StatusCode: status, Data: map[string]any{"response": parsed, "source": "v2"}})
	}
	return aggregateResults(results), partial, nil
}

func (s *Service) resolveBridgeScope(cmdCtx domain.CommandContext) ([]domain.Bridge, *domain.AppError) {
	cfg, appErr := s.store.Load()
	if appErr != nil {
		return nil, appErr
	}
	if len(cfg.Bridges) == 0 {
		return nil, &domain.AppError{
			Code:     "NO_BRIDGES",
			Message:  "no bridges configured",
			ExitCode: domain.ExitUsage,
			Hints:    []string{"Run: huectl bridge discover", "Run: huectl bridge add --name <name> --address <ip>", "Run: huectl bridge link --bridge <id|name>"},
		}
	}

	if strings.TrimSpace(cmdCtx.Bridge) == "" {
		return cfg.Bridges, nil
	}

	filtered := make([]domain.Bridge, 0)
	needle := strings.TrimSpace(cmdCtx.Bridge)
	for _, bridge := range cfg.Bridges {
		if bridge.ID == needle || strings.EqualFold(bridge.Name, needle) {
			filtered = append(filtered, bridge)
		}
	}
	if len(filtered) == 0 {
		return nil, &domain.AppError{Code: "BRIDGE_NOT_FOUND", Message: "no bridge found for the selected scope", ExitCode: domain.ExitTarget, Details: map[string]any{"bridge": needle}}
	}
	return filtered, nil
}

func (s *Service) verifyFingerprints(ctx context.Context, bridges []domain.Bridge) *domain.AppError {
	for _, bridge := range bridges {
		if strings.TrimSpace(bridge.CertFingerprint) == "" {
			return &domain.AppError{
				Code:     "FINGERPRINT_REQUIRED",
				Message:  "bridge certificate fingerprint is required for authenticated operations",
				ExitCode: domain.ExitAuth,
				Details:  map[string]any{"bridge_id": bridge.ID},
				Hints: []string{
					"Run: huectl bridge link --bridge <id|name>",
				},
			}
		}
		current, err := common.FetchCertFingerprint(ctx, bridge.Address, 5*time.Second)
		if err != nil {
			return domain.WrapError("FINGERPRINT_CHECK", "failed to verify bridge certificate fingerprint", domain.ExitNetwork, err)
		}
		if !strings.EqualFold(current, bridge.CertFingerprint) {
			return &domain.AppError{
				Code:     "FINGERPRINT_MISMATCH",
				Message:  "bridge certificate fingerprint changed",
				ExitCode: domain.ExitAuth,
				Details:  map[string]any{"bridge_id": bridge.ID, "expected": bridge.CertFingerprint, "actual": current},
				Hints:    []string{"Verify bridge identity", "Re-link the bridge if certificate rotation is expected"},
			}
		}
	}
	return nil
}

func (s *Service) listForBridge(
	ctx context.Context,
	bridge domain.Bridge,
	spec resourceSpec,
) ([]map[string]any, int, *domain.AppError) {
	results := make([]map[string]any, 0)
	status := http.StatusOK
	for _, v2Type := range spec.ListTypes {
		items, currentStatus, err := s.v2.List(ctx, bridge, v2Type)
		status = currentStatus
		if err != nil {
			if currentStatus == http.StatusNotFound && spec.FallbackType != "" {
				fallback, fallbackStatus, fallbackErr := s.v1.List(ctx, bridge, spec.FallbackType)
				if fallbackErr != nil {
					return nil, fallbackStatus, fallbackErr
				}
				mapped := flattenV1Map(fallback)
				return mapped, fallbackStatus, nil
			}
			return nil, currentStatus, err
		}
		results = append(results, items...)
	}
	return results, status, nil
}

func (s *Service) findCandidates(
	ctx context.Context,
	bridges []domain.Bridge,
	spec resourceSpec,
	target string,
) ([]resolve.Candidate, *domain.AppError) {
	candidates := make([]resolve.Candidate, 0)
	for _, bridge := range bridges {
		resources, _, err := s.listForBridge(ctx, bridge, spec)
		if err != nil {
			return nil, err
		}
		for _, item := range resources {
			v2ID := toString(item["id"])
			v1ID := findV1SyntheticID(item)
			name := extractName(item)
			if v2ID == "" {
				v2ID = v1ID
			}
			if strings.EqualFold(v2ID, target) || strings.EqualFold(v1ID, target) || strings.EqualFold(name, target) {
				candidates = append(candidates, resolve.Candidate{
					BridgeID:     bridge.ID,
					BridgeName:   bridge.Name,
					ResourceID:   v2ID,
					V1ResourceID: firstNonEmpty(v1ID, v2ID),
					ResourceName: name,
					ResourceType: spec.PrimaryType,
				})
			}
		}
	}
	if len(candidates) == 0 {
		return nil, &domain.AppError{Code: "TARGET_NOT_FOUND", Message: "target could not be resolved", ExitCode: domain.ExitTarget, Details: map[string]any{"target": target}}
	}
	return candidates, nil
}

func (s *Service) backupExport(ctx context.Context, cmdCtx domain.CommandContext, file string) (any, bool, *domain.AppError) {
	if strings.TrimSpace(file) == "" {
		return nil, false, domain.NewError("FILE_REQUIRED", "backup export requires --file", domain.ExitUsage)
	}
	cfg, appErr := s.store.Load()
	if appErr != nil {
		return nil, false, appErr
	}
	bridges, appErr := s.resolveBridgeScope(cmdCtx)
	if appErr != nil {
		return nil, false, appErr
	}

	snapshot := map[string]any{
		"schema":      "huectl/backup-v1",
		"created_at":  time.Now().UTC().Format(time.RFC3339),
		"config":      cfg,
		"bridge_data": map[string]any{},
	}

	bridgeData := map[string]any{}
	for _, bridge := range bridges {
		resources := map[string]any{}
		for domainName, spec := range resourceSpecs() {
			items, _, err := s.listForBridge(ctx, bridge, spec)
			if err != nil {
				resources[domainName] = map[string]any{"error": err.Message, "code": err.Code}
				continue
			}
			resources[domainName] = items
		}
		bridgeData[bridge.ID] = resources
	}
	snapshot["bridge_data"] = bridgeData

	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, false, domain.WrapError("BACKUP_SERIALIZE", "failed to serialize backup", domain.ExitInternal, err)
	}
	if writeErr := os.WriteFile(file, payload, 0o600); writeErr != nil {
		return nil, false, domain.WrapError("BACKUP_WRITE", "failed to write backup file", domain.ExitInternal, writeErr)
	}

	return map[string]any{"file": file, "bytes": len(payload), "bridges": len(bridges)}, false, nil
}

func (s *Service) backupImport(file string) (any, bool, *domain.AppError) {
	if strings.TrimSpace(file) == "" {
		return nil, false, domain.NewError("FILE_REQUIRED", "backup import requires --file", domain.ExitUsage)
	}
	payload, err := os.ReadFile(file)
	if err != nil {
		return nil, false, domain.WrapError("BACKUP_READ", "failed to read backup file", domain.ExitInternal, err)
	}
	parsed := map[string]any{}
	if unmarshalErr := json.Unmarshal(payload, &parsed); unmarshalErr != nil {
		return nil, false, domain.WrapError("BACKUP_PARSE", "failed to parse backup file", domain.ExitUsage, unmarshalErr)
	}
	cfgRaw, ok := parsed["config"]
	if !ok {
		return nil, false, domain.NewError("BACKUP_INVALID", "backup does not contain config section", domain.ExitUsage)
	}
	cfgBytes, marshalErr := json.Marshal(cfgRaw)
	if marshalErr != nil {
		return nil, false, domain.WrapError("BACKUP_PARSE", "failed to encode config section", domain.ExitInternal, marshalErr)
	}
	cfg := config.Config{}
	if unmarshalErr := json.Unmarshal(cfgBytes, &cfg); unmarshalErr != nil {
		return nil, false, domain.WrapError("BACKUP_PARSE", "failed to parse config section", domain.ExitUsage, unmarshalErr)
	}
	if saveErr := s.store.Save(cfg); saveErr != nil {
		return nil, false, saveErr
	}
	return map[string]any{"imported": true, "bridges": len(cfg.Bridges), "file": file}, false, nil
}

func (s *Service) backupDiff(ctx context.Context, cmdCtx domain.CommandContext, file string) (any, bool, *domain.AppError) {
	if strings.TrimSpace(file) == "" {
		return nil, false, domain.NewError("FILE_REQUIRED", "backup diff requires --file", domain.ExitUsage)
	}
	payload, err := os.ReadFile(file)
	if err != nil {
		return nil, false, domain.WrapError("BACKUP_READ", "failed to read backup file", domain.ExitInternal, err)
	}
	stored := map[string]any{}
	if unmarshalErr := json.Unmarshal(payload, &stored); unmarshalErr != nil {
		return nil, false, domain.WrapError("BACKUP_PARSE", "failed to parse backup file", domain.ExitUsage, unmarshalErr)
	}
	live, _, appErr := s.backupExport(ctx, cmdCtx, os.DevNull)
	if appErr != nil {
		return nil, false, appErr
	}

	storedKeys := sortedMapKeys(stored)
	liveMap, _ := live.(map[string]any)
	liveKeys := sortedMapKeys(liveMap)
	return map[string]any{"file": file, "stored_keys": storedKeys, "live_keys": liveKeys}, false, nil
}

func (s *Service) collectEvents(
	ctx context.Context,
	bridges []domain.Bridge,
	duration time.Duration,
) (any, bool, *domain.AppError) {
	deadline := time.Now().Add(duration)
	rows := make([]map[string]any, 0)
	for time.Now().Before(deadline) {
		for _, bridge := range bridges {
			items, _, err := s.v2.List(ctx, bridge, "bridge")
			if err != nil {
				rows = append(rows, map[string]any{"bridge_id": bridge.ID, "bridge_name": bridge.Name, "error": err.Error(), "timestamp": time.Now().UTC().Format(time.RFC3339)})
				continue
			}
			rows = append(rows, map[string]any{"bridge_id": bridge.ID, "bridge_name": bridge.Name, "timestamp": time.Now().UTC().Format(time.RFC3339), "bridge_items": len(items)})
		}
		select {
		case <-ctx.Done():
			return map[string]any{"events": rows, "cancelled": true}, true, nil
		case <-time.After(1 * time.Second):
		}
	}
	return map[string]any{"events": rows, "duration": duration.String()}, false, nil
}

func resourceSpecs() map[string]resourceSpec {
	return map[string]resourceSpec{
		"device": {
			PrimaryType:  "device",
			FallbackType: "lights",
			ListTypes:    []string{"device"},
		},
		"light": {
			PrimaryType:  "light",
			FallbackType: "lights",
			ListTypes:    []string{"light"},
		},
		"room": {
			PrimaryType:  "room",
			FallbackType: "groups",
			ListTypes:    []string{"room"},
		},
		"zone": {
			PrimaryType:  "zone",
			FallbackType: "groups",
			ListTypes:    []string{"zone"},
		},
		"scene": {
			PrimaryType:  "scene",
			FallbackType: "scenes",
			ListTypes:    []string{"scene"},
		},
		"automation": {
			PrimaryType:  "behavior_instance",
			FallbackType: "rules",
			ListTypes:    []string{"behavior_instance"},
		},
		"sensor": {
			PrimaryType:  "motion",
			FallbackType: "sensors",
			ListTypes:    []string{"motion", "temperature", "light_level", "button", "contact", "device_power"},
		},
		"entertainment": {
			PrimaryType:  "entertainment_configuration",
			FallbackType: "groups",
			ListTypes:    []string{"entertainment_configuration"},
		},
		"update": {
			PrimaryType:  "device_software_update",
			FallbackType: "config",
			ListTypes:    []string{"device_software_update"},
		},
	}
}

func aggregateResults(results []domain.BridgeResult) domain.AggregateResult {
	summary := map[string]any{
		"bridges_total":   len(results),
		"bridges_success": 0,
		"bridges_failed":  0,
	}
	for _, result := range results {
		if result.Success {
			summary["bridges_success"] = summary["bridges_success"].(int) + 1
		} else {
			summary["bridges_failed"] = summary["bridges_failed"].(int) + 1
		}
	}
	return domain.AggregateResult{Items: results, Summary: summary}
}

func flattenV1Map(raw map[string]any) []map[string]any {
	flattened := make([]map[string]any, 0, len(raw))
	for key, value := range raw {
		entry := map[string]any{"id": key}
		if asMap, ok := value.(map[string]any); ok {
			for k, v := range asMap {
				entry[k] = v
			}
		} else {
			entry["value"] = value
		}
		flattened = append(flattened, entry)
	}
	return flattened
}

func extractName(item map[string]any) string {
	if metadata, ok := item["metadata"].(map[string]any); ok {
		if name, ok := metadata["name"].(string); ok {
			return name
		}
	}
	if name, ok := item["name"].(string); ok {
		return name
	}
	return ""
}

func buildActionRequest(
	ctx context.Context,
	service *Service,
	bridge domain.Bridge,
	domainName string,
	action string,
	spec resourceSpec,
	resourceID string,
	input ActionInput,
) ([]byte, string, string, *domain.AppError) {
	payload := map[string]any{}
	for key, value := range input.Body {
		payload[key] = value
	}

	method := http.MethodPut
	path := fmt.Sprintf("/resource/%s/%s", spec.PrimaryType, resourceID)

	switch domainName {
	case "light":
		switch action {
		case "on":
			payload["on"] = map[string]any{"on": true}
		case "off":
			payload["on"] = map[string]any{"on": false}
		case "toggle":
			current, _, err := service.v2.Get(ctx, bridge, spec.PrimaryType, resourceID)
			if err != nil {
				return nil, "", "", err
			}
			next := true
			if onMap, ok := current["on"].(map[string]any); ok {
				if currentValue, ok := onMap["on"].(bool); ok {
					next = !currentValue
				}
			}
			payload["on"] = map[string]any{"on": next}
		case "effect":
			if _, ok := payload["effects"]; !ok {
				effect := strings.TrimSpace(toString(input.Body["effect"]))
				if input.LightSet != nil {
					effect = firstNonEmpty(strings.TrimSpace(input.LightSet.Effect), effect)
				}
				payload["effects"] = map[string]any{"effect": firstNonEmpty(effect, "prism")}
			}
		case "flash":
			if _, ok := payload["alert"]; !ok {
				alertAction := strings.TrimSpace(toString(input.Body["action"]))
				if input.LightSet != nil {
					alertAction = firstNonEmpty(strings.TrimSpace(input.LightSet.AlertAction), alertAction)
				}
				payload["alert"] = map[string]any{"action": firstNonEmpty(alertAction, "breathe")}
			}
		case "set":
			if input.LightSet != nil {
				applyLightSetPayload(payload, input.LightSet)
			}
			if len(payload) == 0 {
				return nil, "", "", domain.NewError("BODY_REQUIRED", "light set requires explicit flags (for example --on, --brightness, --kelvin)", domain.ExitUsage)
			}
		}
	case "scene":
		if action == "activate" {
			recall := map[string]any{"action": "active"}
			if input.Scene != nil {
				if input.Scene.Dynamic != nil && *input.Scene.Dynamic {
					recall["action"] = "dynamic_palette"
				}
				if input.Scene.DurationMS != nil {
					recall["duration"] = *input.Scene.DurationMS
				}
			}
			payload = map[string]any{"recall": recall}
		}
	case "sensor":
		if action == "sensitivity" && input.Sensor != nil {
			if input.Sensor.Sensitivity != nil {
				payload["sensitivity"] = map[string]any{"sensitivity": *input.Sensor.Sensitivity}
			}
			if input.Sensor.Enabled != nil {
				payload["enabled"] = *input.Sensor.Enabled
			}
		}
	}

	switch action {
	case "delete":
		method = http.MethodDelete
		payload = map[string]any{}
	case "start":
		payload["active"] = true
	case "stop":
		payload["active"] = false
	case "disable":
		payload["enabled"] = false
	case "enable":
		payload["enabled"] = true
	case "rename":
		name := toString(input.Body["name"])
		if name == "" {
			name = input.Name
		}
		if name == "" {
			return nil, "", "", domain.NewError("NAME_REQUIRED", "rename requires --name", domain.ExitUsage)
		}
		payload["metadata"] = map[string]any{"name": name}
	case "run":
		payload["enabled"] = true
	case "assign", "unassign":
		if len(payload) == 0 {
			return nil, "", "", domain.NewError("BODY_REQUIRED", "assign/unassign requires --child-id", domain.ExitUsage)
		}
	case "clone":
		path = fmt.Sprintf("/resource/%s", spec.PrimaryType)
		method = http.MethodPost
		if len(payload) == 0 {
			payload = map[string]any{"clone_from": resourceID}
		}
	}

	if method == http.MethodDelete {
		return nil, method, path, nil
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, "", "", domain.WrapError("PAYLOAD_SERIALIZE", "failed to serialize payload", domain.ExitUsage, err)
	}
	return encoded, method, path, nil
}

func applyLightSetPayload(payload map[string]any, lightSet *LightSetInput) {
	if lightSet.On != nil {
		payload["on"] = map[string]any{"on": *lightSet.On}
	}
	if lightSet.Brightness != nil {
		payload["dimming"] = map[string]any{"brightness": *lightSet.Brightness}
	}
	if lightSet.Kelvin != nil {
		payload["color_temperature"] = map[string]any{"mirek": kelvinToMirek(*lightSet.Kelvin)}
	}
	if lightSet.XYX != nil && lightSet.XYY != nil {
		payload["color"] = map[string]any{"xy": map[string]any{"x": *lightSet.XYX, "y": *lightSet.XYY}}
	}
	if strings.TrimSpace(lightSet.Effect) != "" {
		payload["effects"] = map[string]any{"effect": lightSet.Effect}
	}
	if lightSet.TransitionMS != nil {
		payload["dynamics"] = map[string]any{"duration": *lightSet.TransitionMS}
	}
	if strings.TrimSpace(lightSet.AlertAction) != "" {
		payload["alert"] = map[string]any{"action": lightSet.AlertAction}
	}
}

func kelvinToMirek(kelvin int) int {
	if kelvin <= 0 {
		return 0
	}
	return 1000000 / kelvin
}

func buildV1FallbackPayload(domainName string, action string, input ActionInput, v2Payload []byte) map[string]any {
	payload := map[string]any{}
	for key, value := range input.Body {
		payload[key] = value
	}

	if len(v2Payload) == 0 {
		return payload
	}

	v2Map := map[string]any{}
	if err := json.Unmarshal(v2Payload, &v2Map); err != nil {
		return payload
	}

	switch domainName {
	case "light":
		if onMap, ok := v2Map["on"].(map[string]any); ok {
			if onValue, ok := onMap["on"].(bool); ok {
				payload["on"] = onValue
			}
		}
		if dimmingMap, ok := v2Map["dimming"].(map[string]any); ok {
			if brightness, ok := asFloat64(dimmingMap["brightness"]); ok {
				bri := int(math.Round((brightness / 100.0) * 254.0))
				if bri < 0 {
					bri = 0
				}
				if bri > 254 {
					bri = 254
				}
				payload["bri"] = bri
			}
		}
		if ctMap, ok := v2Map["color_temperature"].(map[string]any); ok {
			if mirek, ok := asInt(ctMap["mirek"]); ok {
				payload["ct"] = mirek
			}
		}
		if colorMap, ok := v2Map["color"].(map[string]any); ok {
			if xyMap, ok := colorMap["xy"].(map[string]any); ok {
				if x, okX := asFloat64(xyMap["x"]); okX {
					if y, okY := asFloat64(xyMap["y"]); okY {
						payload["xy"] = []float64{x, y}
					}
				}
			}
		}
		if effectsMap, ok := v2Map["effects"].(map[string]any); ok {
			if effect, ok := effectsMap["effect"].(string); ok && strings.TrimSpace(effect) != "" {
				payload["effect"] = effect
			}
		}
		if dynamicsMap, ok := v2Map["dynamics"].(map[string]any); ok {
			if durationMS, ok := asInt(dynamicsMap["duration"]); ok {
				transitionTime := durationMS / 100
				if transitionTime < 0 {
					transitionTime = 0
				}
				payload["transitiontime"] = transitionTime
			}
		}
		if alertMap, ok := v2Map["alert"].(map[string]any); ok {
			if actionName, ok := alertMap["action"].(string); ok && strings.TrimSpace(actionName) != "" {
				payload["alert"] = actionName
			}
		}
		if action == "on" || action == "off" || action == "toggle" {
			if _, ok := payload["on"]; !ok {
				payload["on"] = action != "off"
			}
		}
	case "sensor":
		if sensitivityMap, ok := v2Map["sensitivity"].(map[string]any); ok {
			if sensitivity, ok := asInt(sensitivityMap["sensitivity"]); ok {
				payload["sensitivity"] = sensitivity
			}
		}
		if enabled, ok := v2Map["enabled"].(bool); ok {
			payload["enabled"] = enabled
		}
	}

	return payload
}

func asFloat64(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func asInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case float32:
		return int(typed), true
	default:
		return 0, false
	}
}

func encodeRawBody(input ActionInput) ([]byte, *domain.AppError) {
	if input.RawBody != "" {
		return []byte(input.RawBody), nil
	}
	if len(input.Body) == 0 {
		return nil, nil
	}
	payload, err := json.Marshal(input.Body)
	if err != nil {
		return nil, domain.WrapError("PAYLOAD_SERIALIZE", "failed to serialize --body JSON", domain.ExitUsage, err)
	}
	return payload, nil
}

func findBridgeByID(bridges []domain.Bridge, id string) (domain.Bridge, bool) {
	for _, bridge := range bridges {
		if bridge.ID == id {
			return bridge, true
		}
	}
	return domain.Bridge{}, false
}

func findV1SyntheticID(item map[string]any) string {
	if idv1, ok := item["id_v1"].(string); ok {
		parts := strings.Split(strings.Trim(idv1, "/"), "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}
	if id, ok := item["id"].(string); ok {
		return id
	}
	return ""
}

func toString(value any) string {
	if value == nil {
		return ""
	}
	s, ok := value.(string)
	if ok {
		return strings.TrimSpace(s)
	}
	return fmt.Sprintf("%v", value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func sortedMapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
