package kubedump

var Namespace = "kubedump"
var Port int32 = 9000

// PodRefreshIntervalEnv is the environment variable describing how many seconds kubedump waits between pod updates.
var PodRefreshIntervalEnv = "POD_DESCRIPTION_INTERVAL"

// PodLogRefreshIntervalEnv is the environment variable describing how many seconds kubedump waits between log updates.
var PodLogRefreshIntervalEnv = "POD_LOG_INTERVAL"
