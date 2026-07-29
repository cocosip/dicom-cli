# dicom-cli 讨论稿

## 目标

开发一个使用 Go 的 DICOM CLI，同时服务开发测试和 PACS/影像运维场景。第一期聚焦本地 DICOM 文件处理与 DIMSE SCU 通信；不把批量任务平台作为独立功能。

## 已确认范围

### 本地文件命令

- `inspect`：查看标签、序列及像素数据摘要。
- `validate`：检查文件完整性、传输语法与基础一致性。
- `anonymize`：按预设或规则脱敏并生成处理结果。
- `edit`：按命令行参数对单个 DICOM 文件的 Tag 执行设置/替换、清空、删除，以及序列内嵌套 Tag 修改，并生成新文件。

嵌套 Tag 路径同时支持关键字和十六进制写法，序列项用 `[索引]` 选择，例如 `ContentSequence[0].TextValue` 或 `0040,A730[0].0040,A160`。

`edit --set` 对标准 Tag 可依据 DICOM 字典自动新增并使用对应 VR；私有或字典外 Tag 必须由对应命令参数显式声明 VR。

`edit` 默认只修改指定路径，不隐式维护 UID 引用关系；显式启用 UID 重映射模式时，复用 `anonymize` 的映射规则和单次调用内的一致性保证。

`edit` 支持为指定 UID Tag 自动生成合法的新 UID；固定 Tag 值仍通过普通设置参数传入，UID 生成使用专用参数表达。需要同步单文件内 UID 引用关系时，显式启用 UID 重映射模式。

UID 自动生成默认使用 `2.25.<UUID 对应十进制整数>` 形式；配置中声明机构 UID 根时可按该根生成，命令行可显式选择生成方式。

文本 Tag 默认按源文件的 `(0008,0005) SpecificCharacterSet` 解码和写回。`--charset` 指定输出字符集，并更新 `(0008,0005)`、重新编码全部文本 VR；`--input-charset` 用于源文件缺失或错误声明字符集时的强制读取。

- `convert`：DICOM 像素数据导出 PNG/JPEG、元数据导出 JSON，以及图片封装 DICOM。
- `transcode`：DICOM 在未压缩或压缩传输语法之间重新编码。

### 网络命令

第一期作为 DIMSE SCU，只实现：

- `echo`：DICOM C-ECHO 连通性验证。
- `send`：DICOM C-STORE 图像发送。

`send` 和 `echo` 同时支持命名目标与命令行覆盖：连接信息可由配置文件中的目标名称提供，也可由本次调用的地址、端口或 AE Title 参数覆盖。

运行配置支持 YAML 与 JSON，按文件扩展名识别格式，仅定义命名 PACS 目标、UID 根和运行参数。

规则文件与运行配置分离，集中定义标签清单、目录筛选、脱敏规则、校验规则和图片封装模板。`inspect` 与 `validate` 可使用规则文件中的默认 profile；`anonymize` 必须显式通过 `--profile/-p` 选择命名 profile，避免误修改原始数据。

`anonymize`、`convert`、`transcode` 和 `send` 的目录模式支持从规则文件中选择命名的 DICOM Tag 筛选条件；筛选支持 Tag 存在性、精确值、正则、数值范围及 `and` / `or` 组合。筛选只决定目录中的哪些文件进入处理，单文件命令不提供此能力。

规则文件可在顶层定义可复用的命名 `filters`，目录命令的 profile 也可内嵌专用筛选条件。

规则文件中的目录筛选、脱敏 profile 和自定义校验共用同一套 Tag 路径与条件表达式，避免用户学习多种规则写法。

配置可通过 `--config <文件>` 显式指定；未指定时，优先使用当前目录的 `dicom-cli.yaml`，仅当其不存在才回退到 Windows 或 Linux 用户级系统配置目录中的同名文件；两层文件不合并。

规则文件可通过 `--rules <文件>` 显式指定；未指定时，优先使用当前目录的 `dicom-cli-rules.yaml`，仅当其不存在才回退到 Windows 或 Linux 用户级系统配置目录中的同名文件；两层文件不合并。

配置文件可由用户直接维护，也提供 CLI 辅助管理命令：`config init` 生成运行配置示例，以及命名 PACS 目标的查看、添加、更新和删除。`rules init` 生成规则文件示例；两个初始化命令默认生成 YAML，传 `--format json` 可生成 JSON，且默认不覆盖已有文件，除非显式传覆盖参数。

