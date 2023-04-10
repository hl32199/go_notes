### 不同状态channel的操作结果
| 操作 | nil channel|closed channel|
|:----|:----|:----|
|发送|block|panic|
|接收|block|下面详述|
|关闭|panic|panic|
按以下方式从已关闭channel接收数据，
```go
i,ok := <-ch
```
如果channel中还有数据，则正常读取到数据，ok为true；如果channel已空，则读到零值，ok为false。  
从以上表现推测，channel设计的目的是让发送方关闭channel，不允许接收方关闭channel。