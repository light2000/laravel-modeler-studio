# Laravel Modeler Studio

Laravel Modeler Studio 是一个面向 Laravel 项目的本地化可视化数据库建模工具。

通过画布式建模界面，你可以直观设计：

- Model
- 数据表字段
- 枚举
- 模型关系
- 数据结构

并一键生成结构清晰、可维护的 Laravel 代码。

Studio 采用「本地优先（Local First）」设计，专注于：

- 可视化建模
- 稳定可预测的代码生成
- 清晰的 Laravel 项目结构
- 尽可能减少低代码复杂度

---

# 功能特性

- 可视化 ER 建模画布
- Laravel Model 生成
- Migration 生成
- Enum 枚举生成
- 字典/枚举管理
- 关联关系设计
  - HasOne
  - HasMany
  - BelongsTo
  - BelongsToMany
  - Morph Relations
- AI 字段推荐（FREE）
- AI 枚举推荐（PRO）
- AI 关系推荐（PRO）
- 本地 Schema 管理
- Schema 快照版本管理
- 跨平台支持
  - Windows
  - macOS
  - Linux

---

# 项目架构

```text
Laravel Project
    │
    ├── composer require light2000/laravel-modeler
    │
    └── php artisan modeler:studio
                │
                ├── modeler-studio（模型设计画布）
                └── modeler-generator（代码生成器）
```

Studio 主要负责：

- 模型设计
- 画布交互
- Schema 管理
- AI 辅助设计

实际代码生成由 `laravel-modeler-generator` 完成。

---

# 安装方式

安装 Laravel 扩展包：

```bash
composer require light2000/laravel-modeler
```

安装 Studio：

```bash
php artisan modeler:install
```

自动下载所需二进制文件。

启动 Studio：

```bash
php artisan modeler:studio
```

---

# 环境要求

- PHP 8.2+
- Laravel 12+
- Windows / macOS / Linux

---

# 目录结构

```text
.modeler/
├── bin/
├── data/snapshots/
├── prompt/
├── runtime/
└── templates/
```

---

# 设计理念

Laravel Modeler Studio 并不是一个复杂的低代码平台。

它更关注：

- Schema 即唯一数据源
- 生成代码应保持可读性
- 输出结果稳定、可预测
- 尽量减少侵入式配置
- AI 作为辅助，而不是替代开发者

---

# PRO 功能

免费版已包含完整的：

- 模型设计
- 关系设计
- Laravel 代码生成
- AI 基础字段推荐

PRO 版本额外提供：

- AI 枚举推荐
- AI 关系推荐

---

# 安全性

Studio 完全本地运行，只在配置翻译/AI功能的时候请求相关三方服务接口。

---

# License

MIT

## Links

- Packagist: `light2000/laravel-modeler`
- Generator: `https://github.com/light2000/laravel-modeler-generator`