`config validate` 独立校验运行配置文件。`rules validate` 独立校验规则文件的 schema、命名 profile、规则动作、参数类型和 Tag 路径；两者都不读取或修改 DICOM 文件。

支持受控的环境变量集合覆盖常用连接项，例如 `DICOM_CLI_CONFIG`、`DICOM_CLI_TARGET`、`DICOM_CLI_HOST`、`DICOM_CLI_PORT`、`DICOM_CLI_CALLING_AE` 和 `DICOM_CLI_CALLED_AE`。优先级为：命令行参数、环境变量、当前目录配置、用户级配置、内置默认值。

`send` 接受单文件、目录或标准输入中的文件路径列表。目录默认只扫描当前层；只有 `--recursive` 才扫描子目录。

同一目标的多实例发送默认复用一个 DICOM Association，并在其中连续发起多个 C-STORE 请求。提供参数限制单个 Association 最多发送的实例数，用于控制长连接范围。

默认只使用一个 Association 顺序发送；仅当 `--concurrency` 显式大于 1 时，才并行建立多个 Association。

发送只重试网络错误、超时和 Association 中断。PACS 已对实例返回失败状态时不重试。

PACS 不接受源文件传输语法时，`send` 不做隐式转码，明确报告该文件发送失败；用户需先执行 `transcode`。

`send` 在控制台输出进度和汇总，`--report <文件>` 可输出逐实例详细结果；失败实例可写入路径清单文件，并能作为下一次 `send` 的标准输入。

第一期 DIMSE 使用明文 TCP；TLS 不在首版范围内。

网络超时采用阶段与空闲模型：连接超时 `10s`、Association 协商超时 `30s`、网络读写空闲超时 `5m`。每有数据传输则重置空闲超时；三个值都可由 PACS 目标配置和命令行参数覆盖。

命名 PACS 目标配置预留可选的 TLS、代理和认证字段；第一期只解析并保留这些字段，不启用相应连接行为。

暂不在第一期实现 SCP 接收端、C-FIND、C-GET、C-MOVE、DICOMweb、独立的批量任务系统，以及 DICOM Digital Signatures 的检测、删除或重签。

## 命令输出约定

不同命令的业务结果不强行统一为一种数据格式：

- 查询、校验、发送等命令可输出面向人的控制台结果，也可提供各自定义的机器可读报告。
- 脱敏和转换等命令直接产生 DICOM 或图片文件/目录，控制台只报告处理状态。
- 文件型结果支持 `--output <路径>` 写入指定文件或目录。
- 支持 `--output -` 将 DICOM 或图片二进制写入标准输出，便于管道编排。
- 当标准输出承载二进制内容时，进度、日志、警告和错误详情只能写入标准错误。

`--output -` 只允许恰好生成一个二进制结果；目录输入或全量多帧导出产生多个结果时必须输出到目录。

退出码约定：`0` 为成功，`1` 为文件 I/O、编码、网络或 PACS 操作失败，`2` 为命令行参数或配置错误，`3` 为 `validate` 发现 `error` 或严格模式将 `warning` 视为失败。

## 参数约定

命令参数同时提供长选项与短选项；短选项在全局和同一命令内保持语义一致。常用映射包括：`--recursive/-r`、`--output/-o`、`--config/-c`、`--rules/-R`、`--target/-t`、`--profile/-p`、`--json/-j`、`--quiet/-q` 和 `--verbose/-v`。

日志默认级别为 `info`；`-v/--verbose` 切换为 `debug`，`-q/--quiet` 仅保留 `error`。`--log-format json` 切换结构化日志，所有日志统一写入标准错误。

## 运行配置与规则文件

```yaml
# dicom-cli.yaml
version: v1
uid:                         # 可选：机构 UID 根
targets:                     # 命名 PACS 目标
```

```yaml
# dicom-cli-rules.yaml
version: v1
filters:                     # 可复用的目录筛选条件
inspect:
  profiles:                  # 命名标签清单
anonymize:
  profiles:                  # 命名脱敏规则，可引用或内嵌目录筛选
validate:
  profiles:                  # 命名校验规则
dicom_templates:             # 图片封装 DICOM 的元数据模板
```

`edit` 是单文件直接参数操作，不读取规则 profile。`convert`、`transcode` 和 `send` 的目录模式可通过规则文件中的命名 `filters` 选择输入筛选条件。

