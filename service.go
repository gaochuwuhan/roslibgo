package roslibgo

import (
	"encoding/json"
	"fmt"
)

type Service struct {
	ros          *Ros
	name         string
	serviceType  string
	isAdvertized bool
}

// https://github.com/biobotus/rosbridge_suite/blob/master/ROSBRIDGE_PROTOCOL.md#346-call-service
const ServiceCallOp = "call_service"

type ServiceCall struct {
	Op           string          `json:"op"`
	Id           string          `json:"id,omitempty"`
	Service      string          `json:"service"`
	Args         json.RawMessage `json:"args,omitempty"`
	FragmentSize int             `json:"fragment_size,omitempty"` // not supported
	Compression  string          `json:"compression,omitempty"`   // not supported
}

// https://github.com/biobotus/rosbridge_suite/blob/master/ROSBRIDGE_PROTOCOL.md#347-advertise-service
const ServiceAdvertiseOp = "advertise_service"

type ServiceAdvertise struct {
	Op      string `json:"op"`
	Type    string `json:"type"`
	Service string `json:"service"`
}

// https://github.com/biobotus/rosbridge_suite/blob/master/ROSBRIDGE_PROTOCOL.md#348-unadvertise-service
const ServiceUnadvertiseOp = "unadvertise_service"

type ServiceUnadvertise struct {
	Op      string `json:"op"`
	Service string `json:"service"`
}

// https://github.com/biobotus/rosbridge_suite/blob/master/ROSBRIDGE_PROTOCOL.md#349-service-response
const ServiceResponseOp = "service_response"

type ServiceResponse struct {
	Op      string          `json:"op"`
	Id      string          `json:"id,omitempty"`
	Service string          `json:"service"`
	Values  json.RawMessage `json:"values,omitempty"`
	Result  bool            `json:"result"`
}

type ServiceCallback func(json.RawMessage) (bool, json.RawMessage)

func NewService(ros *Ros, name string, serviceType string) *Service {
	service := Service{ros: ros, name: name, serviceType: serviceType, isAdvertized: false}
	return &service
}

func (service *Service) Call(request json.RawMessage) (json.RawMessage, bool, error) {
	srv, err := service.call(request)
	if err != nil {
		return nil, false, err
	}
	ros := service.ros
	//add id to create/store/retrieve/destroy message
	ros.createMessage(ServiceResponseOp, service.name, srv.Id)
	defer ros.destroyMessage(ServiceResponseOp, service.name, srv.Id)
	ros.message.mutex.Lock()
	ch := ros.message.message[ServiceResponseOp+":"+service.name+srv.Id]
	ros.message.mutex.Unlock()
	v, _ := ros.retrieveMessage(ch)
	return v.(*ServiceResponse).Values, v.(*ServiceResponse).Result, nil
}

func (service *Service) Advertise(callback ServiceCallback) error {
	err := service.advertise()
	if err != nil {
		return err
	}

	go func() {
		ros := service.ros
		//TODO advertise add id(ros.counter())
		ros.createMessage(ServiceCallOp, service.name, "advertise")
		ch := ros.message.message[ServiceResponseOp+":"+service.name]
		defer ros.destroyMessage(ServiceCallOp, service.name, "advertise")

		for {
			srvCall, _ := ros.retrieveMessage(ch)
			result, values := callback(srvCall.(*ServiceCall).Args)
			id := srvCall.(*ServiceCall).Id
			srvResp := ServiceResponse{Op: ServiceResponseOp, Id: id, Service: service.name, Values: values, Result: result}
			err := ros.ws.writeJSON(srvResp)
			if err != nil {
				return // FIXME
			}
		}
	}()
	return nil
}

func (service *Service) Unadvertise() error {
	srv := ServiceAdvertise{Op: ServiceUnadvertiseOp, Service: service.name}
	return service.ros.ws.writeJSON(srv)
}

func (service *Service) call(request json.RawMessage) (ServiceCall, error) {
	ros := service.ros
	id := fmt.Sprintf("ServiceCallOp:%s:%d", service.name, ros.incCounter())
	srv := ServiceCall{Op: ServiceCallOp, Id: id, Service: service.name, Args: request}
	return srv, service.ros.ws.writeJSON(srv)
}

func (service *Service) advertise() error {
	srv := ServiceAdvertise{Op: ServiceAdvertiseOp, Type: service.serviceType, Service: service.name}
	return service.ros.ws.writeJSON(srv)
}
