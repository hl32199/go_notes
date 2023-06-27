#### 消费者消息确认模式？
两种确认模式：自动和手动。  
amqp协议中，consume方法和get方法，no-ack=true即自动确认，no-ack=false即手动确认。（参见ampq 0-9-1协议 1.8.3.3和1.8.3.10节）  
no-ack：
```text
\[no acknowledgement needed\] If this field is set the server does not expect
acknowledgements for messages. That is, when a message is delivered to the
client the server assumes the delivery will succeed and immediately dequeues it.
This functionality may increase performance but at the cost of reliability.
Messages can get lost if a client dies before they are delivered to the application.
```
（参见 ampq 0-9-1协议 1.1 no-ack）
go版消费者客户端：
```text
/*
When autoAck (also known as noAck) is true, the server will acknowledge
  deliveries to this consumer prior to writing the delivery to the network.  When
  autoAck is true, the consumer should not call Delivery.Ack. Automatically
  acknowledging deliveries means that some deliveries may get lost if the
  consumer is unable to process them after the server delivers them.
*/
func (ch *Channel) Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args Table) (<-chan Delivery, error)

/*
When autoAck is true, the server will automatically acknowledge this message so you 
don't have to. But if you are unable to fully process this message before the channel 
or connection is closed, the message will not get requeued.
*/
func (ch *Channel) Get(queue string, autoAck bool) (msg Delivery, ok bool, err error)
```
  
自动确认模式下，消息写入tcp套接字即被认为成功交付。  
自动确认的两个问题：
- 如果消费者tcp连接或channel在消息成功交付到消费者前关闭，则消息会丢失，所以非数据安全。
- 因为prefetch(限制channel上未确认的消息数量)只能跟手动确认模式一起使用，自动确认模式下，
可能造成消费者消息积压导致内存耗尽或进程被操作系统终止。  

#### 手动确认模式下，消息如何确认？
可以使用ack,nack,reject方法确认消息，其中ack和reject是对amqp的实现，nack是rabbitmq的扩展。  
ack和reject对应的amqp协议的1.8.3.13和1.8.3.14节。  
go版客户端的delivery和channel实体都实现了这三个方法：
```text
/*
Ack delegates an acknowledgement through the Acknowledger interface that the client or 
server has finished work on a delivery.
All deliveries in AMQP must be acknowledged. If you called Channel.Consume with autoAck 
true then the server will be automatically ack each message and this method should not 
be called. Otherwise, you must call Delivery.Ack after you have successfully processed 
this delivery.
When multiple is true, this delivery and all prior unacknowledged deliveries on the 
same channel will be acknowledged. This is useful for batch processing of deliveries.
An error will indicate that the acknowledge could not be delivered to the channel it 
was sent from.
Either Delivery.Ack, Delivery.Reject or Delivery.Nack must be called for every delivery 
that is not automatically acknowledged.
*/
func (d Delivery) Ack(multiple bool) error

/*
Reject delegates a negatively acknowledgement through the Acknowledger interface.
When requeue is true, queue this message to be delivered to a consumer on a different 
channel. When requeue is false or the server is unable to queue this message, it will 
be dropped.
If you are batch processing deliveries, and your server supports it, prefer Delivery.Nack.
Either Delivery.Ack, Delivery.Reject or Delivery.Nack must be called for every delivery 
that is not automatically acknowledged.
*/
func (d Delivery) Reject(requeue bool) error
/*
Nack negatively acknowledge the delivery of message(s) identified by the delivery tag 
from either the client or server.
When multiple is true, nack messages up to and including delivered messages up until 
the delivery tag delivered on the same channel.
When requeue is true, request the server to deliver this message to a different consumer. 
If it is not possible or requeue is false, the message will be dropped or delivered to a 
server configured dead-letter queue.
This method must not be used to select or requeue messages the client wishes not to handle, 
rather it is to inform the server that the client is incapable of handling this message at 
this time.
Either Delivery.Ack, Delivery.Reject or Delivery.Nack must be called for every delivery 
that is not automatically acknowledged.
*/
func (d Delivery) Nack(multiple, requeue bool) error
```
nack是为了支持批量reject做的扩展，它多了一个multiple参数。  
对于nack和reject，如果requeue参数为false，消息会被路由到死信队列(如果配置了死信队列)，否则被丢弃。  
如果为true,消息在条件允许的情况下，会被放到原来的位置，如果不能，则会放到更接近队列头部的位置。  
如果所有消费者都reject一条消息，那么会形成死循环，从而大量消耗带宽和cpu资源。  
对于ack和nack，如果multiple=true，那么在当前channel中所有未确认且delivery_tag<=确认参数中的
delever_tag的消息都会被确认。

#### 手动确认模式下，未确认的消息最后都怎么样了？
当连接或通道关闭（This includes TCP connection loss by clients, consumer application (process) failures, and channel-level 
protocol exceptions）时，未确认的消息会被重新入队(requeue)，所以消费者可能会收到之前发送给其他消费者的消息，消费者应该实现消息消费的幂等性。  
消息发送到客户端时，有一个参数`redelivered`，首次投递被设置为false，否则为true。(参见ampq 0-9-1协议 1.8.3.9 和 1.1 redelivered)  
rabbitmq需要一段时间才能探测到客户端不可用，通过心跳和TCP keepalives探测不可用的TCP连接。
#### 消息消费失败，应该如何确认？
先把消费失败分为两种情况：
- 可立即重试的：
可以reject，并制定requeue来重试，或者在消费程序里直接重试。
- 需要延后一定时间重试的：
rabbitmq提供了私信队列功能，可以配置私信队列结合reject方法来实现一定时间后重试，但是不够灵活，想多次重试或灵活设置重试等待时间不容易实现。  
所以可能需要自己实现，比如把失败的消息记录下来，定时扫描重试。记下消息后，即可ack消息。 
- 不可重试的：
告警和记录日志，以便人工介入排查问题和修复。消息需要ack，否则会导致消息一直保留在server端，或客户端关闭后，消息被推送给其他消费者，另外还可能
影响消费者被继续分配消息（如果设置了prefetch，且有较多消费失败的消息）。
### 集群相关
- 对于镜像集群,单个队列负载过重无法分散压力。
- 对于普通集群，消费的队列不在连接的节点，会通过连接的节点转发消息，通信负担重。
- 普通和镜像集群，消息的状态变更都要同步到副本，创建和消费后删除都要同步，通信负担重。（这一点kafka不同，只需要同步offset）

### 问题列表
- 镜像队列，主从间同步的协议：GM（Guaranteed Multicast）。一个节点失效后，相邻节点如何感知到？
- 订阅同一个队列的多个消费者，平摊消息，新加入消费者时，如何变化？

### 参考资料
[AMQP version 0­9­1](https://www.rabbitmq.com/resources/specs/amqp-xml-doc0-9-1.pdf)  
[消费者和生产者确认](https://www.rabbitmq.com/confirms.html)  
[go版rabbitmq客户端文档](https://pkg.go.dev/github.com/streadway/amqp#section-documentation)