```yaml
inspect:
  profiles:
    summary:
      tags: [PatientName, PatientID, StudyInstanceUID]

filters:
  ct-only:
    all:
      - path: Modality
        equals: CT

anonymize:
  profiles:
    default:
      filter: ct-only
      rules:
        - path: PatientName
          action: replace
          value: ANON
        - path: PatientID
          action: clear
        - path: StudyInstanceUID
          action: remap_uid

validate:
  profiles:
    ct-check:
      rules:
        - when:
            path: Modality
            equals: CT
          assert:
            path: SliceThickness
            exists: true
          severity: error
          message: CT 图像必须存在 SliceThickness

dicom_templates:
  secondary-capture:
    tags:
      Modality: OT
      StudyInstanceUID: auto
      SeriesInstanceUID: auto
```

所有规则通过 `path` 表示 DICOM Tag 路径，条件复用存在性、精确值、正则、数值范围与 `and` / `or`；`auto` 表示按已确认的 UID 根策略自动生成合法 UID。

`inspect` 默认输出文件、患者、检查、序列和像素数据摘要；`--all` 展开完整数据集，`--tag` 支持按关键字或十六进制 Tag 精确筛选。它支持 `--output <文件>` 保存查询结果，并支持从统一规则文件选择可复用的标签清单。默认保存可读文本，`--json` 时保存结构化 JSON。

输入能力按命令定义，不以统一实现为由强制所有命令支持目录。`inspect`、`validate` 和 `edit` 只接受单个 DICOM 文件。`anonymize`、`convert` 和 `transcode` 支持单文件和目录；前者用目录保证一次脱敏的 UID 映射覆盖整批输入，后两者用于批量生成新文件。三个命令默认只处理目录当前层，显式 `--recursive` 才遍历子目录。`send` 支持单文件、目录、显式 `--recursive` 和标准输入中的文件路径列表。

目录模式中，筛选条件未命中的文件正常跳过，不计为失败；非 DICOM 或损坏文件记录为该文件失败，但继续处理其余文件并最终返回退出码 `1`。`anonymize`、`convert`、`transcode` 和 `send` 支持 `--fail-fast`，可在首个失败文件时停止。

目录命令的汇总与 JSON 报告包含 `scanned`、`processed`、`skipped`、`failed`；每个跳过项记录筛选未命中的原因。

`--recursive/-r` 不跟随目录符号链接或 Windows junction，避免循环遍历与越界扫描；符号链接文件按普通文件处理。

## 脱敏规则

- 统一规则文件带顶层 schema 版本字段，例如 `version: v1`。
- 提供 DICOM Basic Application Level Confidentiality Profile 及其标准可选项作为内置命名预设；统一规则文件中的显式 Tag 规则在预设之上叠加。
- UID 是否重生成由命令参数或规则显式控制，不作为固定默认行为。
- 脱敏动作仅包括删除、清空、固定替换和 UID 重映射。
- 启用 UID 重生成时，每次 `anonymize` 调用创建新的 UID 映射表。
- 映射表的作用域覆盖该次调用的全部输入：同一检查的 `StudyInstanceUID`、同一序列的 `SeriesInstanceUID`、对象引用中的 UID 必须映射为一致的新值。
- 不同的原 Study、Series 或 SOP UID 必须映射为不同的新 UID，不能将目录内多个原始组强制合并。
- 不同调用之间不复用映射，因此再次脱敏同一份输入会产生新的 UID 集合。
- 私有标签默认保留；内置预设或用户 YAML/JSON 规则决定特定标签的删除、清空、替换或保留。
- 默认报告只输出不含敏感原值的摘要。显式指定 `--report <文件>` 时，报告记录每个处理 Tag 的原值、新值和完整 UID 映射。完整报告属于高度敏感数据，必须作为受控文件单独输出，不能混入普通控制台日志。
- `rules validate` 拒绝同一脱敏 profile 对同一 Tag 路径定义多条动作。

## 转换范围

第一版支持：

- 从 DICOM 导出像素数据为 PNG 或 JPEG。
- 从 DICOM 导出元数据为 JSON，直接使用 `go-dicom` 已支持的 JSON 序列化能力；默认仅输出 `PixelData` 摘要，显式 `--include-pixel-data` 时才以 Base64 输出原始像素内容。
- 将图片封装为 DICOM：同时支持 JSON/YAML 元数据模板与参考 DICOM 模板，命令行参数可覆盖指定字段。
- DICOM 传输语法转码：将输入 DICOM 从一种传输语法转换为另一种传输语法，目标可以是未压缩语法或压缩语法。

