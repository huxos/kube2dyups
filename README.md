#### kube2dyups

监听kubernetes的某个namespaces下的Endpoints实时提交到Tengine的[dyups](http://tengine.taobao.org/document/http_dyups.html)，实现动态负载均衡。

使用go template生成upstream配置，upstream列表支持混合kubernetes内外的服务。

```
CGO_ENABLED=0 go build -o kube-dyups main.go
./kube-dyups --help
Usage of ./kube-dyups:
      --address string                   The IP address to serve on (set to 0.0.0.0 for all interfaces)
      --alsologtostderr                  log to standard error as well as files
      --kube-api-burst int               Burst to use while talking with kubernets apiserver (default 10)
      --kube-api-qps float32             QPS to use while talking with kubernetes apiserver (default 5)
      --kubeconfig string                Path to kubeconfig file with authorization information (the master location is set by the master flag).
      --log-backtrace-at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log-dir string                   If non-empty, write log files in this directory
      --log-flush-frequency duration     Maximum number of seconds between log flushes (default 5s)
      --logtostderr                      log to standard error instead of files (default true)
      --master string                    The address of the kubernetes apiserver
      --namespace string                 What namespace to watch
      --nginx-config-file string         Path of config file for nginx (default "/etc/nginx/conf.d/upstream.conf")
      --nginx-dump-interval duration     Controls how often nginx config file dump (default 5s)
      --nginx-dyups-url string           Nginx Dyups Url (default "http://127.0.0.1:8081")
      --nginx-template-dir string        Path of nginx template
      --port int                         The port that the proxy's http service runs on
      --profiling                        Set true to enable pprof
      --stderrthreshold severity         logs at or above this threshold go to stderr (default 2)
      --sync-period duration             How often configuration from the apiserver is refreshed. Must be greater than 0 (default 30m0s)
  -v, --v Level                          log level for V logs
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging

.....

./kube-dyups \
--kubeconfig=${HOME}/.kube/config  \
--nginx-template-dir=`pwd`/templates  \
--nginx-dyups-url=http://127.0.0.1:8081 \ 
--nginx-config-file=/etc/nginx/conf.d/upstream.conf
--nginx-dump-interval=5s \
--namespace=test \
--sync-period=30m \
-v 4
```
