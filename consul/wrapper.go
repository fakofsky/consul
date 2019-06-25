package consul

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type Wrapper interface {
	StartMetrics(monitorPort int, servicePromID string) error
	StopMetrics() error
	Register(tags []string, version string) error
	Deregister() error
	SendHealthCheck(err error) error
}

type wrapper struct {
	isUseConsul   bool
	serviceName   string
	serviceID     string
	servicePromID string
	servicePort   int
	monitorPort   int
	consulBroker  Broker
}

func (w *wrapper) StartMetrics(monitorPort int, servicePromID string) error {
	if !w.isUseConsul {
		return nil
	}
	go func() {
		err := startMetricServer(w.serviceName, monitorPort)
		if err != nil {
			log.Fatal(err)
		}
	}()

	w.monitorPort = monitorPort
	w.servicePromID = servicePromID

	promService := Service{
		Name: w.serviceName,
		ID:   servicePromID,
		Port: monitorPort,
		Tags: []string{"prom"},
	}

	err := w.consulBroker.Register(promService)
	if err != nil {
		return fmt.Errorf("can not register service %s in consul %v", promService.ID, err)
	}

	return nil
}

func (w *wrapper) StopMetrics() error {
	if !w.isUseConsul {
		return nil
	}

	err := w.consulBroker.Deregister(w.servicePromID)
	if err != nil {
		return fmt.Errorf("do not deregister consul service %s, got error %v", w.servicePromID, err)
	}

	return nil
}

func (w *wrapper) Register(tags []string, version string) error {
	if !w.isUseConsul {
		return nil
	}

	if version != "" {
		tags = append(tags, version)
	}

	appService := Service{
		Name: w.serviceName,
		ID:   w.serviceID,
		Port: w.servicePort,
		Tags: tags,
		Check: CheckOptions{
			TTL: time.Duration(5 * time.Second).String(),
		},
	}

	err := w.consulBroker.Register(appService)
	if err != nil {
		return fmt.Errorf("do not deregister consul service %s, got error %v", w.serviceID, err)
	}

	return nil
}

func (w *wrapper) Deregister() error {
	if !w.isUseConsul {
		return nil
	}

	err := w.consulBroker.Deregister(w.serviceID)
	if err != nil {
		return fmt.Errorf("do not deregister consul service %s, got error %v", w.serviceID, err)
	}

	return nil
}

func (w *wrapper) SendHealthCheck(err error) error {
	if !w.isUseConsul {
		return nil
	}

	if err != nil {
		if err := w.consulBroker.SendHealthCheck(w.serviceID, err.Error()); err != nil {
			return err
		}
	} else {
		if err := w.consulBroker.SendHealthCheck(w.serviceID, ""); err != nil {
			return err
		}
	}

	return nil
}

func NewWrapper(listen string, consulBroker Broker, serviceName, serviceID string) (Wrapper, error) {
	servicePort, err := getServicePort(listen)
	if err != nil {
		return nil, fmt.Errorf("can't parse service port %s", err.Error())
	}

	return &wrapper{
		isUseConsul:  isUseConsul(),
		serviceName:  serviceName,
		serviceID:    serviceID,
		servicePort:  servicePort,
		consulBroker: consulBroker,
	}, nil
}

func GetBroker() (Broker, error) {
	if !isUseConsul() {
		return nil, nil
	}

	return NewBroker()
}

func startMetricServer(serviceName string, port int) error {
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
		rw.Write([]byte(serviceName + " metrics"))
	})

	addr := "0.0.0.0:" + strconv.Itoa(port)
	log.Println("start prometheus monitoring at", addr)
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		return errors.WithMessage(err, "fail start http prometheus interface")
	}

	return nil
}

func isUseConsul() bool {
	for _, environment := range os.Environ() {
		if strings.Contains(environment, "CONSUL_") {
			return true
		}
	}
	return false
}

func getServicePort(hostPort string) (port int, err error) {
	colon := strings.Split(hostPort, ":")
	if len(colon) < 2 {
		log.Println("fail parse service port. not found ':'")
		return
	}
	port, err = strconv.Atoi(colon[1])
	if err != nil {
		log.Println("fail parse service port", err)
		return
	}
	return
}
