# go如何使用epoll

### 创建epoll描述符和注册fd事件
fd创建的时候即会调用epoll_ctl注册事件。  
net.Dial方法(用于创建tcp,udp等链接)，os.stdin,os.stdout,os.stderr,os.Open(用于打开文件)，
os.Create(创建文件)都会初始化poll.pollDesc，调用其初始化方法 init,看代码：
```go
package poll

type pollDesc struct {
	runtimeCtx uintptr
}

var serverInit sync.Once

func (pd *pollDesc) init(fd *FD) error {
	serverInit.Do(runtime_pollServerInit)
	ctx, errno := runtime_pollOpen(uintptr(fd.Sysfd))
	//……
	pd.runtimeCtx = ctx
	return nil
}
```
serverInit.Do(runtime_pollServerInit)初始化epoll描述符，判断如果未初始化则执行初始化：
```go
package sync

func (o *Once) Do(f func()) {
	// Note: Here is an incorrect implementation of Do:
	//
	//	if atomic.CompareAndSwapUint32(&o.done, 0, 1) {
	//		f()
	//	}
	//
	// Do guarantees that when it returns, f has finished.
	// This implementation would not implement that guarantee:
	// given two simultaneous calls, the winner of the cas would
	// call f, and the second would return immediately, without
	// waiting for the first's call to f to complete.
	// This is why the slow path falls back to a mutex, and why
	// the atomic.StoreUint32 must be delayed until after f returns.
	if atomic.LoadUint32(&o.done) == 0 {
		// Outlined slow-path to allow inlining of the fast-path.
		o.doSlow(f)
	}
}

func (o *Once) doSlow(f func()) {
	o.m.Lock()
	defer o.m.Unlock()
	if o.done == 0 {
		defer atomic.StoreUint32(&o.done, 1)
		f()
	}
}
```
internal/poll.runtime_pollServerInit对应着runtime.poll_runtime_pollServerInit
```go
package runtime

func poll_runtime_pollServerInit() {
	netpollGenericInit()
}

func netpollGenericInit() {
	if atomic.Load(&netpollInited) == 0 {
		lockInit(&netpollInitLock, lockRankNetpollInit)
		lock(&netpollInitLock)
		if netpollInited == 0 {
			netpollinit()
			atomic.Store(&netpollInited, 1)
		}
		unlock(&netpollInitLock)
	}
}

var (
	epfd int32 = -1 // epoll descriptor 全局的
	netpollBreakRd, netpollBreakWr uintptr // for netpollBreak 不太清楚作用
	netpollWakeSig uint32 // used to avoid duplicate calls of netpollBreak
)

func netpollinit() {
	epfd = epollcreate1(_EPOLL_CLOEXEC)
	if epfd < 0 {
        //……                    
	}
	r, w, errno := nonblockingPipe()
	//……
	ev := epollevent{
		events: _EPOLLIN,
	}
	*(**uintptr)(unsafe.Pointer(&ev.data)) = &netpollBreakRd
	errno = epollctl(epfd, _EPOLL_CTL_ADD, r, &ev)
	//……
	netpollBreakRd = uintptr(r)
	netpollBreakWr = uintptr(w)
}

func poll_runtime_pollOpen(fd uintptr) (*pollDesc, int) {
	pd := pollcache.alloc()
	//……
	pd.fd = fd
	pd.rg = 0
	pd.wg = 0
    //……

	var errno int32
	errno = netpollopen(fd, pd)
	return pd, int(errno)
}

func netpollopen(fd uintptr, pd *pollDesc) int32 {
	var ev epollevent
	ev.events = _EPOLLIN | _EPOLLOUT | _EPOLLRDHUP | _EPOLLET
	*(**pollDesc)(unsafe.Pointer(&ev.data)) = pd
	return -epollctl(epfd, _EPOLL_CTL_ADD, int32(fd), &ev)
}
```
### 通过epoll_wait结合携程挂起与唤醒提高cpu使用效率
以net包的conn结构体的Read方法为例，往下寻找调用链：
```go
package net

type conn struct {
	fd *netFD
}

// Network file descriptor.
type netFD struct {
	pfd poll.FD
    //……
}

// FD is a file descriptor. The net and os packages embed this type in
// a larger type representing a network connection or OS file.
type FD struct {
	//……
	// I/O poller.
	pd pollDesc
    //……
}

type pollDesc struct {
	runtimeCtx uintptr
}

func (c *conn) Read(b []byte) (int, error) {
    //……
	n, err := c.fd.Read(b)
    //……
	return n, err
}

func (fd *netFD) Read(p []byte) (n int, err error) {
	n, err = fd.pfd.Read(p)
	//……
}
```
```go
package poll

func (fd *FD) Read(p []byte) (int, error) {
	//……
	for {
		//……
				if err = fd.pd.waitRead(fd.isFile); err == nil {
					continue
				}
        //……
		return n, err
	}
}

func (pd *pollDesc) waitRead(isFile bool) error {
	return pd.wait('r', isFile)
}

func (pd *pollDesc) wait(mode int, isFile bool) error {
	//……
	res := runtime_pollWait(pd.runtimeCtx, mode)
	return convertErr(res, isFile)
}
```
这里runtime_pollWait会跳到runtime包的poll_runtime_pollWait方法，不知道为啥。
上面的pollDesc和下面的pollDesc是不同的结构体，如何实现转换的尚不清楚。
```go
package runtime

type pollDesc struct {
	//……
	fd      uintptr
 	rg      uintptr   // pdReady, pdWait, G waiting for read or nil
	wg      uintptr   // pdReady, pdWait, G waiting for write or nil
   //……
}

// poll_runtime_pollWait, which is internal/poll.runtime_pollWait,
// waits for a descriptor to be ready for reading or writing,
// according to mode, which is 'r' or 'w'.
// This returns an error code; the codes are defined above.
//go:linkname poll_runtime_pollWait internal/poll.runtime_pollWait
func poll_runtime_pollWait(pd *pollDesc, mode int) int {
	//……
	for !netpollblock(pd, int32(mode), false) {
		//……
	}
	return pollNoError
}

// returns true if IO is ready, or false if timedout or closed
// waitio - wait only for completed IO, ignore errors
func netpollblock(pd *pollDesc, mode int32, waitio bool) bool {
	gpp := &pd.rg
    	if mode == 'w' {
    		gpp = &pd.wg
    	}
    //……
		gopark(netpollblockcommit, unsafe.Pointer(gpp), waitReasonIOWait, traceEvGoBlockNet, 5)
	//……
}

func gopark(unlockf func(*g, unsafe.Pointer) bool, lock unsafe.Pointer, reason waitReason, traceEv byte, traceskip int) {
	//……
    //注意这里的两个赋值，一个是上面的netpollblockcommit，一个是pollDesc.rg或pollDesc.wg,在park_m要用
    mp.waitlock = lock
	mp.waitunlockf = unlockf
	//……
	mcall(park_m)
}

//mcall会从当前g切换到g0,然后执行park_m,当前g作为park_m的参数
func mcall(fn func(*g))

// park continuation on g0.
func park_m(gp *g) {
	//……
	casgstatus(gp, _Grunning, _Gwaiting)
	dropg()
	//……
    if fn := _g_.m.waitunlockf; fn != nil {
		ok := fn(gp, _g_.m.waitlock)
		_g_.m.waitunlockf = nil
		_g_.m.waitlock = nil
		if !ok {
			if trace.enabled {
				traceGoUnpark(gp, 2)
			}
			casgstatus(gp, _Gwaiting, _Grunnable)
			execute(gp, true) // Schedule it back, never returns.
		}
	}
	schedule()
}

//这个方法在park_m中执行，其中包括把pollDesc.rg或pollDesc.wg赋值为当前g的地址，表示这个g在等待这个事件就绪
func netpollblockcommit(gp *g, gpp unsafe.Pointer) bool {
	r := atomic.Casuintptr((*uintptr)(gpp), pdWait, uintptr(unsafe.Pointer(gp)))
	//……
	return r
}

// One round of scheduler: find a runnable goroutine and execute it.
// Never returns.
func schedule() {
	//……
top:
	//……
	if gp == nil {
		gp, inheritTime = findrunnable() // blocks until work is available
	}
    //……

	execute(gp, inheritTime)
}

// Finds a runnable goroutine to execute.
// Tries to steal from other P's, get g from local or global queue, poll network.
func findrunnable() (gp *g, inheritTime bool) {
	// The conditions here and in handoffp must agree: if
	// findrunnable would return a G to run, handoffp must start
	// an M.
top:
	//……
	//从本地队列获取
	if gp, inheritTime := runqget(_p_); gp != nil {
		return gp, inheritTime
	}
	//从全局队列获取
	//……
		gp := globrunqget(_p_, 0)
	//……

	// Poll network.
	// This netpoll is only an optimization before we resort to stealing.
	// We can safely skip it if there are no waiters or a thread is blocked
	// in netpoll already. If there is any kind of logical race with that
	// blocked thread (e.g. it has already returned from netpoll, but does
	// not set lastpoll yet), this thread will do blocking netpoll below
	// anyway.
	if netpollinited() && atomic.Load(&netpollWaiters) > 0 && atomic.Load64(&sched.lastpoll) != 0 {
		if list := netpoll(0); !list.empty() { // non-blocking
			gp := list.pop()
			injectglist(&list)//把g列表中的g状态改为_Grunnable，按一定规则分到全局或P的本地runq,还要启动一定数量的m
			casgstatus(gp, _Gwaiting, _Grunnable)
			//……
			return gp, false
		}
	}

	// Steal work from other P's.
	//……
			if gp := runqsteal(_p_, p2, stealRunNextG); gp != nil {
				return gp, false
			}
//……

	goto top
}

// netpoll checks for ready network connections.
// Returns list of goroutines that become runnable.
// delay < 0: blocks indefinitely
// delay == 0: does not block, just polls
// delay > 0: block for up to that many nanoseconds
func netpoll(delay int64) gList {
//……
	var events [128]epollevent
retry:
	n := epollwait(epfd, &events[0], int32(len(events)), waitms)//这里就对应这系统调用epoll_wait
	//……
	var toRun gList
	for i := int32(0); i < n; i++ {
		//……
	    var mode int32
        if ev.events&(_EPOLLIN|_EPOLLRDHUP|_EPOLLHUP|_EPOLLERR) != 0 {
            mode += 'r'
        }
        if ev.events&(_EPOLLOUT|_EPOLLHUP|_EPOLLERR) != 0 {
            mode += 'w'
        }
        if mode != 0 {
            pd := *(**pollDesc)(unsafe.Pointer(&ev.data))
            pd.everr = false
            if ev.events == _EPOLLERR {
                pd.everr = true
            }
            netpollready(&toRun, pd, mode)
        }
	}
	return toRun
}

func netpollready(toRun *gList, pd *pollDesc, mode int32) {
	var rg, wg *g
	if mode == 'r' || mode == 'r'+'w' {
		rg = netpollunblock(pd, 'r', true)
	}
	if mode == 'w' || mode == 'r'+'w' {
		wg = netpollunblock(pd, 'w', true)
	}
	if rg != nil {
		toRun.push(rg)
	}
	if wg != nil {
		toRun.push(wg)
	}
}

//这个方法会把就绪的pollDesc的rg或wg改回其他值，同时返回原来在等待就绪的g
func netpollunblock(pd *pollDesc, mode int32, ioready bool) *g {
	gpp := &pd.rg
	if mode == 'w' {
		gpp = &pd.wg
	}

	for {
		old := *gpp
		if old == pdReady {
			return nil
		}
		if old == 0 && !ioready {
			// Only set pdReady for ioready. runtime_pollWait
			// will check for timeout/cancel before waiting.
			return nil
		}
		var new uintptr
		if ioready {
			new = pdReady
		}
		if atomic.Casuintptr(gpp, old, new) {
			if old == pdWait {
				old = 0
			}
			return (*g)(unsafe.Pointer(old))
		}
	}
}

```
### 小结
大概总结一下，因为源码较多，很多略过了,可能有错漏。总结过程如下：
某一个g包含了io的操作，当该g在m中执行到io操作时，比如上面代码中的Read方法，
会把该g的地址存在pollDesc的rg或wg属性，代表该g在等待事件就绪。  
g与m分离，m会顺序的按以下方式寻找下一个可运行的g来运行：
- 从当前p本地的待运行g队列(runq)获取；
- 从全局待运行的g队列(globrunq)获取；
- 非阻塞的调用epoll_wait查询就绪的事件，并获取到就绪的事件上等待的g(前面提到的pollDesc的rg或wg)，
这些g都改为可运行状态，第一个作为下一个运行的目标g，
其他分配到p的runq或globrunq；
- 从其他p的runq偷取一部分g。
这样g在等待io事件就绪的时候，p会转而去处理另一个待运行的g，而等待io就绪的g在io事件就绪之后
会在适当的时机在某个p调度新的待处理g(schedule())的时候被放到全局和某个p的runq，在后续的调度中
可能就会重新获得运行机会。



