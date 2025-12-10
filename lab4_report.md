# Lab4 实现之旅：从规划到完成

## 一、初期规划与需求分析

当我们收到 Lab4 的任务时，需要实现一个完整的 SQL 执行链路，包括三个核心部分：
- **Lab4A**: SQL 全链路（从客户端连接到事务控制）
- **Lab4B**: INSERT 写入链路（数据插入的完整流程）
- **Lab4C**: SELECT 读取链路（数据查询与并行投影）

用户建议我们先制定详细的计划，然后逐个完成并测试每一部分。这种增量式的方法让我们能够快速反馈和纠正错误，而不是一次性实现所有代码后再统一调试。

## 二、Lab4A：SQL全链路实现

### 2.1 核心流程理解

Lab4A 的实现关键是理解 SQL 从客户端到数据库的完整执行路径：
1. **Server 层** (`server/conn.go`)：接收客户端数据包，路由不同的命令
2. **Session 层** (`session/session.go`, `session/tidb.go`)：解析 SQL、编译为执行计划、执行语句
3. **Executor 层** (`executor/adapter.go`, `executor/simple.go`)：构建执行器树、执行操作
4. **事务控制**：BEGIN/COMMIT/ROLLBACK 的实现

### 2.2 关键实现

在 `session/session.go` 的 `execute()` 方法中，我们需要实现 SQL 的三步走：
```
ParseSQL(SQL字符串) → Compile(AST节点) → executeStatement(执行计划)
```

在 `server/conn.go` 中，关键是实现 `dispatch()` 和 `handleQuery()` 来路由和处理不同类型的命令。


## 三、Lab4B：INSERT写入链路

### 3.1 插入流程设计

INSERT 的执行路径相对直接：
1. **Builder** 构建 `InsertExec` 执行器
2. **InsertExec.Open()** 初始化子执行器（如果有 SELECT）
3. **InsertExec.Next()** 根据是普通 INSERT 还是 INSERT...SELECT 分别处理
4. **insertRows** 或 **insertRowsFromSelect** 处理实际数据
5. **InsertValues.addRecord** 将每一行写入存储层


## 四、Lab4C：SELECT读取与并行投影

### 4.1 并行架构理解

这是 Lab4 最复杂的部分。Projection 执行器使用了一个优雅的并行架构，包含三个并发组件：

1. **Fetcher 线程**：从 child 读取数据，分配给 worker
2. **Worker 线程**（多个）：并行计算投影表达式
3. **主线程**：不断从 outputCh 读取已处理的结果

### 4.2 第一次尝试：死锁困境

我们最初的实现方式是让 fetcher 从 `fetcher.outputCh` 读取 output，然后直接发送给 worker，再等待 worker 返回。这导致了**持续的死锁**：

**现象**：测试程序在 10 分钟超时后被杀死，所有 worker 线程都卡在某个通道读取操作上。

**根本原因分析**：通过查看堆栈跟踪，我们发现：
- ParallelExecute 在等待从 `e.outputCh` 读取
- 所有 worker 都在等待读取 input 或 output
- Fetcher 线程不见了（或已经退出）

这表明通道资源的流向有问题。

### 4.3 关键洞察：资源循环利用

读取文档和代码注释后，我们意识到了一个重要的设计模式：**output 对象是一个可重复使用的资源**。

正确的流程应该是：
1. **初始化阶段**：在 `fetcher.outputCh` 中放入多个 output 对象（资源池）
2. **处理循环**：
   - Fetcher 从 `fetcher.outputCh` 读取空的 output 对象
   - Fetcher 把 output 发给 worker.outputCh
   - Worker 从 worker.outputCh 读 output，处理数据
   - Worker 通过 `output.done` 通道发送结果（错误或成功）
   - Fetcher 从 worker.outputCh 读已处理的 output
   - Fetcher 发送到 globalOutputCh 给上层使用
3. **资源回收**：上层消费完后，把 output 通过 `e.fetcher.outputCh <- output` 返还给资源池

### 4.4 第二个关键问题：fetcher提前退出

当从 child 读到的数据为空（没有行）时，fetcher 直接 return 了，导致没有向 globalOutputCh 发送结束信号，上层永远等待。

**修复**：即使读到空数据，也要获取一个 output 对象，向其 done 通道发送错误，然后发送到 globalOutputCh，让上层正确处理结束。

### 4.5 死锁的最终解决

经过多次迭代，我们最终确定了正确的协调模式：

**Fetcher 流程**：
```
读 input → 读 output资源 → 从child读数据 → 发送input和output给worker 
→ 等待worker处理完 → 读回processed output → 发送到globalOutputCh
```

**Worker 流程**：
```
读 input → 读 output → 计算表达式 → 发送结果到output.done 
→ 返还input给fetcher → 写output回outputCh（资源回收）
```

这个设计避免了资源竞争，因为每个对象在任何时刻都有明确的所有权。

## 五、最终验证与测试

### 5.1 Makefile 优化

我们在 Makefile 中添加了 `lab4` 目标，使其依次运行 `lab4a`、`lab4b`、`lab4c`，这样可以一次验证整个链路的正确性。

### 5.2 所有测试通过

最终，所有测试都通过了：
```
lab4a: ok  github.com/pingcap/tidb/server   0.180s
lab4a: ok  github.com/pingcap/tidb/session  0.048s
lab4b: ok  github.com/pingcap/tidb/executor 0.058s
lab4c: ok  github.com/pingcap/tidb/executor 0.061s
```


### 技术亮点

- **三层架构**：Server → Session → Executor 的清晰分离
- **执行器树模式**：通过递归初始化和 Next() 流式处理实现灵活的查询执行
- **并行投影的优雅设计**：Fetcher 负责 I/O，Worker 负责计算，很好地解耦了并发关注点
- **事务支持**：BEGIN/COMMIT/ROLLBACK 的实现确保了数据一致性

## 总结

Lab4 的完成标志着一个完整的 SQL 数据库执行引擎的实现。从网络层接收客户端请求，到解析 SQL，到编译成执行计划，再到并行执行查询，最后把结果返回给客户端 —— 整个循环都已打通。
