package applications

import (
	"bufio"
	"strings"

	"github.com/akordium-id/get-labuh/internal/models"
)

var secretKeywords = []string{
	"SECRET",
	"PASSWORD",
	"PASSWD",
	"TOKEN",
	"KEY",
	"AUTH",
	"PRIVATE",
	"CREDENTIAL",
	"APIKEY",
}

// IsSecretKey menentukan apakah nama key environment terindikasi data sensitif
func IsSecretKey(key string) bool {
	upper := strings.ToUpper(key)
	for _, kw := range secretKeywords {
		if strings.Contains(upper, kw) {
			return true
		}
	}
	return false
}

// ParseEnvContent mem-parsing konten teks .env mentah menjadi slice CreateAppEnvVarInput
func ParseEnvContent(raw string, forceSecret bool) []models.CreateAppEnvVarInput {
	var result []models.CreateAppEnvVarInput
	scanner := bufio.NewScanner(strings.NewReader(raw))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Support "export KEY=VAL"
		if after, ok := strings.CutPrefix(line, "export "); ok {
			line = strings.TrimSpace(after)
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) < 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		if key == "" {
			continue
		}

		val := strings.TrimSpace(parts[1])
		// Unquote jika nilai dibungkus tanda kutip tunggal atau ganda
		if len(val) >= 2 {
			if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
				(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
				val = val[1 : len(val)-1]
			}
		}

		isSecret := forceSecret || IsSecretKey(key)

		result = append(result, models.CreateAppEnvVarInput{
			Key:      key,
			Value:    val,
			IsSecret: isSecret,
		})
	}

	return result
}
