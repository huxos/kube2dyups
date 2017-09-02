package options

import (
	"time"

	utilnginx "github.com/huxos/kube2dyups/util/nginx"
	"github.com/spf13/pflag"
)

// ProxyServerConfig configures and runs the proxy server.
type ProxyServerConfig struct {
	BindAddress     string
	Address         string
	Port            int
	KubeAPIQPS      float32
	KubeAPIBurst    int
	Master          string
	SyncPeriod      time.Duration
	Kubeconfig      string
	Namespace       string
	EnableProfiling bool
	NginxConfig     utilnginx.NginxConfig
}

func NewProxyServerConfig() *ProxyServerConfig {
	return &ProxyServerConfig{
		KubeAPIQPS:      5.0,
		KubeAPIBurst:    10,
		SyncPeriod:      30 * time.Minute,
		EnableProfiling: false,
		NginxConfig: utilnginx.NginxConfig{
			ConfigPath:     "/etc/nginx/conf.d/upstream.conf",
			DumpInterval: time.Duration(5 * time.Second),
			DyUpsUrl:		"http://127.0.0.1:8081",
		},
	}
}

// AddFlags adds flags for a specific ProxyServer to the specified FlagSet.
func (s *ProxyServerConfig) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.Address, "address", s.Address, "The IP address to serve on (set to 0.0.0.0 for all interfaces)")
	fs.IntVar(&s.Port, "port", s.Port, "The port that the proxy's http service runs on")
	fs.Float32Var(&s.KubeAPIQPS, "kube-api-qps", s.KubeAPIQPS, "QPS to use while talking with kubernetes apiserver")
	fs.IntVar(&s.KubeAPIBurst, "kube-api-burst", s.KubeAPIBurst, "Burst to use while talking with kubernets apiserver")
	fs.StringVar(&s.Master, "master", s.Master, "The address of the kubernetes apiserver")
	fs.StringVar(&s.Namespace, "namespace", s.Namespace, "What namespace to watch")
	fs.DurationVar(&s.SyncPeriod, "sync-period", s.SyncPeriod, "How often configuration from the apiserver is refreshed. Must be greater than 0")
	fs.StringVar(&s.Kubeconfig, "kubeconfig", s.Kubeconfig, "Path to kubeconfig file with authorization information (the master location is set by the master flag).")
	fs.BoolVar(&s.EnableProfiling, "profiling", s.EnableProfiling, "Set true to enable pprof")
	fs.StringVar(&s.NginxConfig.ConfigPath, "nginx-config-file", s.NginxConfig.ConfigPath, "Path of config file for nginx")
	fs.StringVar(&s.NginxConfig.TemplateDir, "nginx-template-dir", s.NginxConfig.TemplateDir, "Path of nginx template")
	fs.StringVar(&s.NginxConfig.DyUpsUrl, "nginx-dyups-url", s.NginxConfig.DyUpsUrl, "Nginx Dyups Url")
	fs.DurationVar(&s.NginxConfig.DumpInterval, "nginx-dump-interval", s.NginxConfig.DumpInterval, "Controls how often nginx config file dump")
}