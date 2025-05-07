package etcd_discovery

import (
	"errors"
	"math/rand"
)

type LoadBalanceStrategy int

const (
	RoundRobin LoadBalanceStrategy = iota
	Random
)

func (s *ServiceDiscovery) GetNextAddr(serviceName string, strategy LoadBalanceStrategy) (string, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	addrs := s.services[serviceName]
	if len(addrs) == 0 {
		return "", errors.New("no available service")
	}

	switch strategy {
	case RoundRobin:
		// 简单轮询实现
		lastIndex := s.lastIndex[serviceName]
		nextIndex := (lastIndex + 1) % len(addrs)
		s.lastIndex[serviceName] = nextIndex
		return addrs[nextIndex], nil
	case Random:
		return addrs[rand.Intn(len(addrs))], nil
	default:
		return addrs[0], nil
	}
}
