package app

import (
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/models"
)

// SeedTemplates mengisi katalog service template bawaan jika database masih kosong.
func SeedTemplates(templateRepo *repo.TemplateRepo) {
	templates, _ := templateRepo.GetAll()
	if len(templates) > 0 {
		return
	}

	templatesToSeed := []models.CreateServiceTemplateInput{
		{
			Name:          "WordPress",
			Description:   new("WordPress is a free and open-source content management system."),
			SourceType:    models.TemplateSourceGit,
			RepositoryURL: new("https://github.com/wordpress/wordpress"),
			Category:      models.TemplateCategoryCMS,
			IsOfficial:    true,
		},
		{
			Name:          "Ghost",
			Description:   new("Ghost is a modern, professional publishing platform."),
			SourceType:    models.TemplateSourceGit,
			RepositoryURL: new("https://github.com/TryGhost/Ghost"),
			Category:      models.TemplateCategoryCMS,
			IsOfficial:    true,
		},
		{
			Name:        "Grafana",
			Description: new("Grafana is an open source analytics and interactive visualization web platform."),
			SourceType:  models.TemplateSourceDockerImage,
			DockerImage: new("grafana/grafana"),
			Category:    models.TemplateCategoryMonitoring,
			IsOfficial:  true,
		},
		{
			Name:        "Minio",
			Description: new("MinIO is a high-performance, S3 compatible object store."),
			SourceType:  models.TemplateSourceDockerImage,
			DockerImage: new("minio/minio"),
			Category:    models.TemplateCategoryStorage,
			IsOfficial:  true,
		},
	}

	for _, t := range templatesToSeed {
		_, _ = templateRepo.Create(t)
	}
}
