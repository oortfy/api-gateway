package etcd_discovery

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type ServiceDiscovery struct {
	client        *clientv3.Client
	leaseID       clientv3.LeaseID
	keepAliveChan <-chan *clientv3.LeaseKeepAliveResponse
	key           string
	val           string
	ctx           context.Context
	cancel        context.CancelFunc

	services     map[string][]string
	lastIndex    map[string]int // used for polling strategy
	lock         sync.RWMutex
	watchChan    clientv3.WatchChan
	watchCancel  context.CancelFunc
	prefix       string
	isRegistered bool
}

// NewServiceDiscovery create a service discovery client
func NewServiceDiscovery(endpoints []string, dialTimeout time.Duration) (*ServiceDiscovery, error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: dialTimeout,
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &ServiceDiscovery{
		client:       client,
		ctx:          ctx,
		cancel:       cancel,
		services:     make(map[string][]string),
		isRegistered: false,
	}, nil
}

// RegisterService registration service
func (s *ServiceDiscovery) RegisterService(prefix, serviceName, serviceAddr string, ttl int64) error {
	if s.isRegistered {
		return errors.New("service already registered")
	}

	// generate key，format: /prefix/serviceName/serviceAddr
	s.prefix = prefix
	s.key = "/" + s.prefix + "/" + serviceName + "/" + serviceAddr
	s.val = serviceAddr

	// create Lease
	resp, err := s.client.Grant(s.ctx, ttl)
	if err != nil {
		return err
	}
	s.leaseID = resp.ID

	// registration service
	_, err = s.client.Put(s.ctx, s.key, s.val, clientv3.WithLease(s.leaseID))
	if err != nil {
		return err
	}

	// maintain heartbeat
	s.keepAliveChan, err = s.client.KeepAlive(s.ctx, s.leaseID)
	if err != nil {
		return err
	}

	s.isRegistered = true

	// monitor the renewal status
	go s.listenLeaseRespChan()

	log.Printf("Service registered: %s, addr: %s\n", serviceName, serviceAddr)
	return nil
}

// listenLeaseRespChan monitor lease renewal status
func (s *ServiceDiscovery) listenLeaseRespChan() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case resp := <-s.keepAliveChan:
			if resp == nil {
				log.Println("Keep alive channel closed")
				return
			}
			log.Printf("Keep alive success, leaseID: %x\n", resp.ID)
		}
	}
}

// DiscoverServices discovery Service
func (s *ServiceDiscovery) DiscoverServices(prefix, serviceName string) ([]string, error) {
	s.prefix = prefix
	key := "/" + s.prefix + "/" + serviceName + "/"
	resp, err := s.client.Get(s.ctx, key, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	addrs := s.extractAddrs(resp)
	s.lock.Lock()
	s.services[serviceName] = addrs
	s.lock.Unlock()

	// todo Store the addrs result in the memory cache, and if addrs is equal to nil, retrieve it from the cache

	return addrs, nil
}

// WatchServices monitor service change
func (s *ServiceDiscovery) WatchServices(serviceName string) {
	key := s.prefix + "/" + serviceName + "/"
	ctx, cancel := context.WithCancel(s.ctx)
	s.watchCancel = cancel
	s.watchChan = s.client.Watch(ctx, key, clientv3.WithPrefix())

	go func() {
		for resp := range s.watchChan {
			for _, ev := range resp.Events {
				switch ev.Type {
				case mvccpb.PUT: // add or modify
					s.handlePutEvent(serviceName, ev.Kv)
				case mvccpb.DELETE: // delete
					s.handleDeleteEvent(serviceName, ev.Kv)
				}
			}
		}
	}()
}

func (s *ServiceDiscovery) handlePutEvent(serviceName string, kv *mvccpb.KeyValue) {
	addr := string(kv.Value)
	s.lock.Lock()
	defer s.lock.Unlock()

	addrs := s.services[serviceName]
	for _, a := range addrs {
		if a == addr {
			return // already present
		}
	}

	s.services[serviceName] = append(addrs, addr)
	log.Printf("Service added: %s, addr: %s\n", serviceName, addr)
}

func (s *ServiceDiscovery) handleDeleteEvent(serviceName string, kv *mvccpb.KeyValue) {
	key := string(kv.Key)
	parts := strings.Split(key, "/")
	if len(parts) < 2 {
		return
	}
	addr := parts[len(parts)-1]

	s.lock.Lock()
	defer s.lock.Unlock()

	addrs := s.services[serviceName]
	for i, a := range addrs {
		if a == addr {
			s.services[serviceName] = append(addrs[:i], addrs[i+1:]...)
			log.Printf("Service removed: %s, addr: %s\n", serviceName, addr)
			break
		}
	}
}

// GetService get service addr list
func (s *ServiceDiscovery) GetService(serviceName string) []string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.services[serviceName]
}

// Close service discovery
func (s *ServiceDiscovery) Close() error {
	// cancel context
	s.cancel()

	// cancel watch
	if s.watchCancel != nil {
		s.watchCancel()
	}

	// log off service
	if s.isRegistered {
		_, err := s.client.Delete(s.ctx, s.key)
		if err != nil {
			return err
		}
	}

	// close etcd client
	return s.client.Close()
}

func (s *ServiceDiscovery) extractAddrs(resp *clientv3.GetResponse) []string {
	addrs := make([]string, 0)
	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		parts := strings.Split(key, "/")
		if len(parts) < 2 {
			continue
		}
		addr := parts[len(parts)-1]
		addrs = append(addrs, addr)
	}
	return addrs
}
