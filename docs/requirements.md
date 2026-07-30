# dicom-cli 需求规格说明书

## 1. 文档目的

本文档把 [讨论稿](dicom-cli-discussion.md) 中已经确认的产品决定整理为第一期开发的可验收需求。讨论稿保留决策背景和细节；本文作为开发、测试和验收的范围基线。

## 2. 产品目标与范围

`dicom-cli` 是面向开发测试、PACS 集成和影像运维的本地命令行工具。第一期提供 DICOM 文件查看、校验、脱敏、Tag 修改、格式转换、传输语法转码，以及 DIMSE SCU 的 C-ECHO 和 C-STORE。

第一期不包含 SCP、C-FIND、C-GET、C-MOVE、DICOMweb、独立批量任务系统、TLS 实际连接能力和 DICOM Digital Signatures 处理。

## 3. 技术与运行约束

| 项目 | 要求 |
| --- | --- |
| 开发语言 | Go 1.26 |
| 支持平台 | Windows、Linux |
| CLI | `github.com/spf13/cobra` |
| 配置加载 | `github.com/spf13/viper`，支持 YAML、JSON 和受控环境变量 |
| 日志 | 标准库 `log/slog` |
| DICOM 库 | `github.com/cocosip/go-dicom` |
| 编解码库 | `github.com/cocosip/go-dicom-codecs` |
| 发布方式 | Windows ZIP、Linux tar.gz、`go install` |

架构采用“共享核心与薄命令层”：`cmd` 只负责参数、标准输入输出和退出码；`internal` 包实现可测试的业务用例、DICOM 适配、DIMSE SCU 和输出。

## 4. 命令与输入范围

| 命令 | 输入 | 主要结果 |
| --- | --- | --- |
| `inspect <file>` | 单个 DICOM 文件 | 可读 Tag 摘要或 JSON 报告 |
| `validate <file>` | 单个 DICOM 文件 | 全量校验问题报告 |
| `anonymize <path>` | 单文件或目录 | 新 DICOM 文件及处理摘要 |
| `edit <file>` | 单个 DICOM 文件 | 修改后的新 DICOM 文件 |
| `convert image <path>` | 单文件或目录 DICOM | PNG/JPEG |
| `convert json <path>` | 单文件或目录 DICOM | JSON 元数据 |
| `encapsulate image <path>` | 单文件或目录 PNG/JPEG | 未压缩 Secondary Capture DICOM |
| `transcode <path> --to <syntax>` | 单文件或目录 DICOM | 新传输语法的 DICOM |
| `transcode formats` | 无 | 当前二进制实际注册的语法清单 |
| `echo` | 目标配置或命令行连接参数 | C-ECHO 结果 |
| `send <path>` | 单文件、目录或标准输入路径列表 | C-STORE 进度、汇总和可选详细报告 |
| `config ...` | 运行配置文件 | 配置初始化、校验和目标管理 |
| `rules ...` | 规则文件 | 规则初始化和校验 |

目录处理命令默认只扫描当前层，`--recursive/-r` 才遍历子目录。遍历不跟随目录符号链接或 Windows junction；符号链接文件可作为普通文件处理。筛选未命中应记为跳过，非 DICOM 或损坏文件应记为失败但继续处理，除非指定 `--fail-fast`。

## 5. 全局 CLI、输出与退出码

所有短选项必须在全局和适用命令中保持一致：`--recursive/-r`、`--output/-o`、`--config/-c`、`--rules/-R`、`--target/-t`、`--profile/-p`、`--json/-j`、`--quiet/-q`、`--verbose/-v`。

`--output -` 仅允许单个二进制 DICOM 或图片结果写入标准输出；目录和多帧全部导出必须写入目录。二进制输出时，日志、进度、诊断和错误详情必须写到标准错误。

文件型命令不得原地修改输入，且必须拒绝与输入相同的输出路径。默认输出至当前目录中与命令对应的目录；目录处理默认保留相对层级。输出重名时自动追加序号，不覆盖已有文件或输入文件。

