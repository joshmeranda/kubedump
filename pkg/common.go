package kubedump

var HelmReleaseName = "kubecudmp-server"
var ServiceName = HelmReleaseName
var Namespace = "kubedump"
var Port int32 = 9000

type ResourceKind string

const (
	ResourcePod ResourceKind = "pod"
	ResourceJob              = "job"
)

const (
	DefaultPodDescriptionInterval = 1.0
	DefaultPodLogInterval         = 1.0
	DefaultJobDescriptionInterval = 1.0
)
