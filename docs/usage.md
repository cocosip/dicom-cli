# dicom-cli 使用手册

本手册描述当前 `dicom-cli` 二进制的命令契约，面向交互使用和脚本集成。运行
`dicom-cli <命令> --help` 可以查看所用版本完全一致的帮助信息。

## 使用前先区分输入类型

| 输入类型 | 命令 | 规则 |
| --- | --- | --- |
| 单个 DICOM 文件 | `inspect`、`validate`、`edit` | 只接受一个常规文件，不能传目录。 |
| DICOM 文件或目录 | `anonymize`、`convert`、`transcode`、`send` | 目录默认仅扫描当前层；`--recursive` 才扫描子目录。 |
| 外部图片 | `encapsulate image` | 支持 8 位灰度、8 位 RGB PNG/JPEG 和 16 位灰度 PNG。 |
| 标准输入路径清单 | `send -` | 每行一个实例路径。 |

会写出 DICOM、图片或 JSON 的命令不会修改原始输入。若没有显式 `--output`，不同
命令会使用当前工作目录中的默认输出目录；每个命令的具体规则见对应小节。

## 全局参数与日志

所有业务命令都接受以下全局参数：

| 参数 | 说明 |
| --- | --- |
| `-c, --config <path>` | 指定运行配置文件。 |
| `-R, --rules <path>` | 指定规则文件。 |
| `-v, --verbose` | 将日志级别提高到 debug。 |
| `-q, --quiet` | 只输出 error 级别日志。 |
| `--log-format text|json` | 选择 stderr 日志格式，默认 `text`。 |

`--verbose` 与 `--quiet` 不能同时使用。业务结果写入 stdout，日志和 `send`
进度写入 stderr；调用脚本应按此分流。

帮助、文本结果、进度和本工具生成的诊断使用运行配置根字段 `language`：`en`
为默认英文，`zh-CN` 为简体中文。`lang <en|zh-CN>` 与
`config language <en|zh-CN>` 会保存语言设置；如果没有发现配置文件，前者会在
当前目录创建 `dicom-cli.yaml`。通过 `--config` 选择已有配置时，语言设置写回该文件。
命令名、flag 名、
JSON 字段和值、退出码、规则 DSL、DICOM Tag 和 UID 不随语言变化。配置不存在、无法读取或
`language` 非法时，诊断以英文输出。

运行配置文件按 `--config`、`DICOM_CLI_CONFIG`、当前目录
`dicom-cli.yaml`、用户配置目录 `dicom-cli.yaml`、内置默认值的顺序选择，
只会加载其中一个文件而不会合并多个文件。对于 `echo` 和 `send` 的目标字段，
命令行覆盖值优先于 `DICOM_CLI_TARGET`、`DICOM_CLI_HOST`、`DICOM_CLI_PORT`、
`DICOM_CLI_CALLING_AE`、`DICOM_CLI_CALLED_AE` 等环境变量，再优先于所选配置和默认值。

## 配置与规则

`config init [path]` 生成 YAML 配置；加 `--format json` 生成 JSON，已有文件只有
传 `--force` 才允许覆盖。生成的 YAML 含有可替换的 `local-pacs` 示例目标和默认
DIMSE 超时。`config validate [path]` 校验配置。

`lang <en|zh-CN>` 是语言切换的简写，`config language <en|zh-CN>` 是等效的完整
入口。语言命令可以在无配置时创建当前目录 `dicom-cli.yaml`；目标维护命令
`config target ...` 则要求已经存在配置文件，可通过 `--config` 或
`DICOM_CLI_CONFIG` 明确选择。

命名 PACS 目标通过以下命令维护：

```sh
dicom-cli config target add local-pacs \
  --host pacs.example.test --port 11112 \
  --calling-ae DICOMCLI --called-ae PACS
dicom-cli config target list
dicom-cli config target update local-pacs --host pacs.example.test
dicom-cli config target remove local-pacs
```

