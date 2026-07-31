# dicom-cli

`dicom-cli` 是用于 DICOM 文件处理和 DIMSE SCU 操作的命令行工具。它支持
查看、校验、编辑、脱敏、图像/元数据转换、传输语法转码，以及 C-ECHO 和
C-STORE。

## 安装

从 GitHub Releases 下载与操作系统和 CPU 架构相符的归档，解压后直接运行：

| 平台 | 归档 |
| --- | --- |
| Windows x64 | `dicom-cli_<version>_windows_amd64.zip` |
| Linux x64 | `dicom-cli_<version>_linux_amd64.tar.gz` |
| Linux ARM64 | `dicom-cli_<version>_linux_arm64.tar.gz` |
| macOS Intel | `dicom-cli_<version>_darwin_amd64.tar.gz` |
| macOS Apple Silicon | `dicom-cli_<version>_darwin_arm64.tar.gz` |

Windows 使用 `dicom-cli.exe`；Linux 和 macOS 使用 `dicom-cli`。每个归档都带有
本 README、[完整使用手册](docs/usage.md) 和两个可复制修改的示例文件。

从源码构建需要 `go.mod` 指定的 Go 版本：

```powershell
go build -o dicom-cli.exe ./cmd/dicom-cli
.\dicom-cli.exe --help
```

```sh
go build -o dicom-cli ./cmd/dicom-cli
./dicom-cli --help
```

## 快速开始

查看一个 DICOM 文件：

```sh
dicom-cli inspect study/image.dcm
```

创建并校验本地运行配置与规则文件：

```sh
dicom-cli config init dicom-cli.yaml
dicom-cli config validate dicom-cli.yaml
dicom-cli rules init dicom-cli-rules.yaml
dicom-cli rules validate dicom-cli-rules.yaml
```

使用内置基础 profile 脱敏，源文件不会被改写：

```sh
dicom-cli anonymize --profile basic --output anonymized study
```

导出 DICOM 首帧为 PNG：

```sh
dicom-cli convert image --format png --output image.png study/image.dcm
```

使用命名 PACS 目标进行 C-ECHO：

```sh
dicom-cli echo --config dicom-cli.yaml --target local-pacs
```

## 命令

| 命令 | 用途 |
| --- | --- |
| `inspect <file>` | 查看单个 DICOM 文件的摘要、指定标签或完整数据元素。 |
| `validate <file>` | 执行基础、标准和规则文件中的校验。 |
| `edit <file>` | 写出修改后的单个 DICOM 文件。 |
| `anonymize <file-or-directory>` | 按内置或命名 profile 写出脱敏文件。 |
| `convert image|json` | 从 DICOM 导出图像或 JSON。 |
| `encapsulate image <file-or-directory>` | 将 PNG/JPEG 封装为未压缩的 Secondary Capture DICOM。 |
| `transcode <file>` | 使用当前二进制实际注册的 codec 重编码 DICOM。 |
| `transcode formats` | 显示可用传输语法、别名和编解码能力。 |
| `echo` | 对目标执行 DICOM C-ECHO。 |
| `send <file-or-directory-or->` | 向目标执行 DICOM C-STORE。 |
| `config` | 初始化、校验和维护运行配置及命名 PACS 目标。 |
| `rules` | 初始化和校验 DICOM 规则文件。 |

完整参数、输入输出约定、配置优先级、DIMSE 超时和命令示例见
[使用手册](docs/usage.md)。

## 语言

命令帮助、文本结果、进度和本工具生成的诊断由运行配置文件的根字段 `language`
控制。支持 `en`（默认）和 `zh-CN`；使用 `lang <en|zh-CN>` 修改所选配置后，
后续命令会自动使用新语言。命令行、JSON 字段和退出码保持不变。

```yaml
language: zh-CN
```

```sh
dicom-cli -c dicom-cli.yaml lang zh-CN
```

## 退出码

| 退出码 | 含义 |
| --- | --- |
| `0` | 命令成功完成。 |
| `1` | 处理、文件、网络、DIMSE 或批处理操作失败。 |
| `2` | 命令参数、配置或规则输入无效。 |
| `3` | DICOM 校验失败；`validate --strict` 会把 warning 也视为失败。 |

## 兼容性边界

归档和 CI 使用合成 fixture 验证基础行为。真实 DICOM 样本、真实 PACS 和外部
codec 互操作需要在受控环境中另行验收。HTJ2K 在 `transcode formats` 中始终为
experimental，不能据此视为完成生产互操作验证。
