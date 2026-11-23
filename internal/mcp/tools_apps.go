package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/docker/docker/api/types/container"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/worker"
)

func (s *MCPServer) registerAppTools() {
	// 1. list_applications
	s.internalServer.AddTool(
		mcp.NewTool("list_applications",
			mcp.WithDescription("Daftar seluruh aplikasi yang terdaftar di get-labuh beserta status, domain, git branch, dan port"),
		),
		s.handleListApplications,
	)

	// 2. get_application
	s.internalServer.AddTool(
		mcp.NewTool("get_application",
			mcp.WithDescription("Mendapatkan informasi detail aplikasi berdasarkan Application ID"),
			mcp.WithString("id", mcp.Required(), mcp.Description("ID aplikasi")),
		),
		s.handleGetApplication,
	)

	// 3. deploy_application
	s.internalServer.AddTool(
		mcp.NewTool("deploy_application",
			mcp.WithDescription("Memulai proses build & deployment aplikasi (asynchronous queue). Menghasilkan Deployment ID."),
			mcp.WithString("id", mcp.Required(), mcp.Description("ID aplikasi")),
			mcp.WithBoolean("force_rebuild", mcp.Description("Paksa rebuild tanpa cache docker")),
		),
		s.handleDeployApplication,
	)

	// 4. get_deployment_status
	s.internalServer.AddTool(
		mcp.NewTool("get_deployment_status",
			mcp.WithDescription("Mendapatkan status, langkah aktif (stepper), dan log build deployment"),
			mcp.WithString("deployment_id", mcp.Required(), mcp.Description("ID deployment")),
		),
		s.handleGetDeploymentStatus,
	)

	// 5. get_application_logs
	s.internalServer.AddTool(
		mcp.NewTool("get_application_logs",
			mcp.WithDescription("Mengambil cuplikan log runtime stdout/stderr dari container aplikasi"),
			mcp.WithString("id", mcp.Required(), mcp.Description("ID aplikasi")),
			mcp.WithNumber("tail", mcp.Description("Jumlah baris log terakhir yang diambil (default 100)")),
		),
		s.handleGetApplicationLogs,
	)

	// 6. rollback_application
	s.internalServer.AddTool(
		mcp.NewTool("rollback_application",
			mcp.WithDescription("Me-rollback container aplikasi ke image versi sebelumnya (zero rebuild required)"),
			mcp.WithString("id", mcp.Required(), mcp.Description("ID aplikasi")),
		),
		s.handleRollbackApplication,
	)

	// 7. restart_application
	s.internalServer.AddTool(
		mcp.NewTool("restart_application",
			mcp.WithDescription("Me-restart container aplikasi tanpa rebuild"),
			mcp.WithString("id", mcp.Required(), mcp.Description("ID aplikasi")),
		),
		s.handleRestartApplication,
	)

	// 8. stop_application
	s.internalServer.AddTool(
		mcp.NewTool("stop_application",
			mcp.WithDescription("Menghentikan container aplikasi"),
			mcp.WithString("id", mcp.Required(), mcp.Description("ID aplikasi")),
		),
		s.handleStopApplication,
	)
}

