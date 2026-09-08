package models

import "time"

type TemplateSourceType string

const (
	TemplateSourceGit         TemplateSourceType = "git"
	TemplateSourceDockerImage TemplateSourceType = "docker_image"
	TemplateSourceCompose     TemplateSourceType = "compose"
)

type TemplateCategory string

const (
	TemplateCategoryCMS      TemplateCategory = "cms"
	TemplateCategoryDatabase TemplateCategory = "database"
	TemplateCategoryMonitoring TemplateCategory = "monitoring"
	TemplateCategoryStorage  TemplateCategory = "storage"
	TemplateCategoryOther    TemplateCategory = "other"
)

type ServiceTemplate struct {
	ID            string
	Name          string
	Description   *string
	SourceType    TemplateSourceType
	RepositoryURL *string
	DockerImage   *string
	ComposeYAML   *string
	IconURL       *string
	Category      TemplateCategory
	IsOfficial    bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type CreateServiceTemplateInput struct {
	Name          string
	Description   *string
	SourceType    TemplateSourceType
	RepositoryURL *string
	DockerImage   *string
	ComposeYAML   *string
	IconURL       *string
	Category      TemplateCategory
	IsOfficial    bool
}

type TemplateVariable struct {
	ID          string
	TemplateID  string
	Key         string
	DisplayName string
	Description *string
	Required    bool
	DefaultValue *string
	CreatedAt   time.Time
}

type CreateTemplateVariableInput struct {
	TemplateID   string
	Key          string
	DisplayName  string
	Description  *string
	Required     bool
	DefaultValue *string
}
