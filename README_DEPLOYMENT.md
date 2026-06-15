# 完整部署流程 - Windows批处理脚本版

由于bash环境故障，已创建Windows批处理脚本完成所有任务。

## 执行顺序

按照以下顺序依次执行批处理文件：

### 任务2: 配置MySQL安装包到10.1.81.41

```cmd
step0_setup_mysql_package.bat
```

**功能**:
- SSH到10.1.81.41创建/opt/packet目录
- 提示输入MySQL安装包路径（需要您提供实际路径）
- 上传安装包到10.1.81.41:/opt/packet/
- (可选) 启动HTTP服务提供下载
- **密码**: hcfc!2017

### 任务3a: 编译最新Agent

```cmd
step1_compile_agent.bat
```

**功能**:
- 编译agent/cmd/main.go
- 生成agent可执行文件
- 包含最新的进程清理修复

### 任务3b: 部署Agent到所有机器

```cmd
step2_deploy_agents.bat
```

**功能**:
- 清理10.1.81.16/17/18上的旧进程和数据
- 部署最新Agent到所有机器
- 验证Agent健康状态（端口9090）

### 任务3c: 部署MGR集群

**方式1: 通过Agent API直接部署**

```cmd
step3_deploy_mgr_via_api.bat
```

**功能**:
- 部署主节点10.1.81.16:3306
- 部署副本节点10.1.81.17:3307
- 部署副本节点10.1.81.18:3308
- 自动等待每个节点启动
- 验证MGR集群状态

**方式2: 通过Web界面部署**

1. 打开浏览器: http://localhost:3000
2. 登录平台
3. 进入"集群管理" -> "新建集群"
4. 选择"MySQL Group Replication (MGR)"
5. 配置参数：
   - 集群名称: MGR-Cluster-Test
   - 主节点: 10.1.81.16:3306
   - 副本1: 10.1.81.17:3307
   - 副本2: 10.1.81.18:3308
   - Root密码: root123
6. 点击"开始部署"

### 任务4: 验证集群和平台功能

```cmd
step4_verify_cluster.bat
```

**功能**:
- 验证MGR集群成员状态
- 检查主节点可写状态
- 检查组复制状态
- 检查Backend服务
- 提供Web界面验证清单

## 完整执行命令

在PowerShell或cmd中：

```cmd
cd d:\test_tmple\new_dbops\dbops

REM 任务2: 配置MySQL包
step0_setup_mysql_package.bat

REM 任务3: 编译和部署
step1_compile_agent.bat
step2_deploy_agents.bat
step3_deploy_mgr_via_api.bat

REM 任务4: 验证
step4_verify_cluster.bat
```

## 预期结果

完成后应看到：

1. **10.1.81.41**: /opt/packet/目录有MySQL安装包
2. **10.1.81.16/17/18**: Agent运行在9090端口
3. **MGR集群**: 3个节点ONLINE，1个PRIMARY + 2个SECONDARY
4. **Web界面**: http://localhost:3000 显示集群状态

## 注意事项

- 所有SSH操作会提示输入密码（如果没有配置免密登录）
- 10.1.81.41密码: hcfc!2017
- MySQL root密码: root123
- Agent API Token: dev-agent-token-CHANGE-ME-at-least-16

## 故障排查

如果任一步骤失败：

1. 查看Agent日志:
   ```cmd
   ssh root@10.1.81.16 "tail -100 /opt/dbops-agent/stderr.log"
   ```

2. 查看MySQL错误日志:
   ```cmd
   ssh root@10.1.81.16 "tail -100 /data/mysql/3306/error.log"
   ```

3. 检查进程:
   ```cmd
   ssh root@10.1.81.16 "ps aux | grep mysqld"
   ```

4. 手动清理重试:
   ```cmd
   ssh root@10.1.81.16 "pkill -9 mysqld; rm -rf /data/mysql/*"
   ```