图片封装 DICOM 支持 8 位灰度、8 位 RGB 的 PNG/JPEG，以及 16 位灰度 PNG；TIFF、BMP 等输入格式不在第一版范围内。

图片封装 DICOM 默认使用 Explicit VR Little Endian；`--transfer-syntax` 可指定 `go-dicom-codecs` 支持的别名或 UID。

转码需要按目标传输语法重新编码像素数据，并更新文件元信息中的传输语法；除必要的编码相关元数据外，原数据集应保持不变。压缩传输语法以 `go-dicom-codecs` 实际提供的编码器和解码器为准；第一期同时覆盖 DICOM 的原始未压缩编码。实现前需通过 PoC 生成实际支持语法清单。

原始未压缩传输语法包含 Implicit VR Little Endian、Explicit VR Little Endian，以及已退休但需兼容旧设备的 Explicit VR Big Endian。

`transcode --to` 同时接受可读别名与标准传输语法 UID，例如 `--to jpeg-ls` 或 `--to 1.2.840.10008.1.2.4.80`。

`transcode formats` 列出当前版本可解码和可编码的传输语法、别名与 UID；清单按 `go-dicom-codecs` 的实际能力生成。

已按本机 `go-dicom` 与 `go-dicom-codecs` 源码及 codec 包测试验证的传输语法如下：

- 原始：Implicit VR Little Endian (`1.2.840.10008.1.2`)、Explicit VR Little Endian (`1.2.840.10008.1.2.1`)、Explicit VR Big Endian (`1.2.840.10008.1.2.2`，已退休)。
- RLE：RLE Lossless (`1.2.840.10008.1.2.5`)。
- JPEG：Baseline 8-bit (`1.2.840.10008.1.2.4.50`)、Extended 8/12-bit (`1.2.840.10008.1.2.4.51`)、Lossless Process 14 (`1.2.840.10008.1.2.4.57`)、Lossless SV1 (`1.2.840.10008.1.2.4.70`)。
- JPEG-LS：Lossless (`1.2.840.10008.1.2.4.80`)、Near-Lossless (`1.2.840.10008.1.2.4.81`)。
- JPEG 2000：Lossless (`1.2.840.10008.1.2.4.90`)、Lossy (`1.2.840.10008.1.2.4.91`)、Multi-component Lossless (`1.2.840.10008.1.2.4.92`)、Multi-component (`1.2.840.10008.1.2.4.93`)。
- HTJ2K：Lossless (`1.2.840.10008.1.2.4.201`)、RPCL Lossless (`1.2.840.10008.1.2.4.202`)、Lossy/Lossless (`1.2.840.10008.1.2.4.203`)；库将其标为实验性，`transcode formats` 必须标记该状态。

CLI 编译时导入相应 codec 注册包，`transcode formats` 只列出当前二进制实际注册的编解码器。

转码允许所有已支持的目标传输语法。任何会产出 DICOM 或图片的命令不得原地修改输入：输出必须写入新文件、新目录或二进制标准输出，并拒绝与任一输入相同的输出路径。

文件型命令未指定 `--output` 时，默认写入当前目录下按命令命名的输出目录；`--output` 可覆盖该位置。

输出路径发生同名冲突时，自动追加序号生成新文件名，不覆盖已有结果或输入文件。

目录输入时，`anonymize`、`convert` 和 `transcode` 默认保留输入目录的相对层级；可通过参数改为平铺输出，平铺时仍自动追加序号处理重名。

图片封装时，自动生成 UID，并按 Secondary Capture 的内置默认值补全可推断字段；合并模板和命令行参数后仍无法推断的必填字段必须报错。

图片目录封装 DICOM 时，模板未给出 Study/Series UID 则本次命令的全部输入图片共享一个新 Study UID 和一个新 Series UID；每张图片生成独立的 SOP Instance UID。模板显式给出 UID 时按模板使用。

多帧 DICOM 导出图片时默认导出第一帧，`--frame` 选择指定帧，`--all-frames` 分别导出全部帧。

`convert image` 是像素数据导出，不是诊断显示渲染：不应用窗宽窗位、Modality LUT、Rescale 或 Presentation LUT，也不提供 `--window`。它只解码传输语法，并按目标 PNG/JPEG 格式写出像素样本。

