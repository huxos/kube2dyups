./kube-dyups \
--kubeconfig=${HOME}/.kube/config  \
--nginx-template-dir=`pwd`/templates  \
--nginx-dyups-url=http://127.0.0.1:8081 \ 
--nginx-config-file=/etc/nginx/conf.d/upstream.conf
--nginx-dump-interval=5s \
--namespace=test \
--sync-period=30m \
-v 4
