package utils_test

import (
	"crypto/md5"
	"fmt"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"github.com/mightymoud/sidekick/utils"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestHandleEnvFile(t *testing.T) {
	envFileName := "test.env"
	dockerEnvProperty := []string{}
	var envFileChecksum string

	envContent := "KEY1=value1\nKEY2=value2"

	err := os.WriteFile(envFileName, []byte(envContent), 0644)
	assert.NoError(t, err)
	defer os.Remove(envFileName)
	defer os.Remove("encrypted.env")
	envFile, envFileErr := os.Open(fmt.Sprintf("./%s", envFileName))
	assert.NoError(t, envFileErr)

	envMap, envParseErr := godotenv.Parse(envFile)
	assert.NoError(t, envParseErr)
	envFileContent, envMarshalErr := godotenv.Marshal(envMap)
	assert.NoError(t, envMarshalErr)

	viper.Set("publicKey", "age1lgjx644dkpj2nas84pfe4dsd96tph8yxhgf6zfh58kqw06qycavsz00rzm")

	err = utils.HandleEnvFile(envFileName, &dockerEnvProperty, &envFileChecksum)
	assert.NoError(t, err)

	assert.Contains(t, dockerEnvProperty, "KEY2=${KEY2}")
	assert.Contains(t, dockerEnvProperty, "KEY1=${KEY1}")

	expectedChecksum := fmt.Sprintf("%x", md5.Sum([]byte(envFileContent)))
	assert.Equal(t, expectedChecksum, envFileChecksum)
}
func TestLoadAppConfig(t *testing.T) {
	configContent := `
name: test
version: V1
image: ""
url: MOCK_URL
port: 3000
createdAt: Mon Nov 11 21:42:50 KST 2024
`
	err := os.WriteFile("sidekick.yml", []byte(configContent), 0644)
	assert.NoError(t, err)
	defer os.Remove("sidekick.yml")

	appConfig, err := utils.LoadAppConfig()
	assert.NoError(t, err)

	assert.Equal(t, "test", appConfig.Name)
	assert.Equal(t, "V1", appConfig.Version)
	assert.Equal(t, "MOCK_URL", appConfig.Url)
}

func TestLoadAppConfig_FileNotFound(t *testing.T) {
	os.Remove("sidekick.yml")

	_, err := utils.LoadAppConfig()
	assert.Error(t, err)
	assert.Equal(t, "Sidekick app config not found. Please run sidekick launch first", err.Error())
}
