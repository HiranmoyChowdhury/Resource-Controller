package fake

import (
	real "github.com/HiranmoyChowdhury/ResourceController/pkg/apis/rcs/v1alpha1"
)

var realCopyDB map[string]*real.RanChy
var createdDB bool
var fakeCopyDB map[string]*RanChy

func GenerateFakeCopy(chy *real.RanChy) *RanChy {
	var replica *int32
	if chy.Spec.DeploymentSpec.Replicas != nil {
		replica = chy.Spec.DeploymentSpec.Replicas
	}
	var port *int32
	if chy.Spec.ServiceSpec.Port != nil {
		port = chy.Spec.ServiceSpec.Port
	}
	var nodePort *int32
	if chy.Spec.ServiceSpec.NodePort != nil {
		nodePort = chy.Spec.ServiceSpec.NodePort
	}
	var targetPort *int32

	if chy.Spec.ServiceSpec.TargetPort != nil {
		nodePort = chy.Spec.ServiceSpec.TargetPort
	}
	commands := make([]string, len(chy.Spec.DeploymentSpec.Commands))
	_ = copy(commands, chy.Spec.DeploymentSpec.Commands)
	labels := make(map[string]string)
	for k, v := range chy.Spec.Labels {
		labels[k] = v
	}
	return &RanChy{
		ObjectMeta: ObjectMeta{
			Name:      chy.ObjectMeta.Name,
			Namespace: chy.ObjectMeta.Namespace,
		},
		Status: RanChyStatus{
			AvailableReplicas: chy.Status.AvailableReplicas,
		},
		Spec: RanChySpec{
			DeletionPolicy: DeletionPolicy(chy.Spec.DeletionPolicy),
			DeploymentSpec: DeploymentSpec{
				Name:     chy.Spec.DeploymentSpec.Name,
				Replicas: replica,
				Image:    chy.Spec.DeploymentSpec.Image,
				Commands: commands,
			},
			ServiceSpec: ServiceSpec{
				Name:        chy.Spec.ServiceSpec.Name,
				ServiceType: chy.Spec.ServiceSpec.ServiceType,
				Port:        port,
				NodePort:    nodePort,
				TargetPort:  targetPort,
			},
			Labels: labels,
		},
	}

}

func UpdateRealCopyFromFake(realC *real.RanChy, fakeC *RanChy) {
	var replica *int32
	if fakeC.Spec.DeploymentSpec.Replicas != nil {
		replica = fakeC.Spec.DeploymentSpec.Replicas
	}
	var port *int32
	if fakeC.Spec.ServiceSpec.Port != nil {
		port = fakeC.Spec.ServiceSpec.Port
	}
	var nodePort *int32
	if fakeC.Spec.ServiceSpec.NodePort != nil {
		nodePort = fakeC.Spec.ServiceSpec.NodePort
	}
	var targetPort *int32

	if fakeC.Spec.ServiceSpec.TargetPort != nil {
		nodePort = fakeC.Spec.ServiceSpec.TargetPort
	}
	commands := make([]string, len(fakeC.Spec.DeploymentSpec.Commands))
	_ = copy(commands, fakeC.Spec.DeploymentSpec.Commands)
	labels := make(map[string]string)
	for k, v := range fakeC.Spec.Labels {
		labels[k] = v
	}
	realC.Name = fakeC.Name
	realC.Namespace = fakeC.Namespace
	realC.Status.AvailableReplicas = fakeC.Status.AvailableReplicas
	realC.Spec.DeletionPolicy = real.DeletionPolicy(fakeC.Spec.DeletionPolicy)
	realC.Spec.DeploymentSpec = real.DeploymentSpec{
		Name:     fakeC.Spec.DeploymentSpec.Name,
		Replicas: replica,
		Image:    fakeC.Spec.DeploymentSpec.Image,
		Commands: commands,
	}
	realC.Spec.ServiceSpec = real.ServiceSpec{
		Name:        fakeC.Spec.ServiceSpec.Name,
		ServiceType: fakeC.Spec.ServiceSpec.ServiceType,
		Port:        port,
		NodePort:    nodePort,
		TargetPort:  targetPort,
	}
	realC.Spec.Labels = labels

}
func GetRealCopy(name string) *real.RanChy {
	return realCopyDB[name]
}
func SetRealCopy(obj *real.RanChy) {
	if createdDB == false {
		createDB()
	}
	realCopyDB[obj.Name] = obj
}
func GetFakeCopy(chy *real.RanChy) *RanChy {
	if createdDB == false {
		createDB()
	}
	var key string = string(chy.UID)

	fakeCopy, present := fakeCopyDB[key]

	if present == false {
		fakeCopy = GenerateFakeCopy(chy)
	}
	fakeCopyDB[key] = fakeCopy
	return fakeCopy

}
func createDB() {
	createdDB = true
	realCopyDB = make(map[string]*real.RanChy)
	fakeCopyDB = make(map[string]*RanChy)
}
