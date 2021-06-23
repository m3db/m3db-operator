# set the resources according to your needs. Too less resource will prevent some containers from functioning 
minikube start --cpus 4 --memory 8192 --vm=true
minikube addons enable ingress
# so that we can use `top` if metrics aren't visible in Grafana
minikube addons enable metrics-server