`config target add` 必须同时提供 `--host`、`--port`、`--calling-ae` 和
`--called-ae`。`update` 只更新实际传入的字段。可从
[`examples/dicom-cli.yaml`](../examples/dicom-cli.yaml) 复制完整示例。

`rules init [path]` 和 `rules validate [path]` 的覆盖与格式行为和 `config` 相同。
未传路径时，规则文件按 `--rules`、`DICOM_CLI_RULES`、当前目录
`dicom-cli-rules.yaml`、用户配置目录 `dicom-cli-rules.yaml` 的顺序查找。
规则文件包含 `filters`、`inspect.profiles`、`anonymize.profiles`、
`validate.profiles` 与 `dicom_templates`；未知字段会被拒绝。完整示例见
[`examples/dicom-cli-rules.yaml`](../examples/dicom-cli-rules.yaml)。

## inspect、validate 与 edit

### inspect

```sh
dicom-cli inspect image.dcm
dicom-cli inspect --tag PatientName --tag 0040,A730[0].0040,A160 image.dcm
dicom-cli inspect --profile summary --rules dicom-cli-rules.yaml image.dcm
dicom-cli inspect --all --json --output report.json image.dcm
```

`inspect` 只接受一个 DICOM 文件。默认输出患者、检查、序列和像素摘要；
`--all` 输出所有数据元素，`--tag` 可重复指定 DICOM 关键字或十六进制 Tag 路径，
`--profile` 从规则文件选择标签清单。`--json` 输出 JSON，`--output` 将报告写入文件。

### validate

```sh
dicom-cli validate image.dcm
dicom-cli validate --strict --json --output validation.json image.dcm
dicom-cli validate --profile ct-required-identifiers --rules dicom-cli-rules.yaml image.dcm
dicom-cli validate --charset-check image.dcm
```

`validate` 只接受一个 DICOM 文件。它会收集多个独立问题；默认 warning 不改变
退出码，`--strict` 会将 warning 也按校验失败处理。`--profile` 叠加规则文件的
命名校验 profile。`--charset-check` 是可选检测：它比较 `(0008,0005)` Specific
Character Set 声明与文本元素的原始字节，候选编码为声明值、`ISO_IR 192`（UTF-8）、
`GB18030` 和 `GBK`。能够确定声明编码无法解码而另一候选可解码时报告 error；声明
编码仍可解码、但多个文本元素一致地更符合另一候选时报告 warning；ASCII 或多个候选
同样合理时不报告问题。该命令不会修改 DICOM 文件或自动修正 Character Set，报告只包含
编码、置信度和 Tag 路径，不包含原始字节或文本值。配合 `--strict` 时，warning 也会以
退出码 `3` 表示校验失败。

### edit

```sh
dicom-cli edit image.dcm --set PatientName=ANON^PATIENT --output edited.dcm
dicom-cli edit image.dcm --clear PatientID --delete AccessionNumber --output edited.dcm
dicom-cli edit image.dcm --generate-uid StudyInstanceUID --uid-root 1.2.156.112618 --output edited.dcm
```

`edit` 只接受一个文件并始终写出新文件。`--set` 使用 `TagPath=value`，`--clear`、
`--delete` 和 `--generate-uid` 接受可重复的 TagPath；标准 Tag 自动推断 VR，私有
或未知 Tag 需要 `--vr TagPath=VR`。`--remap-uids` 重映射文件中的 UID 值。
`--charset` 设置输出字符集，`--input-charset` 覆盖输入字符集解释。输入路径不能
与输出路径相同，且至少需要一个编辑操作。未传 `--output` 时，结果写入当前目录下的
`edit` 子目录。

在 PowerShell 中，`--set` 的值包含空格时，必须为整个 `TagPath=value` 参数加引号，
不能只引用等号右侧的值；否则空格后的文本会被视为额外的位置参数。例如：

```powershell
dicom-cli edit D:\11.dcm --set "0008,1030=Study Description" --set 01f1,104e=xx
```

## anonymize

```sh
dicom-cli anonymize --profile basic --output anonymized image.dcm
dicom-cli anonymize --profile research --rules dicom-cli-rules.yaml --output anonymized study
dicom-cli anonymize --profile basic --recursive --filter ct-images --output anonymized study
```

