package plugin

import "github.com/akordium-id/get-labuh/internal/models"

type Plugin interface {
	Name() string
	Version() string
	Initialize(app *LabuhApp) error
}

type LabuhApp struct {
	Projects  []*models.Project
	Templates []*models.ServiceTemplate
}

type PluginManifest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Entry   string `json:"entry"`
}
