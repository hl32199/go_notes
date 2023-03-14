## 复制和同步
### ISR
ISR(In-Sync Replicas) + OSR(Outof-Sync Replicas) = AR(Assigned Replicas 所有副本)  
leader负责维护，leader有单独的线程定期检测ISR中follower是否脱离ISR。  
延迟时间replica.lag.time.max.ms和延迟条数replica.lag.max.messages(Kafka 0.10.x版本移除了此参数)，
两个维度任意一个超过阈值，follower会被移入OSR，新加入的follower先进入OSR.
ISR中包括leader和follower.
### HW
HighWatermark,高水位.取值ISR中最小的LEO.consumer只能消费HW指向的及更早的消息。

## 数据可靠性和持久性
### acks配置
broker端参数，控制broker在响应producer成功之前需要收到ISR中多少副本的同步确认(包括leader)。取值：
1：leader成功收到即可响应；
0：producer无需等待broker确认,消息添加到socket buffer即认为成功；
-1/all:leader等待所有ISR中的副本确认后才响应producer。
### replication.factor配置
broker端参数，备份数量，应该大于min.insync.replicas,否则有一个副本不可用，则整个partition都不可用了。
### min.insync.replicas
topic级别参数。  
当producer将ack设置为“all”（或“-1”）时，min.insync.replicas指定了被认为写入成功的最小副本数。如果这个最小值不能满足，
那么producer将会引发一个异常（NotEnoughReplicas或NotEnoughReplicasAfterAppend）。
当一起使用时，min.insync.replicas和acks允许您强制更大的耐久性保证。 
一个经典的情况是创建一个复本数为3的topic，将min.insync.replicas设置为2，并且producer使用“all”选项。 
这将确保如果大多数副本没有写入producer则抛出异常。
### enable.auto.commit
consumer端参数，应设置为false,手动提交以保证消费完成才提交。
### unclean.leader.election.enable
broker端参数，指定不在ISR中的副本是否能够被选举为leader。是在可用性和数据一致性之间的选择，置为true选择可用性优先（会优先选择ISR中的副本），
false选择一致性优先。

## 消费者负载均衡
通过消费者组实现负载均衡，一个topic的任意一个partition只能由一个订阅这个topic的消费者组中的一个消费者实例(进程或线程都可)消费，
但消费者组和订阅的topic的关系是多对多。  
一个消费者组中的任意一个消费者实例启动时，需要调用JoinGroup API来加入消费者组(消费者组不存在会创建消费者组)，然后调用SyncGroup API
## rebalance
## 消息延迟批量提交造成的消息消费不及时

## 疑问

- “不过有个事情你还是要注意一下，经常有人会问主机名这个设置中我到底使用 IP 地址还是主机名。这里我给出统一的建议：最好全部使用主机名，
即 Broker 端和 Client 端应用配置中全部填写主机名。”用hostname只能在局域网中吧，还需要局域网能确保hostname能用来找到服务器？
- “如果你在某些地方使用了 IP 地址进行连接，可能会发生无法连接的问题。”为什么会无法连接？
- “但是自 1.1 开始，这种情况被修正了，坏掉的磁盘上的数据会自动地转移到其他正常的磁盘上，而且 Broker 还能正常工作。(Failover)”坏掉的磁盘，数据如何转移？
评论中有提到从其他副本转移，如果其他副本数据不全，就会丢失数据？如果是leader节点磁盘损坏，时候还会触发leader选举？
>回答：
> 1. Broker自动在好的路径上重建副本，然后从leader同步；  
> 2. Kafka支持工具能够将某个路径上的数据拷贝到其他路径上

- 消费者已拉取未ack的消息，消费者再次拉取，会重复拉到吗？
>回答：
>Kafka处理这个问题的方式不同。我们的Topic分为一组完全有序的分区，每个分区在任何给定时间由每个订阅消费者组中的一个消费者消费。这意味着消费者在每个分区中的位置只是一个整数，即要消费的下一条消息的偏移量。这使得关于已消耗的内容的状态非常小，每个分区只有一个数字。此状态可以定期检查点。这使得相当于消息确认的功能成本非常低。  
[Kafka ack](http://bcxw.net/article/671.html)

- 消费者如何提交offset,没消费一条就提交，还是可以批量提交比如提交100代表offset<=100的消息都被消费，下次提交200代表offset<=200的
消息都被消费；如果每个消息都提交，能否乱序的提交offset?


#### 参考资料
[https://colobu.com/2017/01/26/A-Guide-To-The-Kafka-Protocol/](https://colobu.com/2017/01/26/A-Guide-To-The-Kafka-Protocol/)