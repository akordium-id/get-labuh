package compose_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/compose"
)

func TestParseComposeYAML_Valid(t *testing.T) {
	yamlContent := `
version: '3.8'
services:
  web:
    image: nginx:alpine
    ports:
      - "80:80"
      - 443:443
    environment:
      - APP_ENV=production
      - DEBUG=false
    volumes:
      - ./html:/usr/share/nginx/html
    depends_on:
      - api

  api:
    build: ./api
    ports:
      - "8080:8080"
    environment:
      DB_HOST: postgres
      DB_PORT: 5432
    depends_on:
      postgres:
        condition: service_healthy

  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_PASSWORD: secretpassword
    volumes:
      - pgdata:/var/lib/postgresql/data

volumes:
  pgdata:
`

	services, err := compose.ParseComposeYAML(yamlContent)
	require.NoError(t, err)
	require.Len(t, services, 3)

	// Deterministic sorting: api, postgres, web
	assert.Equal(t, "api", services[0].Name)
	assert.Equal(t, "./api", services[0].BuildPath)
	assert.Equal(t, []string{"8080:8080"}, services[0].Ports)
	assert.Equal(t, "postgres", services[0].Environment["DB_HOST"])
	assert.Equal(t, "5432", services[0].Environment["DB_PORT"])
	assert.Contains(t, services[0].DependsOn, "postgres")

	assert.Equal(t, "postgres", services[1].Name)
	assert.Equal(t, "postgres:15-alpine", services[1].Image)
	assert.Equal(t, "secretpassword", services[1].Environment["POSTGRES_PASSWORD"])
	assert.Equal(t, []string{"pgdata:/var/lib/postgresql/data"}, services[1].Volumes)

	assert.Equal(t, "web", services[2].Name)
	assert.Equal(t, "nginx:alpine", services[2].Image)
	assert.Equal(t, []string{"80:80", "443:443"}, services[2].Ports)
	assert.Equal(t, "production", services[2].Environment["APP_ENV"])
	assert.Contains(t, services[2].DependsOn, "api")
}

func TestParseComposeYAML_Invalid(t *testing.T) {
	// Empty content
	_, err := compose.ParseComposeYAML("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kosong")

	// Malformed YAML syntax
	malformed := `
services:
  web:
    image: [unclosed
`
	err = compose.ValidateComposeYAML(malformed)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sintaks YAML tidak valid")

	// Missing services section
	noServices := `
version: '3.8'
networks:
  app-net:
`
	err = compose.ValidateComposeYAML(noServices)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "blok 'services' wajib ada")
}