`anonymize` 接受单文件或目录，目录默认只扫描当前层，`--recursive` 才进入子目录。
默认使用内置 `basic` profile；`--profile` 可选规则文件中的命名 profile，
`--option` 可重复指定标准 profile 选项。支持的选项为：

- `retain-safe-private`
- `retain-uids`
- `retain-device-identity`
- `retain-institution-identity`
- `retain-patient-characteristics`
- `retain-longitudinal-temporal-information-with-full-dates`
- `retain-longitudinal-temporal-information-with-modified-dates`
- `clean-descriptors`
- `clean-structured-content`
- `clean-graphics`

目录输出默认保留输入相对层级；`--flatten` 改为平铺并自动处理重名。`--filter` 从规则
文件选择命名筛选条件，可用于单文件和目录输入。`--fail-fast` 在首个文件失败时停止，
默认继续处理其余文件。未传 `--output` 时，结果写入当前目录下的 `anonymize` 子目录。
`--report` 写出可能包含敏感前后值和 UID 映射的详细报告，应妥善保护；`--json`
只输出汇总。单文件可以用 `--output -` 将二进制 DICOM 写到 stdout，多个二进制结果
不能写到 stdout。

## convert、encapsulate 与 transcode

### convert

```sh
dicom-cli convert image --input image.dcm --format png --output image.png
dicom-cli convert image --input image.dcm --all-frames --output frames
dicom-cli convert json --input image.dcm --output metadata.json
dicom-cli convert xml --input image.dcm --output metadata.xml
dicom-cli convert pixeldata --input image.dcm --output pixels.bin
```

四个子命令都必须通过 `--input/-i <path>` 提供一个 DICOM 文件或目录，不支持位置输入
参数。`convert image` 导出 PNG 或 JPEG。`--frame` 是从 1 开始的帧号；默认导出首帧，
`--all-frames` 导出每一帧。`convert json` 和 `convert xml` 都只导出元数据，不包含
PixelData。`convert pixeldata` 将原始帧载荷按顺序拼接为 `.bin` 文件；未压缩实例写出原始
采样字节，封装压缩实例写出压缩帧字节，不解码、不转码，也不包含 DICOM 元素头、Basic
Offset Table 或 fragment Item 头。

四个子命令都支持文件或目录输入、`--recursive`、`--flatten`、`--fail-fast` 和
`--output`。目录默认不递归；`convert image` 只处理 `.dcm` 文件，扩展名不匹配的
目录项会作为跳过项。图片二进制 stdout 要求恰好一个结果；多帧、多文件和目录输入
不得使用 `--output -`。未传 `--output` 时，结果写入当前目录下的 `convert` 子目录。

### encapsulate

```sh
dicom-cli encapsulate image --patient-name ANON^PATIENT --output image.dcm source.png
```

`encapsulate image <input>` 将 8 位灰度、8 位 RGB PNG/JPEG 或 16 位灰度 PNG
封装为 Secondary Capture DICOM。必须由 `--patient-name`、`--template` 或
`--reference` 提供 PatientName。输出固定为未压缩的 Explicit VR Little Endian；
不提供传输语法或压缩选项。目录模式支持 `--recursive`、`--flatten`、`--fail-fast` 和
`--output`，并在一次调用内共享新 Study/Series UID、为每张图片生成独立 SOP Instance UID。
单图输出路径必须使用 `.dcm` 扩展名；未传 `--output` 时，结果写入当前目录下的
`convert` 子目录。该命令不支持 TIFF、BMP、其他位深或压缩传输语法选择。

### transcode

```sh
dicom-cli transcode formats
dicom-cli transcode --input image.dcm --to rle --output compressed.dcm
dicom-cli transcode --input image.dcm --to jpeg2000-lossless --output j2k-lossless.dcm
dicom-cli transcode --input study --recursive --to 1.2.840.10008.1.2.1 --output output
```

