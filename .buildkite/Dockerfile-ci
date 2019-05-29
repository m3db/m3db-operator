FROM golang:1.12-stretch

RUN mkdir /helm && \
  cd /helm && \
  wget -q -O helm.tgz https://storage.googleapis.com/kubernetes-helm/helm-v2.11.0-linux-amd64.tar.gz && \
  tar xzvf helm.tgz && \
  mv linux-amd64/helm /bin && \
  rm -rf /helm
