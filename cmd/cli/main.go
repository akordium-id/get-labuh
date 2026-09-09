package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	configFile string
	apiKey     string
	baseURL    string
)

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List projects",
	Run: func(cmd *cobra.Command, args []string) {
		data, err := apiRequest(http.MethodGet, "/projects", nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
	},
}

var appsCmd = &cobra.Command{
	Use:   "apps",
	Short: "List applications",
	Run: func(cmd *cobra.Command, args []string) {
		data, err := apiRequest(http.MethodGet, "/projects", nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
	},
}

var deployCmd = &cobra.Command{
	Use:   "deploy <app-id>",
	Short: "Trigger deployment for an application",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := fmt.Sprintf("/applications/%s/deploy", args[0])
		data, err := apiRequest(http.MethodPost, path, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
	},
}

var statusCmd = &cobra.Command{
	Use:   "status <app-id>",
	Short: "Show application status",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := fmt.Sprintf("/applications/%s/status", args[0])
		data, err := apiRequest(http.MethodGet, path, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs <app-id>",
	Short: "Stream application logs",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Streaming logs for %s (not yet implemented)\n", args[0])
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Config file: %s\n", viper.ConfigFileUsed())
		fmt.Printf("Base URL: %s\n", baseURL)
		fmt.Printf("API Key: %s\n", maskString(apiKey))
	},
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "labuh-cli",
		Short: "CLI for Labuh - self-hosted PaaS",
		Long:  "Command line interface for managing Labuh applications, deployments, and projects.",
	}

	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file (default is $HOME/.labuh/config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&apiKey, "api-key", "k", "", "API key for authentication")
	rootCmd.PersistentFlags().StringVarP(&baseURL, "url", "u", "http://localhost:3000", "Labuh API base URL")

	cobra.OnInitialize(initConfig)

	rootCmd.AddCommand(projectsCmd)
	rootCmd.AddCommand(appsCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(configCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func maskString(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return s[:4] + "****"
}

func initConfig() {
	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		configDir := filepath.Join(home, ".labuh")
		viper.AddConfigPath(configDir)
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	_ = viper.ReadInConfig()

	if viper.IsSet("api_key") && apiKey == "" {
		apiKey = viper.GetString("api_key")
	}
	if viper.IsSet("base_url") && baseURL == "" {
		baseURL = viper.GetString("base_url")
	}
}

func apiRequest(method, path string, body any) ([]byte, error) {
	var reqBody []byte
	if body != nil {
		var err error
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, baseURL+"/api/v1"+path, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API error: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}
