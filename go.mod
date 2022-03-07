module github.com/m3db/m3db-operator

go 1.16

require (
	github.com/ant31/crd-validation v0.0.0-20180801212718-38f6a293f140
	github.com/apex/log v1.3.0 // indirect
	github.com/axw/gocov v1.0.0
	github.com/bmatcuk/doublestar v1.3.1 // indirect
	github.com/briandowns/spinner v1.11.1 // indirect
	github.com/cheekybits/genny v1.0.0 // indirect
	github.com/cirocosta/grafana-sync v0.0.0-20181123215626-6cbb4a9501c1
	github.com/d4l3k/messagediff v1.2.1
	github.com/emicklei/go-restful v2.9.6+incompatible // indirect
	github.com/evanphx/json-patch v4.11.0+incompatible
	github.com/fossas/fossa-cli v1.0.30
	github.com/garethr/kubeval v0.0.0-20180821130434-c44f5193dc94
	github.com/gnewton/jargo v0.0.0-20150417131352-41f5f186a805 // indirect
	github.com/go-openapi/inflect v0.19.0 // indirect
	github.com/go-swagger/go-swagger v0.19.0
	github.com/go-swagger/scan-repo-boundary v0.0.0-20180623220736-973b3573c013 // indirect
	github.com/gogo/protobuf v1.3.2
	github.com/golang/mock v1.6.0
	github.com/golangci/golangci-lint v1.43.0
	github.com/hashicorp/go-retryablehttp v0.6.0
	github.com/imdario/mergo v0.3.8 // indirect
	github.com/m3db/build-tools v0.0.0-20181013000606-edd1bdd1df8a
	github.com/m3db/m3 v1.4.2
	github.com/m3db/m3x v0.0.0-20190408051622-ebf3c7b94afd // indirect
	github.com/m3db/tools v0.0.0-20181008195521-c6ded3f34878
	github.com/pkg/errors v0.9.1
	github.com/rakyll/statik v0.1.6
	github.com/remeh/sizedwaitgroup v1.0.0 // indirect
	github.com/rhysd/go-github-selfupdate v1.2.2 // indirect
	github.com/rveen/ogdl v0.0.0-20200522080342-eeeda1a978e7 // indirect
	github.com/stretchr/testify v1.7.0
	github.com/toqueteos/webbrowser v1.2.0 // indirect
	github.com/uber-go/tally v3.4.2+incompatible
	github.com/urfave/cli v1.22.2 // indirect
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	go.uber.org/atomic v1.9.0
	go.uber.org/zap v1.19.0
	golang.org/x/lint v0.0.0-20210508222113-6edffad5e616
	gopkg.in/go-ini/ini.v1 v1.57.0 // indirect
	gopkg.in/src-d/go-git.v4 v4.13.1 // indirect
	k8s.io/api v0.22.2
	k8s.io/apiextensions-apiserver v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
	k8s.io/code-generator v0.22.2
	k8s.io/kube-openapi v0.0.0-20211020163157-7327e2aaee2b
	k8s.io/utils v0.0.0-20210819203725-bdf08cb9a70a
)

replace github.com/apache/thrift => github.com/m3db/thrift v0.0.0-20151001171628-53dd39833a08

replace github.com/couchbase/vellum => github.com/m3db/vellum v0.0.0-20180830064305-51c732079c88

// Use a replace until we can upstream fixes.
replace github.com/ant31/crd-validation => github.com/jeromefroe/crd-validation v0.0.0-20211020211201-e17083648306
