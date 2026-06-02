package ha

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// redisClient is a minimal Redis client implementing only the RESP protocol
// commands needed for HA operations. It does not use an external library,
// keeping the engine dependency-free for Redis while still being production-safe.
type redisClient struct {
	addr    string
	mu      sync.Mutex
	conn    net.Conn
	reader  *bufio.Reader
	timeout time.Duration
}

func newRedisClient(addr string, timeout time.Duration) *redisClient {
	return &redisClient{addr: addr, timeout: timeout}
}

func (c *redisClient) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return nil
	}
	conn, err := net.DialTimeout("tcp", c.addr, c.timeout)
	if err != nil {
		return err
	}
	c.conn = conn
	c.reader = bufio.NewReader(conn)
	return nil
}

func (c *redisClient) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
		c.reader = nil
	}
}

// do sends a RESP command and returns the reply.
func (c *redisClient) do(ctx context.Context, args ...string) (interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		conn, err := net.DialTimeout("tcp", c.addr, c.timeout)
		if err != nil {
			return nil, fmt.Errorf("redis connect %s: %w", c.addr, err)
		}
		c.conn = conn
		c.reader = bufio.NewReader(conn)
	}

	if deadline, ok := ctx.Deadline(); ok {
		c.conn.SetDeadline(deadline)
	} else {
		c.conn.SetDeadline(time.Now().Add(c.timeout))
	}

	// Encode as RESP array.
	var sb strings.Builder
	fmt.Fprintf(&sb, "*%d\r\n", len(args))
	for _, arg := range args {
		fmt.Fprintf(&sb, "$%d\r\n%s\r\n", len(arg), arg)
	}
	if _, err := io.WriteString(c.conn, sb.String()); err != nil {
		c.conn.Close()
		c.conn = nil
		c.reader = nil
		return nil, fmt.Errorf("redis write: %w", err)
	}

	reply, err := c.readReply()
	if err != nil {
		c.conn.Close()
		c.conn = nil
		c.reader = nil
		return nil, err
	}
	return reply, nil
}

func (c *redisClient) readReply() (interface{}, error) {
	line, err := c.reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("redis read: %w", err)
	}
	line = strings.TrimRight(line, "\r\n")
	if len(line) == 0 {
		return nil, fmt.Errorf("redis: empty reply")
	}
	prefix := line[0]
	rest := line[1:]
	switch prefix {
	case '+': // Simple string
		return rest, nil
	case '-': // Error
		return nil, fmt.Errorf("redis error: %s", rest)
	case ':': // Integer
		n, err := strconv.ParseInt(rest, 10, 64)
		return n, err
	case '$': // Bulk string
		n, err := strconv.Atoi(rest)
		if err != nil {
			return nil, fmt.Errorf("redis bulk len: %w", err)
		}
		if n == -1 {
			return nil, nil
		}
		buf := make([]byte, n+2)
		if _, err := io.ReadFull(c.reader, buf); err != nil {
			return nil, fmt.Errorf("redis bulk read: %w", err)
		}
		return string(buf[:n]), nil
	case '*': // Array
		n, err := strconv.Atoi(rest)
		if err != nil {
			return nil, fmt.Errorf("redis array len: %w", err)
		}
		if n == -1 {
			return nil, nil
		}
		arr := make([]interface{}, n)
		for i := range arr {
			arr[i], err = c.readReply()
			if err != nil {
				return nil, err
			}
		}
		return arr, nil
	}
	return nil, fmt.Errorf("redis: unknown reply prefix %q", prefix)
}

func (c *redisClient) ping(ctx context.Context) error {
	reply, err := c.do(ctx, "PING")
	if err != nil {
		return err
	}
	if s, ok := reply.(string); !ok || s != "PONG" {
		return fmt.Errorf("redis: unexpected PING reply %v", reply)
	}
	return nil
}

func (c *redisClient) get(ctx context.Context, key string) (string, error) {
	reply, err := c.do(ctx, "GET", key)
	if err != nil {
		return "", err
	}
	if reply == nil {
		return "", nil
	}
	s, _ := reply.(string)
	return s, nil
}

func (c *redisClient) setex(ctx context.Context, key, value string, ttl time.Duration) error {
	secs := strconv.FormatInt(int64(ttl/time.Second), 10)
	_, err := c.do(ctx, "SETEX", key, secs, value)
	return err
}

func (c *redisClient) del(ctx context.Context, key string) error {
	_, err := c.do(ctx, "DEL", key)
	return err
}

// RedisFailover monitors a set of Redis endpoints and switches the active
// endpoint when the primary becomes unreachable. It implements a simplified
// Sentinel-compatible failover without requiring a Sentinel cluster.
type RedisFailover struct {
	nodeID    string
	endpoints []redisEndpoint

	mu            sync.RWMutex
	activeIdx     int
	activeClient  *redisClient
	failoverCount atomic.Int64

	onFailoverCbs []func(from, to string)

	metricActive    *prometheus.GaugeVec
	metricFailovers prometheus.Counter
	metricLatency   *prometheus.HistogramVec
}

type redisEndpoint struct {
	name    string
	addr    string
	healthy bool
	client  *redisClient
}

// RedisFailoverConfig configures the Redis failover manager.
type RedisFailoverConfig struct {
	NodeID  string
	Addrs   []string // [0] = primary, rest = replicas
	Names   []string // friendly names (same length as Addrs)
	Timeout time.Duration
	Reg     prometheus.Registerer
}