| 退出码 | 含义 |
| --- | --- |
| `0` | 成功 |
| `1` | 文件 I/O、编码、网络、PACS 或批量中的单文件处理失败 |
| `2` | 参数、配置或规则错误 |
| `3` | `validate` 发现 error，或严格模式将 warning 视为失败 |

日志默认级别为 `info`，`-v` 为 `debug`，`-q` 仅输出 `error`；`--log-format json` 输出结构化日志，日志始终写入标准错误。

## 6. 配置与规则

### 6.1 运行配置

默认文件名为 `dicom-cli.yaml`，保存命名 PACS 目标、机构 UID 根和运行参数。查找优先级为 `--config/-c`、当前目录文件、用户级配置目录文件、内置默认值；当前目录与用户级文件不合并。常用环境变量包括 `DICOM_CLI_CONFIG`、`DICOM_CLI_TARGET`、`DICOM_CLI_HOST`、`DICOM_CLI_PORT`、`DICOM_CLI_CALLING_AE` 和 `DICOM_CLI_CALLED_AE`。

提供 `config init`、`config validate`、`config target list|add|update|remove`。初始化默认写 YAML，`--format json` 写 JSON，且没有显式覆盖选项时不得覆盖现有文件。PACS 目标需预留 TLS、代理和认证字段，但首版只使用明文 TCP。

### 6.2 规则文件

默认文件名为 `dicom-cli-rules.yaml`，顶层必须含 `version: v1`。其统一保存 Tag 清单、目录筛选、脱敏 profile、校验 profile 和图片封装 DICOM 模板。查找与覆盖行为同运行配置，但使用 `--rules/-R`。

提供 `rules init` 和 `rules validate`。规则校验必须验证 schema、命名引用、Tag 路径、动作、参数类型和同一脱敏 profile 中的重复 Tag 动作。规则表达式统一使用 Tag 路径及条件 DSL，支持存在性、精确相等、正则、数值范围、`and` 和 `or`。

## 7. 本地 DICOM 功能

### 7.1 inspect

默认展示文件、患者、检查、序列、实例和像素数据摘要：包括患者出生日期/性别、检查日期时间/Accession Number/描述、序列号/描述/部位/协议、SOP Class 与实例 UID、空间位置与方向，以及像素间距、位深、光度解释和窗宽窗位。默认文本按上述分组输出，组内一行只展示一个字段。`--all` 展开完整数据集。`--tag` 支持 DICOM 关键字与十六进制 Tag，且可通过规则 profile 复用标签清单。默认输出可读文本，`--json` 输出同一结构化摘要，`--output/-o` 保存相应结果。

### 7.2 validate

必须完整收集单文件的所有问题，不能因第一个错误提前结束。校验范围包括解析、文件元信息与传输语法一致性、VR/VM、必填值、SOP Class/IOD 合规性以及用户校验规则。

内置版本化 DICOM 标准规则始终执行；规则文件的 `validate.profiles` 仅作为叠加。每项结果必须包含规则来源、对象路径、严重级别和可读信息。严重级别为 `info`、`warning`、`error`；默认仅 `error` 失败，严格模式下 `warning` 也失败。

### 7.3 anonymize

未指定 `--profile/-p` 时使用内置 DICOM Basic Application Level Confidentiality Profile；可通过该选项选择其他命名 profile，并通过 `--option` 选择标准可选项。规则文件路径使用 `--rules/-R` 指定，外部 profile 在内置 Basic 规则之上叠加；若外部 profile 也命名为 `basic`，必须同时显式传入 `--rules` 与 `--profile basic`。动作仅限删除、清空、固定替换和 UID 重映射；私有 Tag 默认保留。

UID 映射仅作用于一次命令调用的全部输入：相同原 UID 始终映射为相同新 UID，不同原 UID 映射为不同新 UID，映射不跨调用持久化。默认报告不得包含敏感原值；只有 `--report <file>` 才可输出完整 Tag 前后值和 UID 映射。

