package kubedump

var ServiceName = "kubedump-server"
var AppName = "kubedump"
var Namespace = "kubedump"
var Port int32 = 9000

type ResourceKind string

const (
	ResourcePod ResourceKind = "pod"
	ResourceJob              = "job"
)
