# go-redis包中的ClusterClient
## 结构
```go
package redis
//
//执行redis命令的即是这个结构体实例，如：val, err := rdb.Get(ctx, "key").Result()
type ClusterClient struct {
	*clusterClient
	cmdable
	hooks
	ctx context.Context
}

//cmdable 实现了Get Set等redis命令
type cmdable func(ctx context.Context, cmd Cmder) error

//比如Get命令
func (c cmdable) Get(ctx context.Context, key string) *StringCmd {
	cmd := NewStringCmd(ctx, "get", key)
	_ = c(ctx, cmd)
	return cmd
}

type clusterClient struct {
	opt           *ClusterOptions
	nodes         *clusterNodes
	state         *clusterStateHolder //nolint:structcheck
	cmdsInfoCache *cmdsInfoCache      //nolint:structcheck
}

//配置比较多，只关注有关的部分
type ClusterOptions struct {
	// A seed list of host:port addresses of cluster nodes.
	Addrs []string
	MaxRetries      int
	MinRetryBackoff time.Duration
	MaxRetryBackoff time.Duration
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	// PoolSize applies per cluster node and not for the whole cluster.
	PoolSize           int
}

type clusterNodes struct {
	opt *ClusterOptions
	mu          sync.RWMutex
	addrs       []string
    //节点集合，以addr为key
	nodes       map[string]*clusterNode
	activeAddrs []string
	closed      bool
	_generation uint32 // atomic
}

type clusterNode struct {
	Client *Client
	latency    uint32 // atomic
	generation uint32 // atomic
	failing    uint32 // atomic
}

type Client struct {
	*baseClient
	cmdable
	hooks
	ctx context.Context
}

//load在初始化时会被赋值为ClusterClient.loadState,后面会看到
//state存的是 *clusterState
type clusterStateHolder struct {
	load func(ctx context.Context) (*clusterState, error)

	state     atomic.Value
	reloading uint32 // atomic
}

type clusterState struct {
	nodes   *clusterNodes
	Masters []*clusterNode
	Slaves  []*clusterNode

	slots []*clusterSlot

	generation uint32
	createdAt  time.Time
}

//clusterSlot表示一个范围，start end分别表示一段连续的slot的起止位置
type clusterSlot struct {
	start, end int
	nodes      []*clusterNode
}

//初始化话ClusterClient
func NewClusterClient(opt *ClusterOptions) *ClusterClient {
	opt.init()

	c := &ClusterClient{
		clusterClient: &clusterClient{
			opt:   opt,
			nodes: newClusterNodes(opt),
		},
		ctx: context.Background(),
	}
	c.state = newClusterStateHolder(c.loadState)
	c.cmdable = c.Process
    //……
	return c
}

//ClusterClient的state是一个clusterStateHolder实例，其load函数即是ClusterClient的loadState方法
//之所以这么绕一圈，可能是为了让clusterStateHolder作为一个通用的结构体，不仅用于ClusterClient，尽管目前还没有别的地方使用
func newClusterStateHolder(fn func(ctx context.Context) (*clusterState, error)) *clusterStateHolder {
	return &clusterStateHolder{
		load: fn,
	}
}

//c.cmdable方法本身是执行命令的核心部分，各种redis命令和参数都是组装成cmd结构体，传给cmdable


func (c *ClusterClient) Process(ctx context.Context, cmd Cmder) error {
	return c.hooks.process(ctx, cmd, c.process)
}
//hooks只是执行前后钩子，直接看process
func (c *ClusterClient) process(ctx context.Context, cmd Cmder) error {
	cmdInfo := c.cmdInfo(cmd.Name())
    //根据命令找到哈希槽
	slot := c.cmdSlot(cmd)

	var node *clusterNode
	var ask bool
	var lastErr error
    //执行命令多次重试，重试次数由配置决定
	for attempt := 0; attempt <= c.opt.MaxRedirects; attempt++ {
        //每次重试要休眠一段时间，因为有些错误会触发更新集群状态，即ClusterClient.state，后面具体说到
		if attempt > 0 {
			if err := internal.Sleep(ctx, c.retryBackoff(attempt)); err != nil {
				return err
			}
		}

        //根据slot找到node,node在历次重试中可能会被重新置空，比如触发更新集群状态时
		if node == nil {
			var err error
			node, err = c.cmdNode(ctx, cmdInfo, slot)
			if err != nil {
				return err
			}
		}

        //根据选node执行命令，除上一次尝试返回ask外，正常都走else分支
		if ask {
			pipe := node.Client.Pipeline()
			_ = pipe.Process(ctx, NewCmd(ctx, "asking"))
			_ = pipe.Process(ctx, cmd)
			_, lastErr = pipe.Exec(ctx)
			_ = pipe.Close()
			ask = false
		} else {
			lastErr = node.Client.Process(ctx, cmd)
		}

		// If there is no error - we are done.
		if lastErr == nil {
			return nil
		}
        //收到 READONLY 错误响应，会触发更新集群状态，就是c.state.LazyReload，
        //node置空，下次重试重新获取node,前面经过一定时间休眠后，集群状态可能已经更新，会获得新的node
        //收到 READONLY 可能是集群发生了failover
		if isReadOnly := isReadOnlyError(lastErr); isReadOnly || lastErr == pool.ErrClosed {
			if isReadOnly {
				c.state.LazyReload()
			}
			node = nil
			continue
		}

		// If slave is loading - pick another node.
		if c.opt.ReadOnly && isLoadingError(lastErr) {
			node.MarkAsFailing()
			node = nil
			continue
		}

        //重定向，也需要更新集群状态，node替换为MOVED 返回的新地址对应的node
		var moved bool
		var addr string
		moved, ask, addr = isMovedError(lastErr)
		if moved || ask {
			c.state.LazyReload()

			var err error
			node, err = c.nodes.GetOrCreate(addr)
			if err != nil {
				return err
			}
			continue
		}

        //如果是其他可重试的情况，则把node标记为失败，然后重试
		if shouldRetry(lastErr, cmd.readTimeout() == nil) {
			// First retry the same node.
			if attempt == 0 {
				continue
			}

			// Second try another node.
			node.MarkAsFailing()
			node = nil
			continue
		}

        //不是以上任何情况，则直接报错
		return lastErr
	}
	return lastErr
}

//其他可重试的情况，包括客户端数超限，LOADING，READONLY，CLUSTERDOWN，TRYAGAIN
func shouldRetry(err error, retryTimeout bool) bool {
	switch err {
	case io.EOF, io.ErrUnexpectedEOF:
		return true
	case nil, context.Canceled, context.DeadlineExceeded:
		return false
	}

	if v, ok := err.(timeoutError); ok {
		if v.Timeout() {
			return retryTimeout
		}
		return true
	}

	s := err.Error()
	if s == "ERR max number of clients reached" {
		return true
	}
	if strings.HasPrefix(s, "LOADING ") {
		return true
	}
	if strings.HasPrefix(s, "READONLY ") {
		return true
	}
	if strings.HasPrefix(s, "CLUSTERDOWN ") {
		return true
	}
	if strings.HasPrefix(s, "TRYAGAIN ") {
		return true
	}

	return false
}

//更新集群状态的具体操作
func (c *clusterStateHolder) LazyReload() {
	if !atomic.CompareAndSwapUint32(&c.reloading, 0, 1) {
		return
	}
	go func() {
		defer atomic.StoreUint32(&c.reloading, 0)

		_, err := c.Reload(context.Background())
		if err != nil {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}()
}

func (c *clusterStateHolder) Reload(ctx context.Context) (*clusterState, error) {
	state, err := c.load(ctx)
	if err != nil {
		return nil, err
	}
	c.state.Store(state)
	return state, nil
}

//前面提到clusterStateHolder.load被赋值为ClusterClient.loadState
func (c *ClusterClient) loadState(ctx context.Context) (*clusterState, error) {
	//……
	addrs, err := c.nodes.Addrs()
	if err != nil {
		return nil, err
	}

	var firstErr error

    //找到一个可以成功请求的节点，用CLUSTER SLOTS命令获取集群信息，会获得slot分片，对应的主从节点
	for _, idx := range rand.Perm(len(addrs)) {
		addr := addrs[idx]

		node, err := c.nodes.GetOrCreate(addr)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		slots, err := node.Client.ClusterSlots(ctx).Result()
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

        //用获取到的slots信息实例化新的clusterState，替换原来的clusterStateHolder.state
		return newClusterState(c.nodes, slots, node.Client.opt.Addr)
	}

	/*
	 * No node is connectable. It's possible that all nodes' IP has changed.
	 * Clear activeAddrs to let client be able to re-connect using the initial
	 * setting of the addresses (e.g. [redis-cluster-0:6379, redis-cluster-1:6379]),
	 * which might have chance to resolve domain name and get updated IP address.
	 */
	c.nodes.mu.Lock()
	c.nodes.activeAddrs = nil
	c.nodes.mu.Unlock()

	return nil, firstErr
}

func newClusterState(
	nodes *clusterNodes, slots []ClusterSlot, origin string,
) (*clusterState, error) {
	c := clusterState{
		nodes: nodes,

		slots: make([]*clusterSlot, 0, len(slots)),

		generation: nodes.NextGeneration(),
		createdAt:  time.Now(),
	}

	originHost, _, _ := net.SplitHostPort(origin)
	isLoopbackOrigin := isLoopback(originHost)

    //遍历每个slot分片及其中的每个node,每个slot分片中的第一个node是master，其他是slave
    //clusterState.nodes,clusterState.Masters,clusterState.Slaves,clusterState.slots会被填充最新的集群信息
	for _, slot := range slots {
		var nodes []*clusterNode
		for i, slotNode := range slot.Nodes {
			addr := slotNode.Addr
			if !isLoopbackOrigin {
				addr = replaceLoopbackHost(addr, originHost)
			}

			node, err := c.nodes.GetOrCreate(addr)
			if err != nil {
				return nil, err
			}

			node.SetGeneration(c.generation)
			nodes = append(nodes, node)

			if i == 0 {
				c.Masters = appendUniqueNode(c.Masters, node)
			} else {
				c.Slaves = appendUniqueNode(c.Slaves, node)
			}
		}

		c.slots = append(c.slots, &clusterSlot{
			start: slot.Start,
			end:   slot.End,
			nodes: nodes,
		})
	}

	sort.Sort(clusterSlotSlice(c.slots))

	time.AfterFunc(time.Minute, func() {
		nodes.GC(c.generation)
	})

	return &c, nil
}
```