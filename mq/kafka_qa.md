#### 问：kafka提交offset的请求和响应参数？
消费者组（Consumer Group）偏移量(offset)信息，由一个特定的broker维护，这个broker称为消费者组协调员。即消费者需要向从这个特定的broker提交和获取偏移量。
可以通过发出一组协调员发现请求从而获得当前协调员信息`参见通讯协议API:Group Coordinator Request`。  
offset提交请求协议：
```
v0 (supported in 0.8.1 or later)
OffsetCommitRequest => ConsumerGroupId [TopicName [Partition Offset Metadata]]
  ConsumerGroupId => string
  TopicName => string
  Partition => int32
  Offset => int64
  Metadata => string
 
  
v1 (supported in 0.8.2 or later)
OffsetCommitRequest => ConsumerGroupId ConsumerGroupGenerationId ConsumerId [TopicName [Partition Offset TimeStamp Metadata]]
  ConsumerGroupId => string
  ConsumerGroupGenerationId => int32
  ConsumerId => string
  TopicName => string
  Partition => int32
  Offset => int64
  TimeStamp => int64
  Metadata => string
 
v2 (supported in 0.9.0 or later)
OffsetCommitRequest => ConsumerGroup ConsumerGroupGenerationId ConsumerId RetentionTime [TopicName [Partition Offset Metadata]]
  ConsumerGroupId => string
  ConsumerGroupGenerationId => int32
  ConsumerId => string
  RetentionTime => int64
  TopicName => string
  Partition => int32
  Offset => int64
  Metadata => string
```
offset提交的响应协议：
```
v0, v1 and v2:
OffsetCommitResponse => [TopicName [Partition ErrorCode]]]
  TopicName => string
  Partition => int32
  ErrorCode => int16
```
#### 问：拉取一批消息的请求和响应参数？
请求：
```
FetchRequest => ReplicaId MaxWaitTime MinBytes [TopicName [Partition FetchOffset MaxBytes]]
  ReplicaId => int32
  MaxWaitTime => int32
  MinBytes => int32
  TopicName => string
  Partition => int32
  FetchOffset => int64
  MaxBytes => int32
```
响应：
```
v0
FetchResponse => [TopicName [Partition ErrorCode HighwaterMarkOffset MessageSetSize MessageSet]]
  TopicName => string
  Partition => int32
  ErrorCode => int16
  HighwaterMarkOffset => int64
  MessageSetSize => int32
  
v1 (supported in 0.9.0 or later) and v2 (supported in 0.10.0 or later)
FetchResponse => ThrottleTime [TopicName [Partition ErrorCode HighwaterMarkOffset MessageSetSize MessageSet]]
  ThrottleTime => int32
  TopicName => string
  Partition => int32
  ErrorCode => int16
  HighwaterMarkOffset => int64
  MessageSetSize => int32

MessageSet => [Offset MessageSize Message]
  Offset => int64
  MessageSize => int32

```
消息格式：
```
v0
Message => Crc MagicByte Attributes Key Value
  Crc => int32
  MagicByte => int8
  Attributes => int8
  Key => bytes
  Value => bytes
  
v1 (supported since 0.10.0)
Message => Crc MagicByte Attributes Key Value
  Crc => int32
  MagicByte => int8
  Attributes => int8
  Timestamp => int64
  Key => bytes
  Value => bytes
```
#### 问：如果上一次提交的offset是10，下一次提交5，会怎么样？
> 位移提交的语义保障是由你来负责的，Kafka 只会“无脑”地接受你提交的位移。你对位移提交的管理直接影响了你的 Consumer 所能提供的消息语义保障。  

所以kafka应该不会对提交的offset做任何校验，直接覆盖之前的值。  