`transcode formats` 显示当前二进制实际注册的传输语法、别名和编码/解码能力，
可加 `--json`。执行转码时只有 `--to` 必填；未传 `--output` 时，单文件结果写入
当前目录下的 `transcode/<原文件名>`，目录输入则写入 `transcode` 并保留相对层级。
`--to` 接受 `transcode formats` 输出的别名或标准传输语法 UID，例如 `--to rle` 或
`--to 1.2.840.10008.1.2.1`。

`--to` 可直接传入表格中的**标准名称**、**短名称**或 **UID**。例如下面三种写法
等价，均表示 JPEG 2000 Lossless：

```sh
dicom-cli transcode --input image.dcm --to "JPEG 2000 Lossless" --output output.dcm
dicom-cli transcode --input image.dcm --to jpeg2000-lossless --output output.dcm
dicom-cli transcode --input image.dcm --to 1.2.840.10008.1.2.4.90 --output output.dcm
```

当前二进制的 `--to` 完整清单如下。`transcode --help` 与 `transcode formats` 也会
显示同一份“标准名称 / --to 短名称 / UID”信息；若未来构建注册的 codec 不同，以命令
实际输出为准。

| 标准名称（可直接传给 `--to`） | `--to` 短名称 | 标准 UID | 状态 |
| --- | --- | --- | --- |
| Implicit VR Little Endian | `implicit-vr-little-endian` | `1.2.840.10008.1.2` | 可用 |
| Explicit VR Little Endian | `explicit-vr-little-endian` | `1.2.840.10008.1.2.1` | 可用 |
| Explicit VR Big Endian | `explicit-vr-big-endian` | `1.2.840.10008.1.2.2` | 可用 |
| RLE Lossless | `rle` | `1.2.840.10008.1.2.5` | 可用 |
| JPEG Baseline | `jpeg-baseline` | `1.2.840.10008.1.2.4.50` | 可用 |
| JPEG Extended | `jpeg-extended` | `1.2.840.10008.1.2.4.51` | 可用 |
| JPEG Lossless | `jpeg-lossless` | `1.2.840.10008.1.2.4.57` | 可用 |
| JPEG Lossless SV1 | `jpeg-lossless-sv1` | `1.2.840.10008.1.2.4.70` | 可用 |
| JPEG-LS Lossless | `jpeg-ls` | `1.2.840.10008.1.2.4.80` | 可用 |
| JPEG-LS Near-Lossless | `jpeg-ls-near-lossless` | `1.2.840.10008.1.2.4.81` | 可用 |
| JPEG 2000 Lossless | `jpeg2000-lossless` | `1.2.840.10008.1.2.4.90` | 可用 |
| JPEG 2000 | `jpeg2000` | `1.2.840.10008.1.2.4.91` | 可用 |
| JPEG 2000 Multicomponent Lossless | `jpeg2000-multicomponent-lossless` | `1.2.840.10008.1.2.4.92` | 可用 |
| JPEG 2000 Multicomponent | `jpeg2000-multicomponent` | `1.2.840.10008.1.2.4.93` | 可用 |
| High-Throughput JPEG 2000 Lossless | `htj2k-lossless` | `1.2.840.10008.1.2.4.201` | experimental |
| High-Throughput JPEG 2000 Lossless RPCL | `htj2k-lossless-rpcl` | `1.2.840.10008.1.2.4.202` | experimental |
| High-Throughput JPEG 2000 | `htj2k` | `1.2.840.10008.1.2.4.203` | experimental |

`transcode` 通过 **`--input/-i <path>`** 明确指定一个输入路径，不支持多个输入文件，
也不支持 `-` 或 stdin 路径清单。为兼容旧脚本，也可将一个输入路径放在命令末尾；
但不能与 `--input` 同时传入。输入类型决定 `--output` 的含义：

