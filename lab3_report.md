# Lab3 TinySQL 两阶段提交实现与问题解决

## 一、任务目标与拆解
Lab3 主要目标是实现分布式事务的两阶段提交（2PC）协议，包括：
- 两阶段提交的核心流程
- 主键与从键的区分处理
- 错误与异常的精确处理
- 通过所有相关测试用例

## 二、实现步骤
1. **2PC 基本流程梳理**
   - 理解两阶段提交的原理：prewrite、commit 两步。
   - 明确主键与从键的处理差异。
2. **代码实现**
   - 在 2pc.go 中实现 prewrite 和 commit 逻辑。
   - 处理 RPC 错误、region 错误、写写冲突等多种异常。
3. **测试驱动开发**
   - 反复运行 store/tikv 下的测试用例，定位失败点。
   - 结合 failpoint 框架注入错误，验证异常处理。

## 三、遇到的问题与调试过程
### 1. 错误类型不匹配
- **现象**：测试期望返回 terror.ErrResultUndetermined，实际返回原始错误或被 wrap 后的错误。
- **排查**：分析测试用例，发现 terror.ErrorEqual 判断失败。
- **解决**：统一用 terror.ErrResultUndetermined 标记主键 commit 阶段的网络/超时错误。

### 2. undeterminedErr 被覆盖
- **现象**：多次 commit 重试时，undeterminedErr 被后续错误覆盖，导致测试失败。
- **排查**：检查 setUndeterminedErr 实现，发现未保护首次错误。
- **解决**：只在 undeterminedErr 为空时设置，保证首次错误类型。

### 3. region 错误处理不当
- **现象**：所有 region 错误都被标记为 undetermined，导致写写冲突测试失败。
- **排查**：对比测试用例，发现只有先遇到 RPC 错误后再遇到 region 错误才应标记 undetermined。
- **解决**：修正逻辑，仅在已存在 undeterminedErr 时 region 错误才返回 undetermined。

## 四、最终验证
- 通过所有两阶段提交相关测试，包括主键 commit 异常、写写冲突等。
- 代码逻辑清晰，错误处理分层合理。

---

# 总结
Lab3 的实现过程中，核心难点在于异常处理的细致区分和测试驱动的反复调试。通过分析测试用例、精确定位问题、逐步修正逻辑，最终实现了分布式事务的健壮两阶段提交。