func (s *MCPServer) handleListApplications(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.deps.AppRepo == nil {
		return mcp.NewToolResultError("Application repository tidak tersedia"), nil
	}

	apps, err := s.deps.AppRepo.GetAllWithContext(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Gagal mengambil daftar aplikasi: %v", err)), nil
	}

	type AppSummary struct {
		ID            string           `json:"id"`
		Name          string           `json:"name"`
		Slug          string           `json:"slug"`
		EnvironmentID string           `json:"environment_id"`
		Status        models.AppStatus `json:"status"`
		Domain        string           `json:"domain,omitempty"`
		Port          int              `json:"port"`
		Branch        string           `json:"branch,omitempty"`
		SourceType    string           `json:"source_type"`
	}

	var summaries []AppSummary
	for _, app := range apps {
		domain := ""
		if app.CustomDomain != nil {
			domain = *app.CustomDomain
		}
		branch := ""
		if app.Branch != nil {
			branch = *app.Branch
		}
		summaries = append(summaries, AppSummary{
			ID:            app.ID,
			Name:          app.Name,
			Slug:          app.Slug,
			EnvironmentID: app.EnvironmentID,
			Status:        app.Status,
			Domain:        domain,
			Port:          app.AppPort,
			Branch:        branch,
			SourceType:    string(app.SourceType),
		})
	}

	b, _ := json.MarshalIndent(summaries, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}

func (s *MCPServer) handleGetApplication(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.deps.AppRepo == nil {
		return mcp.NewToolResultError("Application repository tidak tersedia"), nil
	}

	appID, _ := req.Params.Arguments["id"].(string)
	if appID == "" {
		return mcp.NewToolResultError("Argumen 'id' aplikasi wajib disertakan"), nil
	}

	app, err := s.deps.AppRepo.GetByIDWithContext(ctx, appID)
	if err != nil || app == nil {
		return mcp.NewToolResultError(fmt.Sprintf("Aplikasi dengan ID '%s' tidak ditemukan", appID)), nil
	}

	sanitized := *app
	if sanitized.GitToken != nil {
		masked := "••••••••"
		sanitized.GitToken = &masked
	}
	if sanitized.WebhookSecret != nil {
		masked := "••••••••"
		sanitized.WebhookSecret = &masked
	}
	if sanitized.BasicAuthPassword != nil {
		masked := "••••••••"
		sanitized.BasicAuthPassword = &masked
	}

	b, _ := json.MarshalIndent(&sanitized, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}

func (s *MCPServer) handleDeployApplication(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.deps.AppRepo == nil || s.deps.DeployRepo == nil || s.deps.DeployWorker == nil {
		return mcp.NewToolResultError("Layanan deployment tidak lengkap"), nil
	}

	appID, _ := req.Params.Arguments["id"].(string)
	if appID == "" {
		return mcp.NewToolResultError("Argumen 'id' aplikasi wajib disertakan"), nil
	}

	app, err := s.deps.AppRepo.GetByIDWithContext(ctx, appID)
	if err != nil || app == nil {
		return mcp.NewToolResultError(fmt.Sprintf("Aplikasi dengan ID '%s' tidak ditemukan", appID)), nil
	}

	commitMsg := "Triggered via MCP Server"
	dep, err := s.deps.DeployRepo.CreateWithContext(ctx, models.CreateDeploymentInput{
		ApplicationID: appID,
		CommitMessage: &commitMsg,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Gagal membuat record deployment: %v", err)), nil
	}

	job, err := worker.BuildDeploymentJobWithResolver(ctx, app, dep.ID, s.deps.EnvResolver)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Gagal mempersiapkan deployment job: %v", err)), nil
	}

	if err := s.deps.DeployWorker.Enqueue(job); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Gagal memasukkan deployment ke antrean: %v", err)), nil
	}

	res := map[string]interface{}{
		"status":         "queued",
		"deployment_id":  dep.ID,
		"application_id": appID,
		"message":        "Deployment berhasil dimasukkan ke antrean",
	}
	b, _ := json.MarshalIndent(res, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}

func (s *MCPServer) handleGetDeploymentStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.deps.DeployRepo == nil {
		return mcp.NewToolResultError("Deployment repository tidak tersedia"), nil
	}

	depID, _ := req.Params.Arguments["deployment_id"].(string)
	if depID == "" {
		return mcp.NewToolResultError("Argumen 'deployment_id' wajib disertakan"), nil
	}

	dep, err := s.deps.DeployRepo.GetByIDWithContext(ctx, depID)
	if err != nil || dep == nil {
		return mcp.NewToolResultError(fmt.Sprintf("Deployment dengan ID '%s' tidak ditemukan", depID)), nil
	}

	b, _ := json.MarshalIndent(dep, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}

func (s *MCPServer) handleGetApplicationLogs(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.deps.AppRepo == nil {
		return mcp.NewToolResultError("Application repository tidak tersedia"), nil
	}

	appID, _ := req.Params.Arguments["id"].(string)
	if appID == "" {
		return mcp.NewToolResultError("Argumen 'id' aplikasi wajib disertakan"), nil
	}

	app, err := s.deps.AppRepo.GetByIDWithContext(ctx, appID)
	if err != nil || app == nil {
		return mcp.NewToolResultError(fmt.Sprintf("Aplikasi dengan ID '%s' tidak ditemukan", appID)), nil
	}

	if app.ContainerID == nil || *app.ContainerID == "" {
		return mcp.NewToolResultError(fmt.Sprintf("Aplikasi '%s' belum memiliki container aktif", app.Name)), nil
	}

	tailLines := 100
	if t, ok := req.Params.Arguments["tail"].(float64); ok && t > 0 {
		tailLines = int(t)
	}
	if tailLines > 2000 {
		tailLines = 2000
	}

	client := s.getDockerClient(ctx, app.ServerID)
	if client == nil {
		return mcp.NewToolResultError("Docker client tidak tersedia"), nil
	}

	reader, err := client.ContainerLogs(ctx, *app.ContainerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       fmt.Sprintf("%d", tailLines),
		Timestamps: false,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Gagal membaca log container: %v", err)), nil
	}
	defer reader.Close()

	buf := new(bytes.Buffer)
	_, _ = io.Copy(buf, reader)

	return mcp.NewToolResultText(buf.String()), nil
}

func (s *MCPServer) handleRollbackApplication(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.deps.AppRepo == nil || s.deps.DeployRepo == nil || s.deps.DeployWorker == nil {
		return mcp.NewToolResultError("Layanan aplikasi atau deployment tidak lengkap"), nil
	}

	appID, _ := req.Params.Arguments["id"].(string)
	if appID == "" {
		return mcp.NewToolResultError("Argumen 'id' aplikasi wajib disertakan"), nil
	}

	app, err := s.deps.AppRepo.GetByIDWithContext(ctx, appID)
	if err != nil || app == nil {
		return mcp.NewToolResultError(fmt.Sprintf("Aplikasi dengan ID '%s' tidak ditemukan", appID)), nil
	}

	prevDeployment, err := s.deps.DeployRepo.GetPreviousSuccessfulWithContext(ctx, appID)
	if err != nil || prevDeployment == nil {
		return mcp.NewToolResultError(fmt.Sprintf("Tidak ditemukan deployment sukses sebelumnya untuk rollback aplikasi '%s'", app.Name)), nil
	}

	commitMsg := "Rollback to " + prevDeployment.ID[:8]
	if prevDeployment.CommitMessage != nil && *prevDeployment.CommitMessage != "" {
		commitMsg = fmt.Sprintf("Rollback to: %s", *prevDeployment.CommitMessage)
	}

	rollbackDep, err := s.deps.DeployRepo.CreateWithContext(ctx, models.CreateDeploymentInput{
		ApplicationID: appID,
		CommitHash:    prevDeployment.CommitHash,
		CommitMessage: &commitMsg,
		ImageName:     prevDeployment.ImageName,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Gagal membuat deployment rollback: %v", err)), nil
	}

	_ = s.deps.DeployRepo.MarkStartedWithContext(ctx, rollbackDep.ID)
	_ = s.deps.DeployRepo.UpdateStatusWithContext(ctx, rollbackDep.ID, models.DeployStatusDeploying)
	_ = s.deps.AppRepo.UpdateStatus(app.ID, models.AppStatusBuilding)

	job, err := worker.BuildDeploymentJobWithResolver(ctx, app, rollbackDep.ID, s.deps.EnvResolver)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Gagal mempersiapkan job rollback: %v", err)), nil
	}

	if prevDeployment.ImageName != nil && *prevDeployment.ImageName != "" {
		job.DockerImage = *prevDeployment.ImageName
		job.SourceType = string(models.SourceTypeDockerImage)
	}

	if err := s.deps.DeployWorker.Enqueue(job); err != nil {
		_ = s.deps.DeployRepo.UpdateStatusWithContext(ctx, rollbackDep.ID, models.DeployStatusFailed)
		return mcp.NewToolResultError(fmt.Sprintf("Gagal mengantrekan job rollback: %v", err)), nil
	}

	res := map[string]interface{}{
		"status":         "rollback_initiated",
		"application_id": appID,
		"deployment_id":  rollbackDep.ID,
		"target_image":   prevDeployment.ImageName,
		"message":        fmt.Sprintf("Rollback untuk aplikasi '%s' berhasil dimasukkan ke antrean", app.Name),
	}
	b, _ := json.MarshalIndent(res, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}

func (s *MCPServer) handleRestartApplication(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.deps.AppRepo == nil {
		return mcp.NewToolResultError("Application repository tidak tersedia"), nil
	}

	appID, _ := req.Params.Arguments["id"].(string)
	if appID == "" {
		return mcp.NewToolResultError("Argumen 'id' aplikasi wajib disertakan"), nil
	}

	app, err := s.deps.AppRepo.GetByIDWithContext(ctx, appID)
	if err != nil || app == nil {
		return mcp.NewToolResultError(fmt.Sprintf("Aplikasi dengan ID '%s' tidak ditemukan", appID)), nil
	}

	if app.ContainerID == nil || *app.ContainerID == "" {
		return mcp.NewToolResultError(fmt.Sprintf("Aplikasi '%s' tidak memiliki container yang sedang berjalan", app.Name)), nil
	}

	client := s.getDockerClient(ctx, app.ServerID)
	if client == nil {
		return mcp.NewToolResultError("Docker client tidak tersedia"), nil
	}

	if err := client.RestartContainer(ctx, *app.ContainerID, 10); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Gagal me-restart container: %v", err)), nil
	}

	res := map[string]interface{}{
		"status":         "restarted",
		"application_id": appID,
		"container_id":   *app.ContainerID,
		"message":        fmt.Sprintf("Container aplikasi '%s' berhasil di-restart", app.Name),
	}
	b, _ := json.MarshalIndent(res, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}

func (s *MCPServer) handleStopApplication(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.deps.AppRepo == nil {
		return mcp.NewToolResultError("Application repository tidak tersedia"), nil
	}

	appID, _ := req.Params.Arguments["id"].(string)
	if appID == "" {
		return mcp.NewToolResultError("Argumen 'id' aplikasi wajib disertakan"), nil
	}

	app, err := s.deps.AppRepo.GetByIDWithContext(ctx, appID)
	if err != nil || app == nil {
		return mcp.NewToolResultError(fmt.Sprintf("Aplikasi dengan ID '%s' tidak ditemukan", appID)), nil
	}

	if app.ContainerID == nil || *app.ContainerID == "" {
		return mcp.NewToolResultError(fmt.Sprintf("Aplikasi '%s' tidak memiliki container yang sedang berjalan", app.Name)), nil
	}

	client := s.getDockerClient(ctx, app.ServerID)
	if client == nil {
		return mcp.NewToolResultError("Docker client tidak tersedia"), nil
	}

	if err := client.StopContainer(ctx, *app.ContainerID, 10); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Gagal menghentikan container: %v", err)), nil
	}

	_ = s.deps.AppRepo.UpdateStatusWithContext(ctx, appID, models.AppStatusStopped)

	res := map[string]interface{}{
		"status":         "stopped",
		"application_id": appID,
		"container_id":   *app.ContainerID,
		"message":        fmt.Sprintf("Container aplikasi '%s' berhasil dihentikan", app.Name),
	}
	b, _ := json.MarshalIndent(res, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}
