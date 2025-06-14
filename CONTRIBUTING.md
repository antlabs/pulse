# Contributing to Pulse

感谢您对Pulse项目的贡献！本文档将帮助您了解如何为项目做贡献。

## 开发流程

### 1. Fork和Clone

```bash
# Fork项目到您的GitHub账户
# 然后Clone到本地
git clone https://github.com/YOUR_USERNAME/pulse.git
cd pulse
```

### 2. 创建分支

```bash
git checkout -b feature/your-feature-name
```

### 3. 开发和测试

在开发过程中，请确保：

- 编写清晰的代码和注释
- 添加必要的测试用例
- 运行测试确保通过

```bash
# 运行测试
make test

# 运行测试并生成覆盖率报告
make test-coverage

# 运行代码检查
make lint
```

### 4. 提交代码

```bash
git add .
git commit -m "feat: add your feature description"
git push origin feature/your-feature-name
```

### 5. 创建Pull Request

- 创建PR并填写详细的描述
- 确保所有CI检查通过
- 等待代码审查

## CI/CD 流程

项目使用GitHub Actions进行持续集成，包括：

- **测试**：在多个Go版本(1.21, 1.22, 1.23)和操作系统(Ubuntu, macOS)上运行测试
- **代码检查**：使用golangci-lint进行代码质量检查
- **覆盖率**：生成测试覆盖率报告并上传到Codecov
- **构建**：验证代码可以正常构建

## 代码规范

- 遵循Go官方代码规范
- 使用`gofmt`格式化代码
- 通过`golangci-lint`检查
- 为新功能添加测试用例
- 保持测试覆盖率

## 报告问题

如果您发现bug或有功能建议：

1. 搜索现有的issues，避免重复
2. 使用适当的issue模板
3. 提供详细的描述和复现步骤

## 许可证

通过贡献代码，您同意您的贡献将在Apache 2.0许可证下发布。 