func NewRedisFailover(cfg RedisFailoverConfig) (*RedisFailover, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 2 * time.Second
	}
	if cfg.Reg == nil {
		cfg.Reg = prometheus.DefaultRegisterer
	}
	rf := &RedisFailover{nodeID: cfg.NodeID}

	for i, addr := range cfg.Addrs {
		name := addr
		if i < len(cfg.Names) {
			name = cfg.Names[i]
		}
		client := newRedisClient(addr, cfg.Timeout)
		rf.endpoints = append(rf.endpoints, redisEndpoint{
			name:    name,
			addr:    addr,
			healthy: true,
			client:  client,
		})
	}

	if len(rf.endpoints) == 0 {
		return nil, fmt.Errorf("redis failover: no endpoints configured")
	}

	// Connect to primary.
	if err := rf.endpoints[0].client.connect(); err != nil {
		log.Printf("[ha/redis] WARNING: primary unreachable at startup: %v", err)
		rf.endpoints[0].healthy = false
		// Try replicas.
		for i := 1; i < len(rf.endpoints); i++ {
			if err2 := rf.endpoints[i].client.connect(); err2 == nil {
				rf.activeIdx = i
				rf.activeClient = rf.endpoints[i].client
				log.Printf("[ha/redis] using replica %s as initial active", rf.endpoints[i].name)
				break
			}
		}
	} else {
		rf.activeClient = rf.endpoints[0].client
	}

	f := promauto.With(cfg.Reg)
	labels := prometheus.Labels{"node_id": cfg.NodeID}
	rf.metricActive = f.NewGaugeVec(prometheus.GaugeOpts{
		Name:        "ha_redis_endpoint_active",
		Help:        "1 if this endpoint is the active Redis",
		ConstLabels: labels,
	}, []string{"endpoint"})
	rf.metricFailovers = f.NewCounter(prometheus.CounterOpts{
		Name: "ha_redis_failovers_total", Help: "Redis failover count",
		ConstLabels: labels,
	})
	rf.metricLatency = f.NewHistogramVec(prometheus.HistogramOpts{
		Name:        "ha_redis_ping_latency_seconds",
		Help:        "Redis ping latency",
		ConstLabels: labels,
		Buckets:     prometheus.DefBuckets,
	}, []string{"endpoint"})

	return rf, nil
}

// Client returns the active Redis client.
func (rf *RedisFailover) Client() *redisClient {
	rf.mu.RLock()
	defer rf.mu.RUnlock()
	return rf.activeClient
}

// OnFailover registers a callback invoked after Redis failover.
func (rf *RedisFailover) OnFailover(cb func(from, to string)) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.onFailoverCbs = append(rf.onFailoverCbs, cb)
}

// Run starts the health monitoring loop.
func (rf *RedisFailover) Run(ctx context.Context) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			rf.healthCheck(ctx)
		}
	}
}

func (rf *RedisFailover) healthCheck(ctx context.Context) {
	for i := range rf.endpoints {
		ep := &rf.endpoints[i]
		start := time.Now()
		hctx, cancel := context.WithTimeout(ctx, 1*time.Second)
		err := ep.client.ping(hctx)
		cancel()
		latency := time.Since(start)
		rf.metricLatency.WithLabelValues(ep.name).Observe(latency.Seconds())

		if err != nil {
			if ep.healthy {
				log.Printf("[ha/redis] endpoint %s unhealthy: %v", ep.name, err)
				ep.healthy = false
				ep.client.close()
				rf.metricActive.WithLabelValues(ep.name).Set(0)
				rf.mu.RLock()
				isActive := rf.activeIdx == i
				rf.mu.RUnlock()
				if isActive {
					rf.failover(ctx)
				}
			}
		} else {
			if !ep.healthy {
				log.Printf("[ha/redis] endpoint %s recovered", ep.name)
				ep.healthy = true
				rf.metricActive.WithLabelValues(ep.name).Set(1)
			}
		}
	}
}

func (rf *RedisFailover) failover(ctx context.Context) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	fromName := rf.endpoints[rf.activeIdx].name
	for i, ep := range rf.endpoints {
		if !ep.healthy || i == rf.activeIdx {
			continue
		}
		if err := ep.client.connect(); err != nil {
			continue
		}
		pingCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
		err := ep.client.ping(pingCtx)
		cancel()
		if err != nil {
			continue
		}
		rf.activeIdx = i
		rf.activeClient = ep.client
		rf.metricFailovers.Inc()
		rf.failoverCount.Add(1)
		rf.metricActive.WithLabelValues(ep.name).Set(1)
		log.Printf("[ha/redis] FAILOVER: %s → %s", fromName, ep.name)

		cbs := make([]func(string, string), len(rf.onFailoverCbs))
		copy(cbs, rf.onFailoverCbs)
		go func() {
			for _, cb := range cbs {
				cb(fromName, ep.name)
			}
		}()
		return
	}
	log.Printf("[ha/redis] CRITICAL: no healthy Redis endpoint available")
}

// Get retrieves a key from the active Redis.
func (rf *RedisFailover) Get(ctx context.Context, key string) (string, error) {
	c := rf.Client()
	if c == nil {
		return "", fmt.Errorf("redis: no active client")
	}
	return c.get(ctx, key)
}

// SetEx sets a key with a TTL.
func (rf *RedisFailover) SetEx(ctx context.Context, key, value string, ttl time.Duration) error {
	c := rf.Client()
	if c == nil {
		return fmt.Errorf("redis: no active client")
	}
	return c.setex(ctx, key, value, ttl)
}

// Del deletes a key.
func (rf *RedisFailover) Del(ctx context.Context, key string) error {
	c := rf.Client()
	if c == nil {
		return fmt.Errorf("redis: no active client")
	}
	return c.del(ctx, key)
}
