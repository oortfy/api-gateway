package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	grpcpool "api-gateway/pkg/grpc"
)

// GRPCProxy handles gRPC proxy operations
type GRPCProxy struct {
	pool        *grpcpool.ClientPool
	methodCache sync.Map // cache for method descriptors
}

// NewGRPCProxy creates a new gRPC proxy instance
func NewGRPCProxy(maxIdle time.Duration, maxConns int) *GRPCProxy {
	return &GRPCProxy{
		pool: grpcpool.NewClientPool(grpcpool.PoolConfig{
			MaxIdle:  maxIdle,
			MaxConns: maxConns,
		}),
	}
}

// ProxyHandler handles HTTP to gRPC conversion
func (p *GRPCProxy) ProxyHandler(c *gin.Context, target string, fullMethodName string) {
	ctx := c.Request.Context()

	// Get connection from pool
	conn, err := p.pool.GetConn(ctx, target)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": fmt.Sprintf("failed to connect to gRPC service: %v", err)})
		return
	}
	defer p.pool.ReleaseConn(target)

	// Get method descriptor from cache or create new
	methodDesc, err := p.getMethodDescriptor(fullMethodName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid gRPC method: %v", err)})
		return
	}

	// Create input/output messages using protoreflect
	inputMsg := dynamicMessage(methodDesc.Input())
	outputMsg := dynamicMessage(methodDesc.Output())

	// Parse request body into input message
	if c.Request.Body != nil {
		decoder := json.NewDecoder(c.Request.Body)
		jsonData := make(map[string]interface{})
		if err := decoder.Decode(&jsonData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		// Convert JSON to proto message
		jsonBytes, _ := json.Marshal(jsonData)
		if err := protojson.Unmarshal(jsonBytes, inputMsg); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse request body"})
			return
		}
	}

	// Prepare metadata
	md := metadata.New(nil)
	for k, v := range c.Request.Header {
		md.Set(strings.ToLower(k), v...)
	}
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Make gRPC call
	if err := conn.Invoke(ctx, fullMethodName, inputMsg, outputMsg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("gRPC call failed: %v", err)})
		return
	}

	// Convert response to JSON
	marshaler := protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: true,
	}
	jsonBytes, err := marshaler.Marshal(outputMsg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal response"})
		return
	}

	c.Data(http.StatusOK, "application/json", jsonBytes)
}

// dynamicMessage creates a new dynamic proto message from a descriptor
func dynamicMessage(desc protoreflect.MessageDescriptor) proto.Message {
	msgType, err := protoregistry.GlobalTypes.FindMessageByName(desc.FullName())
	if err != nil {
		return nil
	}
	return msgType.New().Interface()
}

func (p *GRPCProxy) getMethodDescriptor(fullMethodName string) (protoreflect.MethodDescriptor, error) {
	// Check cache first
	if cached, ok := p.methodCache.Load(fullMethodName); ok {
		return cached.(protoreflect.MethodDescriptor), nil
	}

	// Parse service and method names
	parts := strings.Split(fullMethodName, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid method name format")
	}
	serviceName := parts[0]
	methodName := parts[1]

	// Get service descriptor
	sd, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(serviceName))
	if err != nil {
		return nil, err
	}
	service := sd.(protoreflect.ServiceDescriptor)

	// Get method descriptor
	method := service.Methods().ByName(protoreflect.Name(methodName))
	if method == nil {
		return nil, fmt.Errorf("method %s not found", methodName)
	}

	// Cache the descriptor
	p.methodCache.Store(fullMethodName, method)
	return method, nil
}

// Close closes the gRPC connection pool
func (p *GRPCProxy) Close() {
	p.pool.Close()
}
