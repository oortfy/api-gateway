package proxy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// MockGRPCServer is a mock implementation of a gRPC server for testing
type MockGRPCServer struct {
	mock.Mock
}

func (m *MockGRPCServer) RegisterService(sd *grpc.ServiceDesc, ss interface{}) {
	m.Called(sd, ss)
}

func (m *MockGRPCServer) GracefulStop() {
	m.Called()
}

func (m *MockGRPCServer) Serve(lis interface{}) error {
	args := m.Called(lis)
	return args.Error(0)
}

// On adds a new expected call
func (m *MockGRPCServer) On(methodName string, arguments ...interface{}) *mock.Call {
	return m.Mock.On(methodName, arguments...)
}

// MockGRPCStream is a mock implementation of grpc.ServerStream
type MockGRPCStream struct {
	mock.Mock
	ctx context.Context
}

func (m *MockGRPCStream) SetHeader(md metadata.MD) error {
	args := m.Called(md)
	return args.Error(0)
}

func (m *MockGRPCStream) SendHeader(md metadata.MD) error {
	args := m.Called(md)
	return args.Error(0)
}

func (m *MockGRPCStream) SetTrailer(md metadata.MD) {
	m.Called(md)
}

func (m *MockGRPCStream) Context() context.Context {
	return m.ctx
}

func (m *MockGRPCStream) SendMsg(msg interface{}) error {
	args := m.Called(msg)
	return args.Error(0)
}

func (m *MockGRPCStream) RecvMsg(msg interface{}) error {
	args := m.Called(msg)
	return args.Error(0)
}

// Additional mock types for GRPC message testing
type MockProtoReflect struct {
	protoreflect.Message
}

type MockProtoMessage struct {
	proto.Message
	Data map[string]interface{}
}

func (m *MockProtoMessage) ProtoReflect() protoreflect.Message {
	return &MockProtoReflect{}
}

func TestMockGRPCImplementations(t *testing.T) {
	// Test that our mocks implement the required interfaces
	mockServer := new(MockGRPCServer)
	mockStream := new(MockGRPCStream)
	mockMsg := new(MockProtoMessage)

	// Verify objects are created properly
	assert.NotNil(t, mockServer)
	assert.NotNil(t, mockStream)
	assert.NotNil(t, mockMsg)

	// Setup expectations
	mockServer.On("GracefulStop").Return()
	mockStream.On("SetHeader", metadata.MD{}).Return(nil)

	// Call methods to ensure they're properly implemented
	mockServer.GracefulStop()
	err := mockStream.SetHeader(metadata.MD{})

	// Assert expectations
	mockServer.AssertExpectations(t)
	mockStream.AssertExpectations(t)
	assert.NoError(t, err)
}
