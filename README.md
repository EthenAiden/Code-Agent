# Agent Server

一个基于多智能体架构的 AI 代码助手后端服务，使用 Go 语言开发，集成了意图分类、任务规划、执行和重规划等功能。

## 项目概述

Agent Server 是一个智能代码助手系统，采用多层次的智能体架构：

- **Sequential Agent（顺序智能体）**：顶层协调器，负责整体流程编排
- **Intent Classifier（意图分类器）**：识别用户意图，将请求路由到合适的处理流程
- **Plan-Execute Agent（计划执行智能体）**：包含规划器、执行器和重规划器
  - **Planner（规划器）**：将复杂任务分解为可执行的步骤
  - **Executor（执行器）**：执行具体的任务步骤
  - **Replanner（重规划器）**：根据执行结果动态调整计划

## 技术栈

- **语言**：Go 1.26.1
- **Web 框架**：CloudWeGo Hertz
- **AI 框架**：CloudWeGo Eino
- **数据库**：MySQL
- **缓存**：Redis
- **LLM**：OpenAI API

## 核心功能

### 1. 多智能体架构
- 意图识别与分类
- 智能任务规划
- 自动化任务执行
- 动态重规划能力

### 2. 工具系统
- **文件操作**：读取、写入、列出目录
- **代码执行**：安全的代码执行环境
- **项目上下文**：智能项目信息管理

### 3. 项目管理
- 创建和管理多个项目会话
- 项目级别的对话历史
- 项目上下文持久化

### 4. 消息历史
- 完整的对话历史记录
- 支持 MySQL 持久化存储
- Redis 缓存加速访问

## 项目结构

```
agent-server/
├── agent/                    # 智能体核心模块
│   ├── context/             # 上下文管理
│   ├── executor/            # 任务执行器
│   ├── intent/              # 意图分类器
│   ├── model/               # LLM 模型封装
│   ├── planner/             # 任务规划器
│   ├── planexecute/         # 计划执行智能体
│   ├── replanner/           # 重规划器
│   ├── sequential/          # 顺序智能体
│   └── tools/               # 工具集合
├── config/                  # 配置管理
├── handler/                 # HTTP 处理器
├── middleware/              # 中间件
├── model/                   # 数据模型
├── repository/              # 数据访问层
├── service/                 # 业务逻辑层
├── docker-compose.yml       # Docker 编排配置
├── Dockerfile              # Docker 镜像构建
├── go.mod                  # Go 模块依赖
├── init-db-improved.sql    # 数据库初始化脚本
└── main.go                 # 应用入口
```

## 快速开始

### 前置要求

- Go 1.26.1 或更高版本
- MySQL 5.7 或更高版本
- Redis 6.0 或更高版本
- OpenAI API Key

### 环境配置

1. 复制环境变量模板：
```bash
cp .env.example .env
```

2. 编辑 `.env` 文件，配置必要的环境变量：

```env
# OpenAI Configuration
OPENAI_API_KEY=your_openai_api_key
OPENAI_BASE_URL=https://api.openai.com/v1
OPENAI_MODEL=gpt-4

# Database Configuration
DB_HOST=localhost
DB_PORT=3306
DB_USER=root
DB_PASSWORD=your_password
DB_NAME=agent_sessions

# Redis Configuration
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=

# Server Configuration
PORT=8888

# Project Root (optional)
PROJECT_ROOT=/path/to/your/project
```

### 使用 Docker Compose 启动（推荐）

1. 启动所有服务：
```bash
docker-compose up -d
```

2. 查看日志：
```bash
docker-compose logs -f
```

3. 停止服务：
```bash
docker-compose down
```

### 本地开发启动

1. 安装依赖：
```bash
go mod download
```

2. 初始化数据库：
```bash
mysql -u root -p < init-db-improved.sql
```

3. 启动 Redis：
```bash
redis-server
```

4. 运行应用：
```bash
go run main.go
```

服务将在 `http://localhost:8888` 启动。

## API 文档

### 健康检查

#### GET /health
检查服务健康状态

**响应示例：**
```json
{
  "status": "healthy",
  "timestamp": "2024-03-20T10:00:00Z"
}
```

#### GET /ready
检查服务就绪状态

### 项目管理

所有 API 端点都需要在请求头中包含认证信息：
```
Authorization: Bearer your_token
```

#### POST /api/v1/projects
创建新项目

**请求体：**
```json
{
  "name": "My Project",
  "description": "Project description",
  "metadata": {
    "language": "go",
    "framework": "hertz"
  }
}
```

**响应示例：**
```json
{
  "project_id": "uuid-string",
  "name": "My Project",
  "description": "Project description",
  "created_at": "2024-03-20T10:00:00Z"
}
```

#### GET /api/v1/projects
获取所有项目列表

**响应示例：**
```json
{
  "projects": [
    {
      "project_id": "uuid-string",
      "name": "My Project",
      "created_at": "2024-03-20T10:00:00Z"
    }
  ]
}
```

#### GET /api/v1/projects/:project_id
获取项目详情

#### DELETE /api/v1/projects/:project_id
删除项目

### 对话管理

