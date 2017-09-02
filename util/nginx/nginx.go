package nginx

import (
	"time"
	"net/http"
	"github.com/huxos/kube2dyups/util/config"

	"github.com/golang/glog"
	"bytes"
	"strings"
	"io/ioutil"
	"errors"
)

// Configuration object for constructing Nginx.
type NginxConfig struct {
	ConfigPath   string
	TemplateDir  string
	DumpInterval time.Duration
	DyUpsUrl     string
}

// Nginx represents a real Nginx instance.
type Nginx struct {
	config     NginxConfig
	configurer *config.Configurer
}

func NewInstance(cfg NginxConfig) (*Nginx, error) {
	configurer, err := config.NewConfigurer(cfg.ConfigPath)
	if err != nil {
		return nil, err
	}
	return &Nginx{
		config:     cfg,
		configurer: configurer,
	}, nil
}

func (p *Nginx) DumpConfig(cfgBytes []byte) error {

	start := time.Now()

	err := p.configurer.WriteConfig(cfgBytes)
	if err != nil {
		return err
	}
	defer func() {
		glog.V(4).Infof("dump nginx config file took %v", time.Since(start))
	}()

	return nil
}

func (p *Nginx) DyupsAddOrUpdate(name string, cfgBytes []byte) error {

	start := time.Now()
	timeout := time.Duration(1500 * time.Millisecond)
	client := &http.Client{
		Timeout: timeout,
	}
	updateUrl := strings.Join([]string{
		p.config.DyUpsUrl,
		"/upstream/",
		name,
	}, "")
	resp := &http.Response{}
	for _ = range []int{0, 1} {
		err := errors.New("")
		resp, err = client.Post(updateUrl, "text/plain", bytes.NewReader(cfgBytes))
		if err != nil {
			glog.V(4).Infof("post upstream %s to dyups failed: %v", updateUrl, err)
			return err
		}
		if resp.StatusCode == 200 {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				glog.V(4).Infof("fetch responce data for %s  failed: %v", updateUrl, err)
				return err
			}
			if string(body) == "success" {
				glog.V(4).Infof("update upstream %s successed", name)
			}
			break
		} else if resp.StatusCode == 409 {
			time.Sleep(time.Duration(300 * time.Millisecond))
			glog.V(4).Infof("get a 499 code, update upstream %s failed, try angin ....", name)
			continue
		} else if resp.StatusCode == 500 {
			glog.V(4).Infof("update upstream failed you need to reload nginx to make the Nginx work at a good state.")
			break
		} else {
			glog.V(4).Infof("fetch Unkonwn status code: %d", resp.StatusCode)
			break
		}
	}
	defer func() {
		resp.Body.Close()
		glog.V(4).Infof("post nginx upstream %s to dyups took %v", name, time.Since(start))
	}()

	return nil
}

func (p *Nginx) DyupsDel(name string) error {

	start := time.Now()

	defer func() {
		glog.V(4).Infof("delete nginx upstream %s took %v", name, time.Since(start))
	}()

	return nil
}
