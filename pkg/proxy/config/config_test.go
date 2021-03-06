/*
Copyright 2014 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config_test

import (
	"reflect"
	"sort"
	"sync"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	. "github.com/GoogleCloudPlatform/kubernetes/pkg/proxy/config"
)

const TomcatPort int = 8080
const TomcatName = "tomcat"

var TomcatEndpoints = map[string]string{"c0": "1.1.1.1:18080", "c1": "2.2.2.2:18081"}

const MysqlPort int = 3306
const MysqlName = "mysql"

var MysqlEndpoints = map[string]string{"c0": "1.1.1.1:13306", "c3": "2.2.2.2:13306"}

type sortedServices []api.Service

func (s sortedServices) Len() int {
	return len(s)
}
func (s sortedServices) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s sortedServices) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

type ServiceHandlerMock struct {
	services []api.Service
	updated  sync.WaitGroup
}

func NewServiceHandlerMock() *ServiceHandlerMock {
	return &ServiceHandlerMock{services: make([]api.Service, 0)}
}

func (h *ServiceHandlerMock) OnUpdate(services []api.Service) {
	sort.Sort(sortedServices(services))
	h.services = services
	h.updated.Done()
}

func (h *ServiceHandlerMock) ValidateServices(t *testing.T, expectedServices []api.Service) {
	h.updated.Wait()
	if !reflect.DeepEqual(h.services, expectedServices) {
		t.Errorf("Expected %#v, Got %#v", expectedServices, h.services)
	}
}

func (h *ServiceHandlerMock) Wait(waits int) {
	h.updated.Add(waits)
}

type sortedEndpoints []api.Endpoints

func (s sortedEndpoints) Len() int {
	return len(s)
}
func (s sortedEndpoints) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s sortedEndpoints) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

type EndpointsHandlerMock struct {
	endpoints []api.Endpoints
	updated   sync.WaitGroup
}

func NewEndpointsHandlerMock() *EndpointsHandlerMock {
	return &EndpointsHandlerMock{endpoints: make([]api.Endpoints, 0)}
}

func (h *EndpointsHandlerMock) OnUpdate(endpoints []api.Endpoints) {
	sort.Sort(sortedEndpoints(endpoints))
	h.endpoints = endpoints
	h.updated.Done()
}

func (h *EndpointsHandlerMock) ValidateEndpoints(t *testing.T, expectedEndpoints []api.Endpoints) {
	h.updated.Wait()
	if !reflect.DeepEqual(h.endpoints, expectedEndpoints) {
		t.Errorf("Expected %#v, Got %#v", expectedEndpoints, h.endpoints)
	}
}

func (h *EndpointsHandlerMock) Wait(waits int) {
	h.updated.Add(waits)
}

func CreateServiceUpdate(op Operation, services ...api.Service) ServiceUpdate {
	ret := ServiceUpdate{Op: op}
	ret.Services = make([]api.Service, len(services))
	for i, value := range services {
		ret.Services[i] = value
	}
	return ret
}

func CreateEndpointsUpdate(op Operation, endpoints ...api.Endpoints) EndpointsUpdate {
	ret := EndpointsUpdate{Op: op}
	ret.Endpoints = make([]api.Endpoints, len(endpoints))
	for i, value := range endpoints {
		ret.Endpoints[i] = value
	}
	return ret
}

func TestNewServiceAddedAndNotified(t *testing.T) {
	config := NewServiceConfig()
	channel := config.Channel("one")
	handler := NewServiceHandlerMock()
	handler.Wait(1)
	config.RegisterHandler(handler)
	serviceUpdate := CreateServiceUpdate(ADD, api.Service{ObjectMeta: api.ObjectMeta{Name: "foo"}, Spec: api.ServiceSpec{Port: 10}})
	channel <- serviceUpdate
	handler.ValidateServices(t, serviceUpdate.Services)

}

func TestServiceAddedRemovedSetAndNotified(t *testing.T) {
	config := NewServiceConfig()
	channel := config.Channel("one")
	handler := NewServiceHandlerMock()
	config.RegisterHandler(handler)
	serviceUpdate := CreateServiceUpdate(ADD, api.Service{ObjectMeta: api.ObjectMeta{Name: "foo"}, Spec: api.ServiceSpec{Port: 10}})
	handler.Wait(1)
	channel <- serviceUpdate
	handler.ValidateServices(t, serviceUpdate.Services)

	serviceUpdate2 := CreateServiceUpdate(ADD, api.Service{ObjectMeta: api.ObjectMeta{Name: "bar"}, Spec: api.ServiceSpec{Port: 20}})
	handler.Wait(1)
	channel <- serviceUpdate2
	services := []api.Service{serviceUpdate2.Services[0], serviceUpdate.Services[0]}
	handler.ValidateServices(t, services)

	serviceUpdate3 := CreateServiceUpdate(REMOVE, api.Service{ObjectMeta: api.ObjectMeta{Name: "foo"}})
	handler.Wait(1)
	channel <- serviceUpdate3
	services = []api.Service{serviceUpdate2.Services[0]}
	handler.ValidateServices(t, services)

	serviceUpdate4 := CreateServiceUpdate(SET, api.Service{ObjectMeta: api.ObjectMeta{Name: "foobar"}, Spec: api.ServiceSpec{Port: 99}})
	handler.Wait(1)
	channel <- serviceUpdate4
	services = []api.Service{serviceUpdate4.Services[0]}
	handler.ValidateServices(t, services)
}

func TestNewMultipleSourcesServicesAddedAndNotified(t *testing.T) {
	config := NewServiceConfig()
	channelOne := config.Channel("one")
	channelTwo := config.Channel("two")
	if channelOne == channelTwo {
		t.Error("Same channel handed back for one and two")
	}
	handler := NewServiceHandlerMock()
	config.RegisterHandler(handler)
	serviceUpdate1 := CreateServiceUpdate(ADD, api.Service{ObjectMeta: api.ObjectMeta{Name: "foo"}, Spec: api.ServiceSpec{Port: 10}})
	serviceUpdate2 := CreateServiceUpdate(ADD, api.Service{ObjectMeta: api.ObjectMeta{Name: "bar"}, Spec: api.ServiceSpec{Port: 20}})
	handler.Wait(2)
	channelOne <- serviceUpdate1
	channelTwo <- serviceUpdate2
	services := []api.Service{serviceUpdate2.Services[0], serviceUpdate1.Services[0]}
	handler.ValidateServices(t, services)
}

func TestNewMultipleSourcesServicesMultipleHandlersAddedAndNotified(t *testing.T) {
	config := NewServiceConfig()
	channelOne := config.Channel("one")
	channelTwo := config.Channel("two")
	handler := NewServiceHandlerMock()
	handler2 := NewServiceHandlerMock()
	config.RegisterHandler(handler)
	config.RegisterHandler(handler2)
	serviceUpdate1 := CreateServiceUpdate(ADD, api.Service{ObjectMeta: api.ObjectMeta{Name: "foo"}, Spec: api.ServiceSpec{Port: 10}})
	serviceUpdate2 := CreateServiceUpdate(ADD, api.Service{ObjectMeta: api.ObjectMeta{Name: "bar"}, Spec: api.ServiceSpec{Port: 20}})
	handler.Wait(2)
	handler2.Wait(2)
	channelOne <- serviceUpdate1
	channelTwo <- serviceUpdate2
	services := []api.Service{serviceUpdate2.Services[0], serviceUpdate1.Services[0]}
	handler.ValidateServices(t, services)
	handler2.ValidateServices(t, services)
}

func TestNewMultipleSourcesEndpointsMultipleHandlersAddedAndNotified(t *testing.T) {
	config := NewEndpointsConfig()
	channelOne := config.Channel("one")
	channelTwo := config.Channel("two")
	handler := NewEndpointsHandlerMock()
	handler2 := NewEndpointsHandlerMock()
	config.RegisterHandler(handler)
	config.RegisterHandler(handler2)
	endpointsUpdate1 := CreateEndpointsUpdate(ADD, api.Endpoints{
		ObjectMeta: api.ObjectMeta{Name: "foo"},
		Endpoints:  []string{"endpoint1", "endpoint2"},
	})
	endpointsUpdate2 := CreateEndpointsUpdate(ADD, api.Endpoints{
		ObjectMeta: api.ObjectMeta{Name: "bar"},
		Endpoints:  []string{"endpoint3", "endpoint4"},
	})
	handler.Wait(2)
	handler2.Wait(2)
	channelOne <- endpointsUpdate1
	channelTwo <- endpointsUpdate2

	endpoints := []api.Endpoints{endpointsUpdate2.Endpoints[0], endpointsUpdate1.Endpoints[0]}
	handler.ValidateEndpoints(t, endpoints)
	handler2.ValidateEndpoints(t, endpoints)
}

func TestNewMultipleSourcesEndpointsMultipleHandlersAddRemoveSetAndNotified(t *testing.T) {
	config := NewEndpointsConfig()
	channelOne := config.Channel("one")
	channelTwo := config.Channel("two")
	handler := NewEndpointsHandlerMock()
	handler2 := NewEndpointsHandlerMock()
	config.RegisterHandler(handler)
	config.RegisterHandler(handler2)
	endpointsUpdate1 := CreateEndpointsUpdate(ADD, api.Endpoints{
		ObjectMeta: api.ObjectMeta{Name: "foo"},
		Endpoints:  []string{"endpoint1", "endpoint2"},
	})
	endpointsUpdate2 := CreateEndpointsUpdate(ADD, api.Endpoints{
		ObjectMeta: api.ObjectMeta{Name: "bar"},
		Endpoints:  []string{"endpoint3", "endpoint4"},
	})
	handler.Wait(2)
	handler2.Wait(2)
	channelOne <- endpointsUpdate1
	channelTwo <- endpointsUpdate2

	endpoints := []api.Endpoints{endpointsUpdate2.Endpoints[0], endpointsUpdate1.Endpoints[0]}
	handler.ValidateEndpoints(t, endpoints)
	handler2.ValidateEndpoints(t, endpoints)

	// Add one more
	endpointsUpdate3 := CreateEndpointsUpdate(ADD, api.Endpoints{
		ObjectMeta: api.ObjectMeta{Name: "foobar"},
		Endpoints:  []string{"endpoint5", "endpoint6"},
	})
	handler.Wait(1)
	handler2.Wait(1)
	channelTwo <- endpointsUpdate3
	endpoints = []api.Endpoints{endpointsUpdate2.Endpoints[0], endpointsUpdate1.Endpoints[0], endpointsUpdate3.Endpoints[0]}
	handler.ValidateEndpoints(t, endpoints)
	handler2.ValidateEndpoints(t, endpoints)

	// Update the "foo" service with new endpoints
	endpointsUpdate1 = CreateEndpointsUpdate(ADD, api.Endpoints{
		ObjectMeta: api.ObjectMeta{Name: "foo"},
		Endpoints:  []string{"endpoint77"},
	})
	handler.Wait(1)
	handler2.Wait(1)
	channelOne <- endpointsUpdate1
	endpoints = []api.Endpoints{endpointsUpdate2.Endpoints[0], endpointsUpdate1.Endpoints[0], endpointsUpdate3.Endpoints[0]}
	handler.ValidateEndpoints(t, endpoints)
	handler2.ValidateEndpoints(t, endpoints)

	// Remove "bar" service
	endpointsUpdate2 = CreateEndpointsUpdate(REMOVE, api.Endpoints{ObjectMeta: api.ObjectMeta{Name: "bar"}})
	handler.Wait(1)
	handler2.Wait(1)
	channelTwo <- endpointsUpdate2

	endpoints = []api.Endpoints{endpointsUpdate1.Endpoints[0], endpointsUpdate3.Endpoints[0]}
	handler.ValidateEndpoints(t, endpoints)
	handler2.ValidateEndpoints(t, endpoints)
}
