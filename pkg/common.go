package kubedump

var HelmReleaseName = "kubedump-server"
var ServiceName = HelmReleaseName
var Namespace = "kubedump"
var Port int32 = 9000

type ResourceKind string

const (
	ResourcePod ResourceKind = "Pod"
	ResourceJob              = "Job"
)

const (
	DefaultPodDescriptionInterval = 1.0
	DefaultPodLogInterval         = 1.0
	DefaultJobDescriptionInterval = 1.0
)
