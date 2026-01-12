package mono

import "fmt"

const (
	BasePort             = 19000
	PortRangePerWorktree = 100
)

type Allocation struct {
	Service       string
	ContainerPort int
	HostPort      int
}

func Allocate(envID int64, servicePorts map[string][]int) []Allocation {
	basePort := BasePort + (int(envID) * PortRangePerWorktree)

	var allocations []Allocation
	usedPorts := make(map[int]bool)
	portIndex := 0

	for service, ports := range servicePorts {
		for _, containerPort := range ports {
			hostPort := basePort + (containerPort % 100)
			for usedPorts[hostPort] {
				hostPort = basePort + portIndex
				portIndex++
			}
			usedPorts[hostPort] = true
			allocations = append(allocations, Allocation{
				Service:       service,
				ContainerPort: containerPort,
				HostPort:      hostPort,
			})
		}
	}

	return allocations
}

func (a Allocation) String() string {
	return fmt.Sprintf("%s:%d -> %d", a.Service, a.ContainerPort, a.HostPort)
}

func AllocationsToMap(allocations []Allocation) map[string]int {
	result := make(map[string]int)
	for _, a := range allocations {
		result[a.Service] = a.HostPort
	}
	return result
}