### 7.4 edit

仅处理单文件，不能使用规则 profile。支持 `set/replace`、`clear`、`delete` 与嵌套序列路径，例如 `ContentSequence[0].TextValue` 或 `0040,A730[0].0040,A160`。标准缺失 Tag 可按字典自动补充 VR；私有或未知 Tag 必须显式提供 VR。

默认不维护 UID 引用。显式 UID 重映射模式应在文件内保持引用一致，并复用脱敏的 UID 映射策略。指定 UID Tag 可自动生成 `2.25.<UUID 十进制>` 形式的 UID，或按配置的机构 UID 根生成。`--charset` 是输出字符集，必须更新 `(0008,0005)` 并重编码全部文本 VR；`--input-charset` 用于纠正错误或缺失的源字符集声明。

## 8. 转换与转码

`convert image` 支持 DICOM 到 PNG/JPEG，默认导出第一帧，`--frame` 选择帧，`--all-frames` 导出所有帧。它是像素数据导出，不应用窗宽窗位、LUT 或 Rescale。高位灰度写 JPEG 时线性缩放到 8 位，写 PNG 时保留可用原始位深。

`convert json` 使用 `go-dicom` 的 JSON 序列化；默认仅输出 PixelData 摘要，`--include-pixel-data` 才 Base64 输出像素内容。

`encapsulate image` 支持 8 位灰度、8 位 RGB PNG/JPEG 和 16 位灰度 PNG。元数据来自规则模板或参考 DICOM，命令行可覆盖；默认封装为 Secondary Capture，固定使用 Explicit VR Little Endian，不提供压缩或传输语法参数。目录输入未显式提供 Study/Series UID 时，应在一次调用内共享一个新 Study UID 和一个新 Series UID，每图生成独立 SOP Instance UID。

`transcode` 将 DICOM 转换为指定传输语法并更新文件元信息。仅允许修改与编解码及传输语法相关的数据，其他数据集内容必须保持不变。`--to` 接受别名或 UID，且目标能力必须来自当前注册的 `go-dicom-codecs`。`transcode formats` 列出当前二进制实际可用的编码与解码能力，并把 HTJ2K 标为实验性。

首版兼容的原始传输语法为 Implicit VR Little Endian、Explicit VR Little Endian、Explicit VR Big Endian（已退休）。压缩语法以 `go-dicom-codecs` 注册结果为准，已确认包括 RLE、JPEG、JPEG-LS、JPEG 2000 和实验性 HTJ2K。

## 9. DIMSE SCU

`echo` 实现 C-ECHO，`send` 实现 C-STORE。连接参数可由命名目标提供，也可由命令行覆盖。首版只使用明文 TCP。

默认连接超时为 10 秒、Association 协商超时为 30 秒、读写空闲超时为 5 分钟；目标配置和命令行可覆盖。`send` 默认在一个 Association 中顺序发送多个实例，可配置单 Association 最大实例数；`--concurrency > 1` 时建立并行 Association。

仅对网络、超时或 Association 中断重试；PACS C-STORE 响应失败不得重试。发送不会自动转码，目标不接受源传输语法时必须明确失败。`send` 必须输出进度和汇总，支持详细 `--report`，并可把失败路径写为可被标准输入再次消费的清单。

## 10. 测试与验收

测试分三层：合成样本单元/集成测试、后续提供的脱敏真实 DICOM 样本、真实 PACS 环境集成测试。测试代码不得硬编码外部样本路径、PACS 地址或 AE Title；外部依赖统一通过测试配置或环境变量提供。

每个目录处理功能的验收必须覆盖：当前层与递归遍历、筛选跳过、损坏文件继续处理、`--fail-fast`、相对输出路径、输出重名和输入不被修改。网络功能还必须覆盖 Association 复用、重试分类和报告结果。
