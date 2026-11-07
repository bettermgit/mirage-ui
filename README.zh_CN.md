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
模型在本地 [llama.cpp](https://github.com/ggml-org/llama.cpp) 环境
（单台 AMD Ryzen AI Max+ 395 工作站，UD-IQ4_XS 量化）编写、测试并整理完成。

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
curl -fsSL https://raw.githubusercontent.com/<you>/mirage-ui/main/install-fork.sh | FORK_REPO=<you>/mirage-ui bash

# 或在服务器上源码编译（需 Go 1.27+ / Node 26+）：
git clone https://github.com/<you>/mirage-ui && cd mirage-ui && bash install-fork.sh --from-source
```

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

面板与订阅服务支持标准证书配置（设置 → SSL）。配合任一 ACME 工具
（acme.sh / certbot）实现全自动签发与续期；443 被 REALITY 占用时，
签发验证请走 HTTP-01（80 端口）或 DNS-01。

## 维护者：发布 Release

```bash
cd frontend && npm ci && npm run build && cd ..
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o x-ui-linux-amd64 .
CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o x-ui-linux-arm64 .
gh release create v1.0.0 x-ui-linux-amd64 x-ui-linux-arm64
```

## 开发

本地门禁为 `make verify`；工程约定、测试哲学（TDD）与分层规则见
[CLAUDE.md](CLAUDE.md)。

## 许可证

AGPL-3.0（继承自 3x-ui）。内置的订阅转换引擎保留其 MIT 许可
（见 `internal/subconvengine/UPSTREAM-LICENSE`）。
