[English](/README.md) | [中文](/README.zh_CN.md)

# Mirage-UI

**REALITY 伪装站 + 订阅工作台** 一体化代理面板 —— 由三个开源项目修改整合而成。

## 来源与署名

本发行版是以下三个开源项目的修改整合成果：

| 项目 | 许可证 | 使用方式 |
| --- | --- | --- |
| [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) | AGPL-3.0 | 面板底座：Web UI、API、Xray-core 进程管理、数据库结构——本 fork 的全部代码构建于其上 |
| [Earl9/subconv-next](https://github.com/Earl9/subconv-next) | MIT | 订阅转换引擎，经其 fork [bettermgit/subconv-next-custom](https://github.com/bettermgit/subconv-next-custom) 于 commit `b63bbca` 以 vendored 方式引入（`internal/subconvengine/`），详见 [UPSTREAM.md](internal/subconvengine/UPSTREAM.md) |
| [mack-a/v2ray-agent](https://github.com/mack-a/v2ray-agent) | MIT | 设计参考：其一键安装流程启发了随机端口/密钥/路径的快速入站生成器；其 TLS fronting 封面站思路启发了 `internal/camouflage/` 伪装引擎 |

原始版权与许可文本全部保留（`LICENSE`、`internal/subconvengine/UPSTREAM-LICENSE`）。

## AI 创作声明

本发行版中的**每一处代码修改、Bug 修复、测试与文档**——包括伪装站的抗扫描
加固、订阅工作台修复、本 README 与一键安装脚本——全程由 **Qwen3.8-Flash**
模型编写、测试并整理完成。

## 核心特性

### 🎭 伪装站（Camouflage）
- REALITY/TLS 入站一键开启本机伪装站：所有 TLS 握手失败的连接（含无 SNI、
  明文探测）转交静态封面站——端口永远不会显得"死"
- 三套内置多页模板（个人博客 / 企业站 / 摄影站）：真实中文内容、内联样式与
  SVG 插画、robots.txt、RSS、favicon、自定义 404
- 抗扫描：空连接与垃圾探测无法击垮封面服务（Peek 超时 + 噪音丢弃），serve
  意外退出 2 秒自愈
- 封面证书自动生成；业务域名照常使用标准 TLS 配置

### 🛠 订阅工作台（mihomo / subconv）
- 内置规则集目录：规则源的启用 / 编辑 / 预览
- 源 → 发布（publication）→ 私有订阅链接（随机 token 路径，防扫描）
- 发布链接永远生成完整 URL（反代 URI 优先，否则按面板协议 + 订阅端口推导）

### 🩹 稳定性修复
- 工作台写接口统一 JSON 传输（修复表单编码导致的"名称必填"假报错）
- xray `fallbacks` 生成不再携带面板内部标记
- 封面站对扫描器流量的完整健壮性加固

## 一键安装

```bash
# 从你仓库的 Releases 拉取预编译二进制（推荐）
curl -fsSL https://raw.githubusercontent.com/bettermgit/mirage-ui/main/install-fork.sh | bash

# 或在服务器上源码编译（需 Go 1.27+ / Node 26+）：
git clone https://github.com/bettermgit/mirage-ui && cd mirage-ui && bash install-fork.sh --from-source
```

初始账号密码与端口也可以自行指定，不必使用随机生成的：

```bash
# XUI_USERNAME + XUI_PASSWORD 需成对提供（仅首次启动生效）；
# 行首留一个空格可避免密码进入 shell 历史
 XUI_USERNAME=admin XUI_PASSWORD='你的强密码' XUI_PANEL_PORT=8443 \
  bash install-fork.sh
```

同样的变量也适用于 Docker 镜像（`docker run -e XUI_USERNAME=... -e
XUI_PASSWORD=...`），且只在数据库尚无任何用户时生效——永远无法覆盖已存在的
账号。

### 交互模式与管理菜单

在终端里直接运行脚本会进入**问答式安装**：询问面板端口（校验范围与占用）、
可选自设账号密码（隐藏输入、二次确认）；管道执行或环境变量给齐时保持全自动。
Xray 内核在安装时自动下载；若下载失败，之后也可在面板设置中补装。

已安装的服务器上**重新运行脚本**会打开**管理菜单**：启动 / 停止 / 重启 /
状态、修改端口、修改登录凭据（经面板校验当前密码）、查看访问信息、一键升级到
最新 Release（保留旧二进制，启动失败自动回滚）、签发/续期免费 HTTPS 证书、
卸载（默认保留数据，需确认才删除）。

安装完成后终端会打印面板地址与初始账号。安全提示：

1. **首次登录请立即修改密码**
2. 系统防火墙与云安全组需同时放行：面板端口（默认 5566）、订阅端口 2096、
   以及你创建的每个入站端口

## Docker

```bash
docker build -t mirage-ui .
docker run -d --name mirage-ui --restart unless-stopped --network host \
  -v mirage-xui:/etc/x-ui mirage-ui
```

`--network host` 让代理入站无需逐个映射任意端口；需要隔离时可改用
`-p 2053:2053 -p 2096:2096 …`。面板数据库持久化在 `mirage-xui` 卷中。

## HTTPS

安装脚本内置完整证书流程：安装尾声或管理菜单（选项 10）会自动安装
acme.sh、经 HTTP-01（需 80 端口空闲且公网可达）签发免费的 Let's Encrypt
证书、存入安装目录、通过面板自身 API 写入配置并重启——续期由 acme.sh
定时任务自动完成并重载服务。也可仍在「设置 → SSL」手动填写证书路径，
配合任一 ACME 工具（acme.sh / certbot）；443 被 REALITY 占用时，
签发验证请走 HTTP-01（80 端口）或 DNS-01。

## 维护者：发布 Release

```bash
git tag v1.0.1 && git push origin v1.0.1
```

推送 `v*` 标签即触发发版工作流，自动构建 `x-ui-linux-amd64` 与
`x-ui-linux-arm64` 两个二进制并挂到对应 Release。为已存在的标签重新构建：
Actions → release → Run workflow。

## 开发

本地门禁为 `make verify`；工程约定、测试哲学（TDD）与分层规则见
[CLAUDE.md](CLAUDE.md)。

## 许可证

AGPL-3.0（继承自 3x-ui）。内置的订阅转换引擎保留其 MIT 许可
（见 `internal/subconvengine/UPSTREAM-LICENSE`）。