#### 问：offset提交相关的配置？
配置项取决于客户端的实现。
go语言以Shopify/sarama库为例：
```go
type Config struct {
//略…………
	// Consumer is the namespace for configuration related to consuming messages,
	// used by the Consumer.
	Consumer struct {
        //略…………

		// Offsets specifies configuration for how and when to commit consumed
		// offsets. This currently requires the manual use of an OffsetManager
		// but will eventually be automated.
		Offsets struct {
			// Deprecated: CommitInterval exists for historical compatibility
			// and should not be used. Please use Consumer.Offsets.AutoCommit
			CommitInterval time.Duration

			// AutoCommit specifies configuration for commit messages automatically.
			AutoCommit struct {
				// Whether or not to auto-commit updated offsets back to the broker.
				// (default enabled).
				Enable bool

				// How frequently to commit updated offsets. Ineffective unless
				// auto-commit is enabled (default 1s)
				Interval time.Duration
			}

			// The initial offset to use if no offset was previously committed.
			// Should be OffsetNewest or OffsetOldest. Defaults to OffsetNewest.
			Initial int64

			// The retention duration for committed offsets. If zero, disabled
			// (in which case the `offsets.retention.minutes` option on the
			// broker will be used).  Kafka only supports precision up to
			// milliseconds; nanoseconds will be truncated. Requires Kafka
			// broker version 0.9.0 or later.
			// (default is 0: disabled).
			Retention time.Duration

			Retry struct {
				// The total number of times to retry failing commit
				// requests during OffsetManager shutdown (default 3).
				Max int
			}
		}
    //略…………
	}
//略…………
}
```
跟自动提交有关的配置有两个参数：Config.Consumer.Offsets.Enable和Config.Consumer.Offsets.Interval。  
如果设置了自动提交，消费者初始化时会启动一个ticker，ticker每次到期会提交一次offset.  
```go
func newOffsetManagerFromClient(group, memberID string, generation int32, client Client) (*offsetManager, error) {
	// Check that we are not dealing with a closed Client before processing any other arguments
	if client.Closed() {
		return nil, ErrClosedClient
	}

	conf := client.Config()
	om := &offsetManager{
		client: client,
		conf:   conf,
		group:  group,
		poms:   make(map[string]map[int32]*partitionOffsetManager),

		memberID:   memberID,
		generation: generation,

		closing: make(chan none),
		closed:  make(chan none),
	}
	if conf.Consumer.Offsets.AutoCommit.Enable {
		om.ticker = time.NewTicker(conf.Consumer.Offsets.AutoCommit.Interval)
		go withRecover(om.mainLoop)
	}

	return om, nil
}

func (om *offsetManager) mainLoop() {
	defer om.ticker.Stop()
	defer close(om.closed)

	for {
		select {
		case <-om.ticker.C:
			om.Commit()
		case <-om.closing:
			return
		}
	}
}

func (om *offsetManager) Commit() {
	om.flushToBroker()
	om.releasePOMs(false)
}

func (om *offsetManager) flushToBroker() {
	req := om.constructRequest()
	if req == nil {
		return
	}

	broker, err := om.coordinator()
	if err != nil {
		om.handleError(err)
		return
	}

	resp, err := broker.CommitOffset(req)
	if err != nil {
		om.handleError(err)
		om.releaseCoordinator(broker)
		_ = broker.Close()
		return
	}

	om.handleResponse(broker, req, resp)
}

func (om *offsetManager) constructRequest() *OffsetCommitRequest {
	var r *OffsetCommitRequest
	//略…………

	for _, topicManagers := range om.poms {
		for _, pom := range topicManagers {
			pom.lock.Lock()
			if pom.dirty {
				r.AddBlock(pom.topic, pom.partition, pom.offset, perPartitionTimestamp, pom.metadata)
			}
			pom.lock.Unlock()
		}
	}

	if len(r.blocks) > 0 {
		return r
	}

	return nil
}
```
构造提交offset请求时用的offset是partitionOffsetManager结构体中记录的最新被标记的offset值，offset维护逻辑如下：
```go
func (s *consumerGroupSession) MarkMessage(msg *ConsumerMessage, metadata string) {
	s.MarkOffset(msg.Topic, msg.Partition, msg.Offset+1, metadata)
}

func (s *consumerGroupSession) MarkOffset(topic string, partition int32, offset int64, metadata string) {
	if pom := s.offsets.findPOM(topic, partition); pom != nil {
		pom.MarkOffset(offset, metadata)
	}
}
func (pom *partitionOffsetManager) MarkOffset(offset int64, metadata string) {
	pom.lock.Lock()
	defer pom.lock.Unlock()

	if offset > pom.offset {
		pom.offset = offset
		pom.metadata = metadata
		pom.dirty = true
	}
}
```
根据代码，虽然设置了主动提交，但是还是需要手动调用MarkMessage()才能有效的提交offset。  

#### 问：消费消息的逻辑是否可以并行，比如收到某个分区的一批消息，为每一个消息创建一个协程来处理消费逻辑？如果不能，怎样提高cpu利用率来提高消费速度？
从某个分区收到一批消息后，将每条消息分配给一个协程，消息并行执行，前一个消息(假设offset=1)执行失败而没有调用MarkMessage，
后一个消息(假设offset=2)成功执行后调用MarkMessage，此时该分区的offset会被更新为3(2+1,offset表示的是下一个未消费的消息)，
这样offset为1的消息就相当于丢失了。  
所以为了确保消息不会丢失，应该串行的消费一个分区的消息。同时为了避免阻塞后面的消息的消费，对于消费失败的消息应该有一个异常流程
来保证异步的排查修复和重试，进入异常流程后应该调用MarkMessage。
如果想充分利用服务器cpu，可以增加分区数，在消费者组中的消费者数量不变的情况下，单个消费者分配到的分区数会增加，而每个分区有一
个独立的协程在消费。

#### 问：如果消费者服务有10个实例，发版时，每个实例串行重启，是否会导致10次 rebalance?如果是并行重启呢？


### 参考资料：
[kafka通讯协议指南中文版](https://colobu.com/2017/01/26/A-Guide-To-The-Kafka-Protocol/)  
[kafka通讯协议英文原文](https://cwiki.apache.org/confluence/display/KAFKA/A+Guide+To+The+Kafka+Protocol)