package testing

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func CreateSampleNodeApp(tempDir string) (string, error) {
	appDir := filepath.Join(tempDir, "sample-node-app")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create app dir: %w", err)
	}

	packageJSON := `{
  "name": "sample-node-app",
  "version": "1.0.0",
  "scripts": {
    "start": "node server.js"
  },
  "dependencies": {
    "express": "^4.18.2"
  }
}
`
	if err := os.WriteFile(filepath.Join(appDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		return "", fmt.Errorf("failed to write package.json: %w", err)
	}

	serverJS := `const express = require('express');
const app = express();
const PORT = process.env.PORT || 3000;

app.get('/', (req, res) => {
  res.json({ status: 'ok', message: 'Hello from Labuh!' });
});

app.listen(PORT, () => {
  console.log('Sample app listening on port ' + PORT);
});
`
	if err := os.WriteFile(filepath.Join(appDir, "server.js"), []byte(serverJS), 0644); err != nil {
		return "", fmt.Errorf("failed to write server.js: %w", err)
	}

	dockerfile := `FROM node:20-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
EXPOSE 3000
CMD ["node", "server.js"]
`
	if err := os.WriteFile(filepath.Join(appDir, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
		return "", fmt.Errorf("failed to write Dockerfile: %w", err)
	}

	return appDir, nil
}

func CreateSamplePythonApp(tempDir string) (string, error) {
	appDir := filepath.Join(tempDir, "sample-python-app")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create app dir: %w", err)
	}

	requirementsTxt := `flask==3.0.0\n`
	if err := os.WriteFile(filepath.Join(appDir, "requirements.txt"), []byte(requirementsTxt), 0644); err != nil {
		return "", fmt.Errorf("failed to write requirements.txt: %w", err)
	}

	appPy := `from flask import Flask, jsonify
import os

app = Flask(__name__)

@app.route('/')
def health():
    return jsonify({'status': 'ok', 'message': 'Hello from Labuh!'})

if __name__ == '__main__':
    port = int(os.environ.get('PORT', 3000))
    app.run(host='0.0.0.0', port=port)
`
	if err := os.WriteFile(filepath.Join(appDir, "app.py"), []byte(appPy), 0644); err != nil {
		return "", fmt.Errorf("failed to write app.py: %w", err)
	}

	dockerfile := `FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE 3000
CMD ["python", "app.py"]
`
	if err := os.WriteFile(filepath.Join(appDir, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
		return "", fmt.Errorf("failed to write Dockerfile: %w", err)
	}

	return appDir, nil
}

func VerifyHTTPResponse(ctx context.Context, baseURL, path string, expectedStatus int) error {
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reach app: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != expectedStatus {
		return fmt.Errorf("unexpected status: got %d, want %d", resp.StatusCode, expectedStatus)
	}

	return nil
}