#### POST /api/v1/projects/:project_id/chat
发送消息到项目

**请求体：**
```json
{
  "message": "帮我创建一个 HTTP 服务器",
  "stream": false
}
```

**响应示例：**
```json
{
  "message_id": "uuid-string",
  "response": "我将帮你创建一个 HTTP 服务器...",
  "timestamp": "2024-03-20T10:00:00Z"
}
```

#### GET /api/v1/projects/:project_id/messages
获取项目的消息历史

**查询参数：**
- `limit`: 返回消息数量（默认：50）
- `offset`: 偏移量（默认：0）

## 配置说明

### 数据库配置

| 环境变量 | 说明 | 默认值 |
|---------|------|--------|
| DB_HOST | MySQL 主机地址 | localhost |
| DB_PORT | MySQL 端口 | 3306 |
| DB_USER | 数据库用户名 | root |
| DB_PASSWORD | 数据库密码 | - |
| DB_NAME | 数据库名称 | agent_sessions |
| DB_MAX_OPEN_CONNS | 最大连接数 | 20 |
| DB_MAX_IDLE_CONNS | 最大空闲连接数 | 5 |

### Redis 配置

| 环境变量 | 说明 | 默认值 |
|---------|------|--------|
| REDIS_HOST | Redis 主机地址 | localhost |
| REDIS_PORT | Redis 端口 | 6379 |
| REDIS_PASSWORD | Redis 密码 | - |
| REDIS_SESSION_TTL | 会话缓存过期时间 | 24h |
| REDIS_POOL_SIZE | 连接池大小 | 10 |

### OpenAI 配置

| 环境变量 | 说明 | 默认值 |
|---------|------|--------|
| OPENAI_API_KEY | OpenAI API 密钥 | - |
| OPENAI_BASE_URL | API 基础 URL | https://api.openai.com/v1 |
| OPENAI_MODEL | 使用的模型 | gpt-4 |

## 开发指南

### 运行测试

```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./agent/intent/...

# 运行测试并显示覆盖率
go test -cover ./...

# 生成覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 代码规范

项目遵循标准的 Go 代码规范：

```bash
# 格式化代码
go fmt ./...

# 运行 linter
golangci-lint run
```

### 添加新工具

1. 在 `agent/tools/` 目录下创建新的工具文件
2. 实现 `tool.BaseTool` 接口
3. 在 `main.go` 的 `initializeTools` 函数中注册工具

示例：
```go
// agent/tools/my_tool.go
package tools

import (
    "context"
    "github.com/cloudwego/eino/components/tool"
)

type MyTool struct {
    // 工具配置
}

func NewMyTool() tool.BaseTool {
    return &MyTool{}
}

func (t *MyTool) Info(ctx context.Context) (*tool.Info, error) {
    return &tool.Info{
        Name: "my_tool",
        Desc: "工具描述",
    }, nil
}

func (t *MyTool) InvokableRun(ctx context.Context, argumentsInJSON string) (string, error) {
    // 实现工具逻辑
    return "result", nil
}
```

## 故障排查

### 常见问题

1. **数据库连接失败**
   - 检查 MySQL 服务是否运行
   - 验证数据库配置是否正确
   - 确认数据库已创建并初始化

2. **Redis 连接失败**
   - 检查 Redis 服务是否运行
   - 验证 Redis 配置是否正确
   - 注意：Redis 是可选的，服务可以在没有 Redis 的情况下运行

3. **OpenAI API 调用失败**
   - 验证 API Key 是否正确
   - 检查网络连接
   - 确认 API 配额是否充足

### 日志查看

```bash
# Docker 环境
docker-compose logs -f agent-server

# 本地开发
# 日志会直接输出到控制台
```

## 性能优化

### 数据库优化
- 使用连接池管理数据库连接
- 合理设置 `MaxOpenConns` 和 `MaxIdleConns`
- 定期清理过期数据

### Redis 缓存
- 启用 Redis 可显著提升查询性能
- 合理设置 TTL 避免内存溢出
- 使用缓存预热策略

### 并发控制
- 使用 Hertz 的高性能特性
- 合理设置 goroutine 数量
- 避免阻塞操作

## 部署

### Docker 部署

```bash
# 构建镜像
docker build -t agent-server:latest .

# 运行容器
docker run -d \
  --name agent-server \
  -p 8888:8888 \
  --env-file .env \
  agent-server:latest
```

### 生产环境建议

1. 使用反向代理（Nginx/Caddy）
2. 启用 HTTPS
3. 配置日志收集
4. 设置监控和告警
5. 定期备份数据库
6. 使用负载均衡（多实例部署）

## 贡献指南

欢迎贡献代码！请遵循以下步骤：

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

## 许可证

本项目采用 MIT 许可证。详见 [LICENSE](LICENSE) 文件。

## 联系方式

如有问题或建议，请通过以下方式联系：

- 提交 Issue
- 发送邮件至：[your-email@example.com]

## 致谢

- [CloudWeGo](https://www.cloudwego.io/) - 提供高性能的 Go 框架

---

**注意**：本项目仍在积极开发中，API 可能会发生变化。
