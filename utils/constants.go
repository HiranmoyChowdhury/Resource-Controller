package utils

const (
	DefaultReplicaCount = 3
	ContainerName       = "omnitrix"
	DefaultImage        = "nginx:latest"
	DefaultServiceType  = "NodePort"
	// this port should be in a range of 30000-32767
	DefaultServicePort = 30300
)
