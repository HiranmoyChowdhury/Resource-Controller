package utils

const (
	DefaultReplicaCount = 1
	ContainerName       = "omnitrix"
	DefaultImage        = "hiranmoy36/book-bazar"
	DefaultServiceType  = "NodePort"
	// this port should be in a range of 30000-32767
	DefaultServicePort int32 = 30010
)