源 DICOM 为高位灰度而目标为 JPEG 时，按源像素样本范围线性缩放为 8 位；导出 PNG 则保留可用的原始位深。

`convert` 同时支持子命令与 `--to` 两种入口，例如 `convert image input.dcm --format png` 和 `convert input.dcm --to png`；两种写法进入同一转换实现。

## 校验范围

`validate` 同时覆盖：

- DICOM 解析、文件元信息和传输语法一致性。
- VR/VM、必填标签和基础值合法性。
- 按 SOP Class/IOD 的标准合规性校验。
- 用户提供的 JSON/YAML 自定义校验规则。

每条结果需要保留规则来源、对象路径、严重级别和可读说明。

`validate` 必须完整执行所有适用规则并列出单个文件的全部问题，不能在首个 Tag 错误时提前返回；全部结果汇总后再按退出码约定结束。

`validate` 默认输出可读问题列表；`--json/-j` 输出结构化结果，`--output/-o <文件>` 保存相应格式的报告。

CLI 内置版本化的 DICOM 标准规则数据作为默认来源。机构自定义规则仅从统一规则文件的 `validate.profiles` 读取，并在内置标准规则之上叠加；不加载第二份外部规则文件。

即使未提供外部配置，`validate` 也执行内置标准规则并输出校验结果。

结果分为 `info`、`warning` 和 `error`。默认仅 `error` 使命令以非零退出码结束；严格模式参数可将 `warning` 也视为失败。

## 运行与依赖约束

- 语言：Go 1.26。
- 平台：Windows 与 Linux；macOS 不作为保证目标。
- CLI 框架：`github.com/spf13/cobra`。
- 配置：`github.com/spf13/viper` 负责运行配置与规则文件的加载、YAML/JSON、默认路径和环境变量覆盖；CLI 在读取后执行严格 schema 校验。
- 日志：Go 标准库 `log/slog`，支持人可读与 JSON 结构化输出。
- DICOM 处理：`github.com/cocosip/go-dicom`。
- DICOM 编解码：`github.com/cocosip/go-dicom-codecs`。
- 发布：Windows `.zip`、Linux `.tar.gz` 二进制包，以及 `go install`；首版不提供包管理器和容器镜像。
- 测试：合成样本、后续提供的脱敏真实样本和真实 PACS 集成环境三层覆盖；外部样本路径及 PACS 地址不写死，后续通过测试配置或环境变量提供。

## 已确认架构

采用“共享核心与薄命令层”。命令层只处理参数、标准输入输出和退出码；可测试的内部包承载 DICOM 文件处理、转换、脱敏、DIMSE SCU 和输出适配。

这样每个命令可以返回其业务所需的文本、报告、DICOM 或图片，同时复用连接、错误处理和二进制管道规则；后续增加更多 DIMSE 能力也不会耦合到命令解析层。

## 拟定命令树

```text
dicom-cli
  inspect <file>                         # 单文件标签查看
  validate <file>                        # 单文件标准与自定义规则校验
  anonymize <file-or-directory>          # 标签脱敏
  edit <file>                            # 单文件 Tag 修改
  convert image <file-or-directory>      # DICOM 导出 PNG/JPEG
  convert json <file-or-directory>       # DICOM 元数据导出 JSON
  convert dicom <file-or-directory>      # 图片封装 DICOM
  convert <input> --to <format>          # 与以上转换入口等价
  transcode <file-or-directory> --to ... # 传输语法转换
  transcode formats                       # 列出可用传输语法
  echo                                   # C-ECHO SCU
  send <file-or-directory>               # C-STORE SCU
  config init                            # 创建运行配置示例
  config validate                        # 校验运行配置文件
  config target list|add|update|remove   # 管理命名 PACS 目标
  rules init                             # 创建规则文件示例
  rules validate                         # 校验统一规则文件
```

`anonymize`、`convert` 和 `transcode` 的目录模式默认只扫描当前层，`--recursive` 才遍历子目录。`send` 还接受标准输入中的文件路径列表。

## 推荐的包边界

```text
cmd/                 CLI 参数、stdout/stderr、退出码
internal/app/        inspect、validate、anonymize、edit、convert、transcode、send 用例
internal/dicom/      go-dicom / go-dicom-codecs 适配与文件能力
internal/dimse/      C-ECHO、C-STORE SCU、Association 生命周期
internal/output/     各命令的报告与二进制输出处理
```
