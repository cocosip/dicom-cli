# dicom-cli 使用手册

本手册描述当前 `dicom-cli` 二进制的命令契约。运行 `dicom-cli <命令> --help`
可以查看与所用版本完全一致的帮助信息。

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
为默认英文，`zh-CN` 为简体中文。语言随所选配置文件切换，`--config` 仍只负责
选择配置文件；命令名、flag 名、JSON 字段和值、退出码、规则 DSL、DICOM Tag 和 UID
不随语言变化。配置不存在、无法读取或 `language` 非法时，诊断以英文输出。

运行配置文件按 `--config`、`DICOM_CLI_CONFIG`、当前目录
`dicom-cli.yaml`、用户配置目录 `dicom-cli.yaml`、内置默认值的顺序选择，
只会加载其中一个文件而不会合并多个文件。对于 `echo` 和 `send` 的目标字段，
命令行覆盖值优先于 `DICOM_CLI_TARGET`、`DICOM_CLI_HOST`、`DICOM_CLI_PORT`、
`DICOM_CLI_CALLING_AE`、`DICOM_CLI_CALLED_AE` 等环境变量，再优先于所选配置和默认值。

## 配置与规则

`config init [path]` 生成 YAML 配置；加 `--format json` 生成 JSON，已有文件只有
传 `--force` 才允许覆盖。`config validate [path]` 校验配置。目标维护命令需要已存在的
配置文件；可通过 `--config` 或 `DICOM_CLI_CONFIG` 显式指定。

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
```

`validate` 只接受一个 DICOM 文件。它会收集多个独立问题；默认 warning 不改变
退出码，`--strict` 会将 warning 也按校验失败处理。`--profile` 叠加规则文件的
命名校验 profile。

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
dicom-cli convert image --format png --output image.png image.dcm
dicom-cli convert image --all-frames --output frames image.dcm
dicom-cli convert json --output metadata.json image.dcm
```

`convert image <input>` 导出 PNG 或 JPEG。`--frame` 是从 1 开始的帧号；默认导出
首帧，`--all-frames` 导出每一帧。`convert json <input>` 默认将 PixelData 写为摘要，
只有 `--include-pixel-data` 才包含像素字节。

两个子命令都支持文件或目录输入、`--recursive`、`--flatten`、`--fail-fast` 和
`--output`。目录默认不递归。图片二进制 stdout 要求恰好一个结果；多帧、多文件和
目录输入不得使用 `--output -`。未传 `--output` 时，结果写入当前目录下的 `convert`
子目录。

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
`convert` 子目录。

### transcode

```sh
dicom-cli transcode formats
dicom-cli transcode --to rle --output compressed.dcm image.dcm
dicom-cli transcode --to 1.2.840.10008.1.2.1 --output output study
```

`transcode formats` 显示当前二进制实际注册的传输语法、别名和编码/解码能力，
可加 `--json`。执行转码时 `--to` 和 `--output` 都必填；`--to` 接受
`transcode formats` 输出的别名或标准传输语法 UID，例如 `--to rle` 或
`--to 1.2.840.10008.1.2.1`。`transcode <file>` 接受单文件或目录；目录模式支持
`--recursive`、`--flatten`、`--filter`、`--fail-fast`。转码只改变与编解码和传输语法
有关的数据；输出路径不得是输入路径。

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

`anonymize`、`convert`、`transcode` 和目录模式 `send` 中，不符合规则筛选的文件为
正常跳过；损坏或非 DICOM 文件会计为失败。默认批处理继续处理剩余文件，
`--fail-fast` 会在首个失败后停止。

| 退出码 | 含义 |
| --- | --- |
| `0` | 成功。 |
| `1` | 操作失败，包括批处理中的失败文件、网络和 DIMSE 失败。 |
| `2` | 参数、配置或规则输入不合法。 |
| `3` | DICOM 校验失败。 |

真实样本/PACS 验收不在默认命令或 CI 中执行。运行受控外部验收前，应确认样本可合法
使用、PACS 目标已获授权，并将地址、AE Title 和凭据保留在部署环境而非仓库中。
