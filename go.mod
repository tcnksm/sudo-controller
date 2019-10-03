module github.com/tcnksm/sudo-controller

go 1.12

require (
	github.com/go-logr/logr v0.1.0
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/nlopes/slack v0.6.0
	github.com/onsi/ginkgo v1.6.0
	github.com/onsi/gomega v1.4.2
	k8s.io/api v0.0.0-20190409021203-6e4e0e4f393b
	k8s.io/apimachinery v0.0.0-20190404173353-6a84e37a896d
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	sigs.k8s.io/controller-runtime v0.2.2
	upspin.io v0.0.0-20190429152748-8fa1bb72e8fd
)
