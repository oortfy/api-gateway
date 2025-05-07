package etcd_discovery

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReg(t *testing.T) {
	var endpoints = []string{"127.0.0.1:2379"}
	sd, err := NewServiceDiscovery(endpoints, 5*time.Second)
	if err != nil {
		fmt.Printf("connect etcd error: %v\n", err)
		assert.Error(t, err)
	}
	defer sd.Close()
	// registration service
	prefix := "services"
	serviceName := "gina"
	serviceAddr := "127.0.0.1:8888"
	err = sd.RegisterService(prefix, serviceName, serviceAddr, 10) // 10 min TTL
	if err != nil {
		fmt.Printf("Failed to register service: %v\n", err)
		assert.Error(t, err)
	}

}

func TestDiscovery(t *testing.T) {
	var endpoints = []string{"127.0.0.1:2379"}
	sd, err := NewServiceDiscovery(endpoints, 5*time.Second)
	if err != nil {
		fmt.Printf("connect etcd error: %v\n", err)
		assert.Error(t, err)
	}
	defer sd.Close()
	// discovery service
	prefix := "services"
	serviceName := "gina"
	addrs, err := sd.DiscoverServices(prefix, serviceName)
	if err != nil {
		fmt.Printf("Failed to discover services: %v\n", err)
		assert.Error(t, err)
	} else {
		fmt.Printf("Discovered services: %v\n", addrs)
	}

	// monitoring service change
	// sd.WatchServices(serviceName)
	//
	// go func() {
	// 	ticker := time.NewTicker(3 * time.Second)
	// 	defer ticker.Stop()
	//
	// 	for range ticker.C {
	// 		addrs := sd.GetService(serviceName)
	// 		// sd.GetNextAddr(serviceName, Random)
	// 		fmt.Printf("Current services: %v\n", addrs)
	// 	}
	// }()
}
