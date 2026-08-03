# dicom-cli

`dicom-cli` 是面向 DICOM 文件处理和 DIMSE SCU 操作的命令行工具。它将单文件
检查、校验和编辑，与面向文件或目录的脱敏、转换、转码和 C-STORE 操作放在同一
可脚本化入口中。

它实际提供以下能力：

- 检查、校验和编辑单个 DICOM 文件。
- 以 DICOM Basic Application Level Confidentiality Profile 为基础进行脱敏。
- 导出 DICOM 图像帧或元数据 JSON，并将受支持的 PNG/JPEG 封装为 Secondary
  Capture DICOM。
- 按当前二进制注册的 codec 转换传输语法。
- 通过明文 TCP 执行 C-ECHO 和 C-STORE。

## 安装

从 GitHub Releases 下载与操作系统和 CPU 架构相符的归档，解压后直接运行：

| 平台 | 归档 |
| --- | --- |
| Windows x64 | `dicom-cli_<version>_windows_amd64.zip` |
| Linux x64 | `dicom-cli_<version>_linux_amd64.tar.gz` |
| Linux ARM64 | `dicom-cli_<version>_linux_arm64.tar.gz` |
| macOS Intel | `dicom-cli_<version>_darwin_amd64.tar.gz` |
| macOS Apple Silicon | `dicom-cli_<version>_darwin_arm64.tar.gz` |

Windows 的可执行文件为 `dicom-cli.exe`，Linux 和 macOS 为 `dicom-cli`。归档还
包含本 README、[完整使用手册](docs/usage.md) 以及可直接修改的配置和规则示例。

从源码构建使用 `go.mod` 指定的 Go 版本：

```powershell
go build -o dicom-cli.exe ./cmd/dicom-cli
.\dicom-cli.exe --help
```

```sh
go build -o dicom-cli ./cmd/dicom-cli
./dicom-cli --help
```

## 常用流程

### 查看或校验一个文件

```sh
dicom-cli inspect study/image.dcm
dicom-cli inspect --tag PatientName --tag StudyInstanceUID study/image.dcm
dicom-cli validate --strict --json --output validation.json study/image.dcm
```

`inspect`、`validate` 和 `edit` 只接收单个常规文件。`validate --strict` 会把
warning 也作为校验失败处理。

### 创建配置并连接 PACS

```sh
dicom-cli config init dicom-cli.yaml
dicom-cli config target update local-pacs \
  --host pacs.example.test --port 11112 \
  --calling-ae DICOMCLI --called-ae PACS
dicom-cli echo --config dicom-cli.yaml --target local-pacs
```

`config init` 写入包含 `local-pacs` 示例目标的起始配置。生产环境应替换示例主机
和 AE Title；首版 DIMSE 连接仅使用明文 TCP。

### 脱敏目录

```sh
dicom-cli anonymize --profile basic --recursive --output anonymized study
```

源文件不会被改写。目录默认只扫描当前层，使用 `--recursive` 才遍历子目录。默认
profile 为内置 `basic`；命名 profile、筛选条件和模板来自规则文件。

### 导出与转码

```sh
dicom-cli convert image --input study/image.dcm --format png --output image.png
dicom-cli convert image --input study/multiframe.dcm --all-frames --output frames
dicom-cli transcode formats
dicom-cli transcode --input study/image.dcm --to rle --output compressed.dcm
dicom-cli transcode --input study/image.dcm --to jpeg2000-lossless --output j2k-lossless.dcm
dicom-cli transcode --input study --recursive --to rle --output compressed-study
```

`convert image` 的 `--frame` 从 1 开始计数，省略时导出首帧。转码前应先用
`transcode formats` 确认目标别名或 UID 在当前二进制中可用。`transcode` 使用
`--input/-i` 明确指定一个输入路径：输入是文件时 `--output` 为新 `.dcm` 文件，输入
是目录时 `--output` 为输出目录。旧的尾部位置参数仍可用，但不能与 `--input` 同时
传入；不支持多个输入文件或 stdin 路径清单。

例如 JPEG 2000 Lossless 可写为 `--to "JPEG 2000 Lossless"`、
`--to jpeg2000-lossless` 或 `--to 1.2.840.10008.1.2.4.90`。完整、随二进制变化的清单会直接显示在
`dicom-cli transcode --help` 和 `dicom-cli transcode formats` 中；两处均按
“标准名称 / --to 短名称 / UID”显示。

### 发送实例

```sh
dicom-cli send --config dicom-cli.yaml --target local-pacs --recursive study
Get-Content failed-paths.txt | dicom-cli send --target local-pacs -
```

`send` 会尽量复用 Association；进度写入 stderr，汇总写入 stdout。目标不接受
源传输语法时，它不会自动转码，应先使用 `transcode`。

## 命令导航

| 命令 | 适用场景 |
| --- | --- |
| `inspect <file>` | 输出单个文件的摘要、指定标签或完整元素清单。 |
| `validate <file>` | 执行内置和命名规则 profile 校验。 |
| `edit <file>` | 写出经标签、UID 或字符集修改的新文件。 |
| `anonymize <file-or-directory>` | 按内置或命名 profile 脱敏。 |
| `convert image|json --input <path>` | 导出图像帧或 DICOM 元数据。 |
| `encapsulate image <input>` | 将受支持的 PNG/JPEG 写为未压缩 Secondary Capture DICOM。 |
| `transcode --input <path>` | 将一个 DICOM 文件或一个目录转码至指定传输语法。 |
| `transcode formats` | 列出当前二进制实际注册的编解码能力。 |
| `echo` / `send <input>` | 运行 C-ECHO 或 C-STORE。 |
| `config` / `rules` | 生成、校验和维护运行配置、目标与规则。 |
| `lang <en|zh-CN>` | 持久设置文本帮助、结果和诊断的显示语言。 |
| `completion <shell>` | 输出 shell 自动补全脚本。 |

[使用手册](docs/usage.md) 说明所有参数、配置发现优先级、文件/目录输出规则、DIMSE
超时和批处理行为。每个版本的最终命令契约以 `dicom-cli <命令> --help` 为准。

## 语言、输出和退出码

运行配置的 `language` 支持 `en`（默认）和 `zh-CN`。`lang` 与
`config language` 会保存所选语言；当没有发现配置文件时，`lang` 会在当前目录
创建 `dicom-cli.yaml`。命令名、flag、JSON 字段和值、规则 DSL、DICOM Tag/UID 和
退出码始终保持不变。

业务结果通常写入 stdout，日志和 `send` 进度写入 stderr。二进制输出到 stdout 的
模式只允许一个结果；完整约束见使用手册。

| 退出码 | 含义 |
| --- | --- |
| `0` | 命令成功完成。 |
| `1` | 文件处理、批处理、网络或 DIMSE 操作失败。 |
| `2` | 命令参数、配置或规则输入无效。 |
| `3` | DICOM 校验失败；`validate --strict` 也会因 warning 返回此代码。 |

## 兼容性边界

CI 和归档 smoke test 使用合成 fixture。真实 DICOM、真实 PACS 和外部 codec 的
互操作须在受控环境中单独验收。`transcode formats` 中标记为 experimental 的 HTJ2K
不能视为已完成生产互操作验证。
