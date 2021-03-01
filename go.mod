module github.com/m3db/m3db-operator

go 1.13

require (
	github.com/ant31/crd-validation v0.0.0-20180801212718-38f6a293f140
	github.com/axw/gocov v1.0.0
	github.com/cirocosta/grafana-sync v0.0.0-20181123215626-6cbb4a9501c1
	github.com/d4l3k/messagediff v1.2.1
	github.com/emicklei/go-restful v2.9.6+incompatible // indirect
	github.com/evanphx/json-patch v4.5.0+incompatible
	github.com/fortytw2/leaktest v1.3.0 // indirect
	github.com/fossas/fossa-cli v1.0.30
	github.com/garethr/kubeval v0.0.0-20180821130434-c44f5193dc94
	github.com/go-openapi/inflect v0.19.0 // indirect
	github.com/go-openapi/runtime v0.19.5 // indirect
	github.com/go-openapi/spec v0.19.3
	github.com/go-swagger/go-swagger v0.19.0
	github.com/go-swagger/scan-repo-boundary v0.0.0-20180623220736-973b3573c013 // indirect
	github.com/gogo/protobuf v1.3.1
	github.com/golang/mock v1.4.4
	github.com/golangci/golangci-lint v1.37.1
	github.com/googleapis/gnostic v0.3.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.6.0
	github.com/m3db/build-tools v0.0.0-20181013000606-edd1bdd1df8a
	github.com/m3db/m3 v0.15.18-0.20201027011129-53414ba8082a
	github.com/m3db/m3x v0.0.0-20190408051622-ebf3c7b94afd
	github.com/m3db/tools v0.0.0-20181008195521-c6ded3f34878
	github.com/pkg/errors v0.9.1
	github.com/rakyll/statik v0.1.6
	github.com/stretchr/testify v1.7.0
	github.com/toqueteos/webbrowser v1.2.0 // indirect
	github.com/uber-go/tally v3.3.13+incompatible
	github.com/urfave/cli v1.22.2 // indirect
	go.uber.org/atomic v1.6.0
	go.uber.org/zap v1.13.0
	golang.org/x/lint v0.0.0-20191125180803-fdd1cda4f05f
	google.golang.org/appengine v1.6.2 // indirect
	k8s.io/api v0.17.3
	k8s.io/apiextensions-apiserver v0.17.2
	k8s.io/apimachinery v0.17.3
	k8s.io/client-go v0.17.3
	k8s.io/code-generator v0.17.2
	k8s.io/gengo v0.0.0-20200413195148-3a45101e95ac // indirect
	k8s.io/kube-openapi v0.0.0-20200410145947-bcb3869e6f29
	k8s.io/utils v0.0.0-20200603063816-c1c6865ac451
	sigs.k8s.io/yaml v1.2.0 // indirect
)

replace github.com/apache/thrift => github.com/m3db/thrift v0.0.0-20151001171628-53dd39833a08

replace github.com/couchbase/vellum => github.com/m3db/vellum v0.0.0-20180830064305-51c732079c88
