package config

import (
	"fmt"
	"k8s.io/apimachinery/pkg/util/validation"
	"os"
)

type Config struct{ Namespace, Address string }

func Load() (Config, error) {
	c := Config{Namespace: os.Getenv("HSM_NAMESPACE"), Address: os.Getenv("HSM_LISTEN_ADDR")}
	if c.Namespace == "" {
		c.Namespace = "hsm-system"
	}
	if c.Address == "" {
		c.Address = ":8080"
	}
	if len(validation.IsDNS1123Label(c.Namespace)) != 0 {
		return c, fmt.Errorf("invalid HSM_NAMESPACE")
	}
	return c, nil
}
