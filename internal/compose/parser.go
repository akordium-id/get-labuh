package compose

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/akordium-id/get-labuh/internal/models"
)

// RawComposeFile merepresentasikan root dokumen Docker Compose.
type RawComposeFile struct {
	Version  string                            `yaml:"version,omitempty"`
	Services map[string]map[string]interface{} `yaml:"services"`
	Networks map[string]interface{}            `yaml:"networks,omitempty"`
	Volumes  map[string]interface{}            `yaml:"volumes,omitempty"`
}

// ParseComposeYAML mengurai string Docker Compose YAML menjadi daftar definisi sub-service.
func ParseComposeYAML(raw string) ([]models.ComposeServiceDef, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("konten docker-compose YAML kosong")
	}

	var root RawComposeFile
	if err := yaml.Unmarshal([]byte(trimmed), &root); err != nil {
		return nil, fmt.Errorf("sintaks YAML tidak valid: %w", err)
	}

	if len(root.Services) == 0 {
		return nil, fmt.Errorf("blok 'services' wajib ada dan minimal memiliki satu service")
	}

	var result []models.ComposeServiceDef
	for name, def := range root.Services {
		svc := models.ComposeServiceDef{
			Name:        name,
			Environment: make(map[string]string),
		}

		// 1. Image
		if img, ok := def["image"].(string); ok {
			svc.Image = img
		}

		// 2. Build
		if b, ok := def["build"].(string); ok {
			svc.BuildPath = b
		} else if bMap, ok := def["build"].(map[string]interface{}); ok {
			if ctx, ok := bMap["context"].(string); ok {
				svc.BuildPath = ctx
			}
		}

		// 3. Ports
		if ports, ok := def["ports"].([]interface{}); ok {
			for _, p := range ports {
				switch v := p.(type) {
				case string:
					svc.Ports = append(svc.Ports, v)
				case int:
					svc.Ports = append(svc.Ports, fmt.Sprintf("%d", v))
				case map[string]interface{}:
					pub := v["published"]
					tgt := v["target"]
					if pub != nil && tgt != nil {
						svc.Ports = append(svc.Ports, fmt.Sprintf("%v:%v", pub, tgt))
					}
				}
			}
		}

		// 4. Environment
		switch env := def["environment"].(type) {
		case []interface{}:
			for _, item := range env {
				if s, ok := item.(string); ok {
					parts := strings.SplitN(s, "=", 2)
					if len(parts) == 2 {
						svc.Environment[parts[0]] = parts[1]
					} else {
						svc.Environment[parts[0]] = ""
					}
				}
			}
		case map[string]interface{}:
			for k, v := range env {
				svc.Environment[k] = fmt.Sprintf("%v", v)
			}
		}

		// 5. Volumes
		if vols, ok := def["volumes"].([]interface{}); ok {
			for _, v := range vols {
				if s, ok := v.(string); ok {
					svc.Volumes = append(svc.Volumes, s)
				}
			}
		}

		// 6. DependsOn
		switch deps := def["depends_on"].(type) {
		case []interface{}:
			for _, d := range deps {
				if s, ok := d.(string); ok {
					svc.DependsOn = append(svc.DependsOn, s)
				}
			}
		case map[string]interface{}:
			for k := range deps {
				svc.DependsOn = append(svc.DependsOn, k)
			}
		}

		result = append(result, svc)
	}

	// Urutkan secara deterministik berdasarkan nama service
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

// ValidateComposeYAML memvalidasi apakah string YAML valid secara sintaks dan struktur dasar Docker Compose.
func ValidateComposeYAML(raw string) error {
	_, err := ParseComposeYAML(raw)
	return err
}
