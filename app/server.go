package app

import (
	"net"
	"net/http"
	"net/http/pprof"
	"strconv"

	"github.com/huxos/kube2dyups/app/options"
	"github.com/huxos/kube2dyups/proxy/controller"
	"github.com/huxos/kube2dyups/proxy/template"

	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/apimachinery/pkg/util/wait"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
)

type ProxyServer struct {
	Client     *kubeclient.Clientset
	Config     *options.ProxyServerConfig
	Controller *controller.ProxyController
}

func NewProxyServer(
	client *kubeclient.Clientset,
	controller *controller.ProxyController,
	config *options.ProxyServerConfig,
) (*ProxyServer, error) {
	return &ProxyServer{
		Client:     client,
		Config:     config,
		Controller: controller,
	}, nil
}

// NewProxyServerDefault creates a new ProxyServer object with default parameters.
func NewProxyServerDefault(config *options.ProxyServerConfig) (*ProxyServer, error) {
	// Create a Kube Client
	// define api config source
	if config.Kubeconfig == "" && config.Master == "" {
		glog.Warningf("Neither --kubeconfig nor --master was specified. Using default API client. This might not work")
	}
	// This creates a client, first loading any specified kubeconfig
	// file, and then overriding the Master flag, if specified.
	kubeconfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: config.Kubeconfig},
		&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: config.Master}}).ClientConfig()
	if err != nil {
		return nil, err
	}

	// Override kubeconfig qps/burst settings from flags
	kubeconfig.QPS = config.KubeAPIQPS
	kubeconfig.Burst = config.KubeAPIBurst

	client, err := kubeclient.NewForConfig(kubeconfig)
	if err != nil {
		glog.Fatalf("Invalid API configuration: %v", err)
	}

	proxierConfig := template.ProxierConfig{config.NginxConfig}
	proxier, err := template.NewProxier(proxierConfig)
	if err != nil {
		return nil, err
	}

	controller := controller.New(client, proxier, config.SyncPeriod, config.Namespace)
	return NewProxyServer(client, controller, config)
}

// Run runs the specified ProxyServer. This should never exit.
func (s *ProxyServer) Run() error {
	if s.Config.Port > 0 {
		go func() {
			mux := http.NewServeMux()
			healthz.InstallHandler(mux)
			if s.Config.EnableProfiling {
				mux.HandleFunc("/debug/pprof/", pprof.Index)
				mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
				mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
			}
			mux.Handle("/metrics", prometheus.Handler())

			server := &http.Server{
				Addr:    net.JoinHostPort(s.Config.Address, strconv.Itoa(s.Config.Port)),
				Handler: mux,
			}
			glog.Fatal(server.ListenAndServe())
		}()
	}

	s.Controller.Run(wait.NeverStop)
	return nil
}
