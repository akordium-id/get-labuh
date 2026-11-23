package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/akordium-id/get-labuh/internal/models"
)

func (s *MCPServer) registerEnvTools() {
	// 9. get_application_env
	s.internalServer.AddTool(
		mcp.NewTool("get_application_env",
			mcp.WithDescription("Mendapatkan variabel lingkungan aplikasi, termasuk status pewarisan (inheritance) dari Project & Team"),
			mcp.WithString("id", mcp.Required(), mcp.Description("ID aplikasi")),
			mcp.WithBoolean("include_inherited", mcp.Description("Sertakan variabel yang diwariskan dari project & team (default true)")),
		),
		s.handleGetApplicationEnv,
	)

	// 10. set_application_env
	s.internalServer.AddTool(
		mcp.NewTool("set_application_env",
			mcp.WithDescription("Menambah atau memperbarui variabel lingkungan spesifik aplikasi"),
			mcp.WithString("id", mcp.Required(), mcp.Description("ID aplikasi")),
			mcp.WithString("key", mcp.Required(), mcp.Description("Nama kunci variabel (contoh: PORT, API_KEY)")),
			mcp.WithString("value", mcp.Required(), mcp.Description("Nilai variabel")),
			mcp.WithBoolean("is_secret", mcp.Description("Tandai sebagai rahasia untuk dienkripsi")),
		),
		s.handleSetApplicationEnv,
	)

	// 11. get_shared_env
	s.internalServer.AddTool(
		mcp.NewTool("get_shared_env",
			mcp.WithDescription("Melihat daftar shared environment variables di tingkat Project atau Team"),
			mcp.WithString("scope", mcp.Required(), mcp.Description("Lingkup: 'project' atau 'team'")),
			mcp.WithString("scope_id", mcp.Required(), mcp.Description("ID project atau ID team")),
		),
		s.handleGetSharedEnv,
	)

	// 12. set_shared_env
	s.internalServer.AddTool(
		mcp.NewTool("set_shared_env",
			mcp.WithDescription("Menambah atau memperbarui shared environment variable di tingkat Project atau Team"),
			mcp.WithString("scope", mcp.Required(), mcp.Description("Lingkup: 'project' atau 'team'")),
			mcp.WithString("scope_id", mcp.Required(), mcp.Description("ID project atau ID team")),
			mcp.WithString("key", mcp.Required(), mcp.Description("Nama kunci variabel")),
			mcp.WithString("value", mcp.Required(), mcp.Description("Nilai variabel")),
			mcp.WithBoolean("is_secret", mcp.Description("Tandai sebagai rahasia untuk dienkripsi")),
		),
		s.handleSetSharedEnv,
	)
}

func (s *MCPServer) handleGetApplicationEnv(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	appID, _ := req.Params.Arguments["id"].(string)
	if appID == "" {
		return mcp.NewToolResultError("Argumen 'id' aplikasi wajib disertakan"), nil
	}

	includeInherited := true
	if inc, ok := req.Params.Arguments["include_inherited"].(bool); ok {
		includeInherited = inc
	}

	if s.deps.AppRepo == nil {
		return mcp.NewToolResultError("Application repository tidak tersedia"), nil
	}

	app, err := s.deps.AppRepo.GetByIDWithContext(ctx, appID)
	if err != nil || app == nil {
		return mcp.NewToolResultError(fmt.Sprintf("Aplikasi dengan ID '%s' tidak ditemukan", appID)), nil
	}

	if includeInherited && s.deps.EnvResolver != nil {
		detailed, err := s.deps.EnvResolver.ResolveDetailed(ctx, app)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Gagal me-resolve environment variables: %v", err)), nil
		}
		b, _ := json.MarshalIndent(detailed, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	}

	if s.deps.AppEnvRepo != nil {
		raw, err := s.deps.AppEnvRepo.GetByApplicationIDWithContext(ctx, appID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Gagal mengambil environment variables aplikasi: %v", err)), nil
		}
		b, _ := json.MarshalIndent(raw, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	}

	return mcp.NewToolResultError("Layanan environment variables tidak tersedia"), nil
}

func (s *MCPServer) handleSetApplicationEnv(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.deps.AppEnvRepo == nil {
		return mcp.NewToolResultError("App environment repository tidak tersedia"), nil
	}

	appID, _ := req.Params.Arguments["id"].(string)
	key, _ := req.Params.Arguments["key"].(string)
	val, _ := req.Params.Arguments["value"].(string)
	isSecret, _ := req.Params.Arguments["is_secret"].(bool)

	if appID == "" || key == "" {
		return mcp.NewToolResultError("Argumen 'id' dan 'key' wajib disertakan"), nil
	}

	_, err := s.deps.AppEnvRepo.CreateWithContext(ctx, models.CreateAppEnvVarInput{
		ApplicationID: appID,
		Key:           key,
		Value:         val,
		IsSecret:      isSecret,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Gagal menyimpan environment variable: %v", err)), nil
	}

	res := map[string]interface{}{
		"status":         "saved",
		"application_id": appID,
		"key":            key,
	}
	b, _ := json.MarshalIndent(res, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}

func (s *MCPServer) handleGetSharedEnv(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.deps.SharedEnvRepo == nil {
		return mcp.NewToolResultError("Shared environment repository tidak tersedia"), nil
	}

	scopeStr, _ := req.Params.Arguments["scope"].(string)
	scopeID, _ := req.Params.Arguments["scope_id"].(string)

	if scopeStr != "project" && scopeStr != "team" {
		return mcp.NewToolResultError("Argumen 'scope' harus bernilai 'project' atau 'team'"), nil
	}
	if scopeID == "" {
		return mcp.NewToolResultError("Argumen 'scope_id' wajib disertakan"), nil
	}

	list, err := s.deps.SharedEnvRepo.ListByScope(ctx, models.SharedEnvVarScope(scopeStr), scopeID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Gagal mengambil shared env: %v", err)), nil
	}

	b, _ := json.MarshalIndent(list, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}

func (s *MCPServer) handleSetSharedEnv(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.deps.SharedEnvRepo == nil {
		return mcp.NewToolResultError("Shared environment repository tidak tersedia"), nil
	}

	scopeStr, _ := req.Params.Arguments["scope"].(string)
	scopeID, _ := req.Params.Arguments["scope_id"].(string)
	key, _ := req.Params.Arguments["key"].(string)
	val, _ := req.Params.Arguments["value"].(string)
	isSecret, _ := req.Params.Arguments["is_secret"].(bool)

	if scopeStr != "project" && scopeStr != "team" {
		return mcp.NewToolResultError("Argumen 'scope' harus bernilai 'project' atau 'team'"), nil
	}
	if scopeID == "" || key == "" {
		return mcp.NewToolResultError("Argumen 'scope_id' dan 'key' wajib disertakan"), nil
	}

	err := s.deps.SharedEnvRepo.Set(ctx, &models.SharedEnvVar{
		Scope:    models.SharedEnvVarScope(scopeStr),
		ScopeID:  scopeID,
		Key:      key,
		Value:    val,
		IsSecret: isSecret,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Gagal menyimpan shared env variable: %v", err)), nil
	}

	res := map[string]interface{}{
		"status":   "saved",
		"scope":    scopeStr,
		"scope_id": scopeID,
		"key":      key,
	}
	b, _ := json.MarshalIndent(res, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}
