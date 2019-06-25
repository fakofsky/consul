package consul

import (
	"github.com/hashicorp/consul/api"
	"sync"
)

// Broker - represents consul broker interface
type Broker interface {
	Register(serviceData Service) error
	Deregister(serviceID string) error
	SendHealthCheck(serviceID string, error string) error
}

type CheckOptions struct {
	HTTP     string
	Interval string
	TTL      string
}

type Service struct {
	Name  string
	ID    string
	Port  int
	Tags  []string
	Check CheckOptions
}

type broker struct {
	client   *api.Client
	services []*api.AgentServiceRegistration
	sync.Mutex
}

func NewBroker() (Broker, error) {
	consulClient, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return nil, err
	}

	return &broker{
		client:   consulClient,
		services: make([]*api.AgentServiceRegistration, 0),
	}, nil
}

// Register - registers service to consul
func (b *broker) Register(serviceData Service) error {
	serviceRegData := &api.AgentServiceRegistration{
		Name: serviceData.Name,
		ID:   serviceData.ID,
		Port: serviceData.Port,
		Tags: serviceData.Tags,
		Check: &api.AgentServiceCheck{
			HTTP:     serviceData.Check.HTTP,
			Interval: serviceData.Check.Interval,
			TTL:      serviceData.Check.TTL,
		},
	}
	return b.client.Agent().ServiceRegister(serviceRegData)
}

// Deregister - deregisters a service
func (b *broker) Deregister(serviceID string) error {
	return b.client.Agent().ServiceDeregister(serviceID)
}

func (b *broker) SendHealthCheck(serviceID string, error string) error {
	if error == "" {
		if agentErr := b.client.Agent().PassTTL("service:"+serviceID, "ok"); agentErr != nil {
			return agentErr
		}
		return nil
	}

	if agentErr := b.client.Agent().FailTTL("service:"+serviceID, error); agentErr != nil {
		return agentErr
	}

	return nil
}
