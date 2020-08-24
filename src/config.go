package main

import (
	"errors"
	"path/filepath"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type DiscoveryMethod string

func (d *DiscoveryMethod) Decode(value string) error {
	switch value {
	case "dns":
	default:
		return errors.New(`must be "dns"`)
	}
	*d = DiscoveryMethod(value)
	return nil
}

type Config struct {
	HaproxyBinPath        string        `default:"haproxy" split_words:"true"`
	HaproxyConfPath       string        `default:"/etc/haproxy.cfg" split_words:"true"`
	HaproxyAdminSocket    string        `default:"/run/haproxy.sock" split_words:"true"`
	HaproxyEnableLogs     bool          `default:"false" split_words:"true"`
	HaproxyBalance        string        `default:"uri" split_words:"true"`
	HaproxyHttpCheck      string        `default:"meth OPTIONS" split_words:"true"`
	HaproxyBind           string        `default:"0.0.0.0:8888" split_words:"true"`
	HaproxyThreads        int           `default:"1" split_words:"true"`
	HaproxySlots          int           `default:"10" split_words:"true"`
	HaproxyStatsBind      string        `split_words:"true"`
	HaproxyHealthBind     string        `split_words:"true"`
	HaproxyMaxconn        int           `default:"500" split_words:"true"`
	HaproxyTimeoutConnect time.Duration `default:"100ms" split_words:"true"`
	HaproxyTimeoutClient  time.Duration `default:"5s" split_words:"true"`
	HaproxyTimeoutServer  time.Duration `default:"5s" split_words:"true"`
	HaproxyTimeoutCheck   time.Duration `default:"100ms" split_words:"true"`

	DiscoveryMethod        DiscoveryMethod `required:"true" split_words:"true"`
	DiscoveryDNSRefresh    time.Duration   `default:"5s" split_words:"true"`
	DiscoveryDNSName       string          `default:"" split_words:"true"`
	DiscoveryDNSResolvConf string          `default:"/etc/resolv.conf" split_words:"true"`
	DiscoveryDNSUseTCP     bool            `default:"true" split_words:"true"`
	DiscoveryDNSPort       int             `default:"53" split_words:"true"`
}

func getConfig() (*Config, error) {
	var c Config
	if err := envconfig.Process("", &c); err != nil {
		return nil, err
	}
	c.HaproxyConfPath, _ = filepath.Abs(c.HaproxyConfPath)
	c.HaproxyAdminSocket, _ = filepath.Abs(c.HaproxyAdminSocket)
	return &c, nil
}