| 调用形式 | `<input>` | `--output` | 写出结果 |
| --- | --- | --- | --- |
| `transcode -i <file.dcm> --to <syntax> -o <file.dcm>` | 一个 DICOM 文件 | 一个尚不存在的目标文件路径 | 仅写出一个新 DICOM 文件。 |
| `transcode -i <directory> --to <syntax> -o <directory>` | 一个目录 | 输出根目录 | 当前层的每个可处理文件写到该目录。 |
| `transcode -i <directory> --recursive --to <syntax> -o <directory>` | 一个目录 | 输出根目录 | 递归扫描并保留输入相对目录结构。 |

目录模式支持 `--recursive`、`--flatten`、`--filter`、`--fail-fast`。默认不递归并保留
相对目录结构；`--flatten` 将所有结果放入输出根目录，名称冲突时自动追加序号。
`--filter` 仅用于目录输入。转码只改变与编解码和传输语法有关的数据；输出路径不得
是输入路径。

HTJ2K 显示为 experimental。能列出或完成合成样本转码不等于已验证真实样本互操作。

## echo 与 send

目标可由 `--target <name>` 从配置中选择，或由以下参数覆盖：`--host`、`--port`、
`--calling-ae`、`--called-ae`、`--connect-timeout`、`--associate-timeout` 和
`--idle-timeout`。首版只使用明文 TCP。默认连接超时为 10 秒、Association 协商超时
为 30 秒、读写空闲超时为 5 分钟。

```sh
dicom-cli echo --target local-pacs --config dicom-cli.yaml
dicom-cli echo --host pacs.example.test --port 11112 --calling-ae DICOMCLI --called-ae PACS
dicom-cli send --target local-pacs --config dicom-cli.yaml --recursive study
```

`echo` 执行一次 C-ECHO，`--json` 输出 JSON 结果。`send` 接受单文件、目录或 `-`；
输入为 `-` 时从 stdin 读取每行一个文件路径。`send` 的进度写 stderr，stdout 输出
汇总或 `--json` 汇总。`--report` 写详细 JSON，`--failed-list` 写失败路径清单，该清单
可作为下一次 `send -` 的 stdin。

`send` 默认顺序复用一个 Association。`--max-instances` 限制单个 Association 中的
实例数，`--concurrency` 控制并行 Association 数，`--retries` 只重试网络、超时或
Association 中断。PACS 返回的 C-STORE 状态失败不会重试。目标不接受源传输语法时，
`send` 不会隐式转码；请先执行 `transcode`。

## Shell 自动补全

`completion` 输出指定 shell 的补全脚本，支持 `bash`、`zsh`、`fish` 和 `powershell`。
例如可在 PowerShell 中执行：

```powershell
dicom-cli completion powershell > dicom-cli-completion.ps1
```

补全脚本的加载方式取决于所用 shell 的配置；生成脚本不会修改当前 shell 配置。

## 批处理、输出与退出码

`anonymize`、`convert`、`transcode` 和目录模式 `send` 中，规则筛选不匹配的文件为
正常跳过。目录扫描会依命令的输入限制跳过不适用的扩展名；被选中但无法解析或处理
的文件计为失败。默认批处理继续处理剩余文件，`--fail-fast` 会在首个失败后停止。

输出安全边界如下：

- `edit`、单文件 `anonymize` 和 `transcode` 拒绝把输出写回输入路径。
- `anonymize --output -` 仅允许一个选中的输入文件，汇总改写到 stderr。
- `convert image --output -` 仅允许一个图像结果；单文件输入时，`convert json --output -`、
  `convert xml --output -` 和 `convert pixeldata --output -` 可分别写入 stdout。
- `--report` 和 `send --failed-list` 写入文件，不接受 `-`；脱敏报告可能含有原始值和
  UID 映射，应按敏感数据处理。

| 退出码 | 含义 |
| --- | --- |
| `0` | 成功。 |
| `1` | 操作失败，包括批处理中的失败文件、网络和 DIMSE 失败。 |
| `2` | 参数、配置或规则输入不合法。 |
| `3` | DICOM 校验失败。 |

真实样本/PACS 验收不在默认命令或 CI 中执行。运行受控外部验收前，应确认样本可合法
使用、PACS 目标已获授权，并将地址、AE Title 和凭据保留在部署环境而非仓库中。
