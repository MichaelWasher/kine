package mustgather

import (
	"os"

	"github.com/sirupsen/logrus"
)

var gatherRoot = "/must-gather/"
var bindingFile = "/resource-map.yaml"
var healthEndpoint = "/kubernetes.io/health"

// Returns if the value was set
func setEnvIfValue(value *string, key string) bool {
	tmp := os.Getenv(key)
	if tmp == "" {
		return false
	}
	*value = tmp
	return true
}

func init() {
	// Configure values with ENV Vars
	if setEnvIfValue(&gatherRoot, "KINE_MUSTGATHER_DIR") {
		logrus.Infof("Using environment variable for KINE_MUSTGATHER_DIR")
	}
	if setEnvIfValue(&bindingFile, "KINE_RESOURCE_BINDING") {
		logrus.Infof("Using environment variable for KINE_RESOURCE_BINDING")
	}
	if setEnvIfValue(&healthEndpoint, "KINE_HEALTH_ENDPOINT") {
		logrus.Infof("Using environment variable for KINE_HEALTH_ENDPOINT")
	}

}
